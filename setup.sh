#!/bin/bash

# This Script installs the necessary software to host the bare minimum of OpenCloud: The User Interface.
# Consistent with OpenCloud's ideals, the subdependencies for each service are installed when a user enables the service on their cluster
#
# Requirements:
# - Ubuntu/Debian-based Linux distribution
# - sudo access
# - Internet connection
#
# Optional Environment Variables:
# - OPENCLOUD_INSTALL_DIR: Override installation directory (default: /home/$USER/OpenCloud)

set -e  # Exit on any error

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored messages
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Get the directory where the script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

print_info "Starting OpenCloud setup..."

# Check OS compatibility
if [ -f /etc/os-release ]; then
    . /etc/os-release
    if [[ "$ID" != "ubuntu" && "$ID" != "debian" && "$ID_LIKE" != *"ubuntu"* && "$ID_LIKE" != *"debian"* ]]; then
        print_warning "This script is designed for Ubuntu/Debian systems."
        print_warning "Detected OS: $ID. Installation may fail or require modifications."
        read -p "Continue anyway? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
fi

# Step 1: Check for and install Go if not present
print_info "Checking for Go installation..."
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    print_info "Go is already installed (version: $GO_VERSION)"

    # Check if Go version meets minimum requirement (1.24.0)
    REQUIRED_VERSION="1.24.12"
    if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
        print_warning "Go version $GO_VERSION is older than required version $REQUIRED_VERSION"
        print_warning "Please update Go manually to version $REQUIRED_VERSION or higher"
    fi
else
    print_info "Go not found. Installing Go 1.24.12..."

    # Detect architecture
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            GO_ARCH="amd64"
            ;;
        aarch64|arm64)
            GO_ARCH="arm64"
            ;;
        *)
            print_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    # Download and install Go
    GO_VERSION="1.24.12"
    GO_TARBALL="go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
    wget -q "https://go.dev/dl/${GO_TARBALL}" -O "/tmp/${GO_TARBALL}"

    # Remove old Go installation if exists
    sudo rm -rf /usr/local/go

    # Extract new Go
    sudo tar -C /usr/local -xzf "/tmp/${GO_TARBALL}"
    rm "/tmp/${GO_TARBALL}"

    # Add Go to PATH if not already there
    if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    fi
    export PATH=$PATH:/usr/local/go/bin
    source ~/.bashrc

    print_info "Go ${GO_VERSION} installed successfully"
fi

# Step 2: Check for and install Node.js/npm if not present
print_info "Checking for Node.js installation..."
if command -v node &> /dev/null; then
    NODE_VERSION=$(node --version)
    print_info "Node.js is already installed (version: $NODE_VERSION)"
else
    print_info "Node.js not found. Installing Node.js LTS..."
    
    # Install Node.js using NodeSource repository
    curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -
    sudo apt-get install -y nodejs
    
    print_info "Node.js installed successfully"
fi

# Step 3: Check for and install make if not present
print_info "Checking for make installation..."
if command -v make &> /dev/null; then
    print_info "make is already installed"
else
    print_info "make not found. Installing make..."
    sudo apt-get update -qq
    sudo apt-get install -y build-essential
    
    # Verify installation
    if command -v make &> /dev/null; then
        print_info "make installed successfully"
    else
        print_error "Failed to install make. Please install it manually."
        exit 1
    fi
fi

# Step 4: Install TypeScript dependencies for the UI
print_info "Installing TypeScript dependencies in ui/ directory..."
cd "${SCRIPT_DIR}/ui"
npm install
print_info "TypeScript dependencies installed successfully"

# Step 5: Compile the Go app into a binary
print_info "Building Go backend..."
cd "$SCRIPT_DIR"
make build
print_info "Go backend built successfully"

# Step 6: Build the Next.js UI for production
print_info "Building Next.js UI for production..."
cd "${SCRIPT_DIR}/ui"
npm run build
print_info "Next.js UI built successfully"
cd "$SCRIPT_DIR"

# Step 7: Move Go binary to systemd service location
print_info "Setting up Go backend binary..."
# Determine installation directory - use /opt for system-wide install or user's home for user install
INSTALL_DIR="${OPENCLOUD_INSTALL_DIR:-/home/$USER/OpenCloud}"
print_info "Installing to: $INSTALL_DIR"

# Stop the service if it's running to avoid "Text file busy" error
if sudo systemctl is-active --quiet opencloud.service; then
    print_info "Stopping running OpenCloud service..."
    sudo systemctl stop opencloud.service
fi

# Create the target directory structure
sudo mkdir -p "$INSTALL_DIR/bin"
sudo cp "${SCRIPT_DIR}/bin/app" "$INSTALL_DIR/bin/opencloud"
sudo chmod +x "$INSTALL_DIR/bin/opencloud"
print_info "Go binary copied to $INSTALL_DIR/bin/opencloud"

# Step 8: Setup systemd service for the Go backend
print_info "Setting up systemd service for OpenCloud backend..."
# Create a temporary service file with updated paths
TEMP_SERVICE=$(mktemp)
sed "s|/home/ubuntu/OpenCloud|$INSTALL_DIR|g" "${SCRIPT_DIR}/utils/opencloud.service" | \
sed "s|User=ubuntu|User=$USER|g" | \
sed "s|Group=ubuntu|Group=$USER|g" > "$TEMP_SERVICE"

sudo cp "$TEMP_SERVICE" /etc/systemd/system/opencloud.service
rm "$TEMP_SERVICE"

print_info "Systemd service configured for $INSTALL_DIR"

# Reload systemd to recognize the new service
sudo systemctl daemon-reload
print_info "Systemd service configured"

# Step 9: Setup systemd service for the Next.js frontend
print_info "Setting up systemd service for OpenCloud frontend..."

# Stop the frontend service if it's running to avoid conflicts
if sudo systemctl is-active --quiet opencloud-ui.service; then
    print_info "Stopping running OpenCloud frontend service..."
    sudo systemctl stop opencloud-ui.service
fi

# Copy UI build to installation directory
sudo mkdir -p "$INSTALL_DIR/ui"
sudo cp -r "${SCRIPT_DIR}/ui/.next" "$INSTALL_DIR/ui/"
sudo cp -r "${SCRIPT_DIR}/ui/node_modules" "$INSTALL_DIR/ui/"
sudo cp "${SCRIPT_DIR}/ui/package.json" "$INSTALL_DIR/ui/"
sudo cp "${SCRIPT_DIR}/ui/next.config.mjs" "$INSTALL_DIR/ui/" 2>/dev/null || true

# Create a temporary service file with updated paths for frontend
TEMP_UI_SERVICE=$(mktemp)
sed "s|/home/ubuntu/OpenCloud|$INSTALL_DIR|g" "${SCRIPT_DIR}/utils/opencloud-ui.service" | \
sed "s|User=ubuntu|User=$USER|g" | \
sed "s|Group=ubuntu|Group=$USER|g" > "$TEMP_UI_SERVICE"

sudo cp "$TEMP_UI_SERVICE" /etc/systemd/system/opencloud-ui.service
rm "$TEMP_UI_SERVICE"

print_info "Frontend systemd service configured"

# Step 10: Install and configure nginx
print_info "Setting up nginx web server..."

# Check for and install nginx if not present
if ! command -v nginx &> /dev/null; then
    print_info "nginx not found. Installing nginx..."
    sudo apt-get update -qq
    sudo apt-get install -y nginx
    print_info "nginx installed successfully"
else
    print_info "nginx is already installed"
fi

# Copy nginx configuration
print_info "Configuring nginx for OpenCloud..."
sudo cp "${SCRIPT_DIR}/utils/opencloud.nginx.conf" /etc/nginx/sites-available/opencloud

# Remove default nginx site if it exists
if [ -f /etc/nginx/sites-enabled/default ]; then
    sudo rm /etc/nginx/sites-enabled/default
    print_info "Removed default nginx site"
fi

# Create symbolic link to enable the site
sudo ln -sf /etc/nginx/sites-available/opencloud /etc/nginx/sites-enabled/opencloud

# Test nginx configuration
print_info "Testing nginx configuration..."
if sudo nginx -t; then
    print_info "nginx configuration is valid"
else
    print_error "nginx configuration test failed"
    exit 1
fi

print_info "nginx configured successfully"

# Step 11: Start the services
print_info "Starting OpenCloud services..."

# Reload systemd daemon to pick up new service
sudo systemctl daemon-reload

# Start the Go backend
sudo systemctl enable opencloud.service
sudo systemctl start opencloud.service

# Start the Next.js frontend
sudo systemctl enable opencloud-ui.service
sudo systemctl start opencloud-ui.service

# Start nginx
sudo systemctl enable nginx
sudo systemctl restart nginx

# Wait a moment for services to start
sleep 3

# Check service status
print_info "Checking service status..."
if sudo systemctl is-active --quiet opencloud.service; then
    print_info "OpenCloud backend service is running"
else
    print_error "OpenCloud backend service failed to start"
    sudo systemctl status opencloud.service
    exit 1
fi

if sudo systemctl is-active --quiet opencloud-ui.service; then
    print_info "OpenCloud frontend service is running"
else
    print_error "OpenCloud frontend service failed to start"
    sudo systemctl status opencloud-ui.service
    exit 1
fi

if sudo systemctl is-active --quiet nginx; then
    print_info "nginx web server is running"
else
    print_error "nginx service failed to start"
    sudo systemctl status nginx
    exit 1
fi

# Print success message
echo ""
print_info "========================================="
print_info "OpenCloud setup completed successfully!"
print_info "========================================="
print_info ""
print_info "Access OpenCloud via nginx:"
print_info "  Web Interface: http://123.123.123.123"
print_info "  (Replace 123.123.123.123 with your actual server IP)"
print_info ""
print_info "Direct service access (for debugging):"
print_info "  Backend API: http://localhost:3030"
print_info "  Frontend UI: http://localhost:3000"
print_info ""
print_info "Service Management Commands:"
print_info "  Backend: sudo systemctl {start|stop|restart|status} opencloud.service"
print_info "  Frontend: sudo systemctl {start|stop|restart|status} opencloud-ui.service"
print_info "  Nginx: sudo systemctl {start|stop|restart|status} nginx"
print_info ""
print_info "To view logs:"
print_info "  Backend: sudo journalctl -u opencloud.service -f"
print_info "  Frontend: sudo journalctl -u opencloud-ui.service -f"
print_info "  Nginx: sudo tail -f /var/log/nginx/opencloud_access.log"
print_info ""
print_info "To update the server IP address:"
print_info "  Edit /etc/nginx/sites-available/opencloud and change server_name"
print_info "  Then run: sudo nginx -t && sudo systemctl restart nginx"
print_info "========================================="
