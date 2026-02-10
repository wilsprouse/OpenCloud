#!/bin/bash

################################################################################
# Container Registry Service Installer - containerd and buildkit Setup
# 
# This script installs and configures containerd and buildkit for the OpenCloud
# Container Registry Service. It checks if components are already installed and
# only performs installation if necessary.
#
# Requirements:
#   - Ubuntu/Debian-based system
#   - sudo privileges for package installation
#   - systemd for service management
################################################################################

# Exit immediately if a command exits with a non-zero status
set -e

# Enable pipefail to catch errors in pipes
set -o pipefail

################################################################################
# Configuration Variables
################################################################################

readonly CONTAINERD_SERVICE_NAME="containerd"
readonly CONTAINERD_PACKAGE_NAME="containerd"
readonly BUILDKIT_VERSION="v0.16.0"
readonly BUILDKIT_BINARY_URL="https://github.com/moby/buildkit/releases/download/${BUILDKIT_VERSION}/buildkit-${BUILDKIT_VERSION}.linux-amd64.tar.gz"
readonly BUILDKIT_INSTALL_DIR="/usr/local"
readonly BUILDKIT_SOCKET_DIR="/run/buildkit"
readonly BUILDKIT_SOCKET_PATH="${BUILDKIT_SOCKET_DIR}/buildkitd.sock"
readonly BUILDKIT_GROUP_NAME="buildkit"

################################################################################
# Functions
################################################################################

# Print informational message
print_info() {
    echo "[INFO] $1"
}

# Print success message
print_success() {
    echo "[SUCCESS] $1"
}

# Print error message
print_error() {
    echo "[ERROR] $1" >&2
}

# Check if containerd is already installed
check_containerd_installed() {
    if command -v containerd &> /dev/null; then
        return 0  # containerd is installed
    else
        return 1  # containerd is not installed
    fi
}

# Check if buildkit is already installed
check_buildkit_installed() {
    if command -v buildkitd &> /dev/null; then
        return 0  # buildkit is installed
    else
        return 1  # buildkit is not installed
    fi
}

# Ensure buildkit group exists and current user is in it
ensure_buildkit_group_access() {
    print_info "Ensuring ${BUILDKIT_GROUP_NAME} group exists..."

    if getent group "${BUILDKIT_GROUP_NAME}" > /dev/null 2>&1; then
        print_info "Group ${BUILDKIT_GROUP_NAME} already exists"
    else
        sudo groupadd --system "${BUILDKIT_GROUP_NAME}"
        print_success "Created system group: ${BUILDKIT_GROUP_NAME}"
    fi

    # Add current user to the buildkit group
    local current_user
    current_user="$(id -un)"

    print_info "Ensuring user ${current_user} is in ${BUILDKIT_GROUP_NAME} group..."

    if id -nG "${current_user}" | grep -qw "${BUILDKIT_GROUP_NAME}"; then
        print_info "User ${current_user} is already in ${BUILDKIT_GROUP_NAME} group"
    else
        sudo usermod -aG "${BUILDKIT_GROUP_NAME}" "${current_user}"
        print_success "Added user ${current_user} to ${BUILDKIT_GROUP_NAME} group"
        print_info "NOTE: You may need to log out and back in for group membership to apply"
    fi
}

# Install containerd using apt-get
install_containerd() {
    print_info "Updating package index..."
    sudo apt-get update

    print_info "Installing ${CONTAINERD_PACKAGE_NAME}..."
    sudo apt-get install -y "${CONTAINERD_PACKAGE_NAME}"

    print_success "${CONTAINERD_PACKAGE_NAME} package installed successfully"
}

# Install buildkit from GitHub releases
install_buildkit() {
    print_info "Installing buildkit ${BUILDKIT_VERSION}..."
    
    # Create temporary directory for download
    local temp_dir
    temp_dir=$(mktemp -d)
    trap "rm -rf ${temp_dir}" EXIT
    
    # Download buildkit
    print_info "Downloading buildkit from ${BUILDKIT_BINARY_URL}..."
    if ! curl -sSL "${BUILDKIT_BINARY_URL}" -o "${temp_dir}/buildkit.tar.gz"; then
        print_error "Failed to download buildkit"
        return 1
    fi
    
    # Extract and install binaries
    print_info "Extracting buildkit binaries..."
    sudo tar -C "${BUILDKIT_INSTALL_DIR}" -xzf "${temp_dir}/buildkit.tar.gz"
    
    # Verify installation
    if command -v buildkitd &> /dev/null; then
        print_success "buildkit installed successfully"
        local version
        version=$(buildkitd --version 2>&1 | head -n 1)
        print_info "Installed version: ${version}"
    else
        print_error "buildkit installation failed"
        return 1
    fi
    
    return 0
}

# Create buildkit systemd service
create_buildkit_service() {
    print_info "Creating buildkit systemd service..."
    
    # Create socket directory
    sudo mkdir -p "${BUILDKIT_SOCKET_DIR}"
    
    # Create systemd service file
    sudo tee /etc/systemd/system/buildkit.service > /dev/null <<EOF
[Unit]
Description=BuildKit daemon for OpenCloud Container Registry
Documentation=https://github.com/moby/buildkit
After=network.target containerd.service
Requires=containerd.service

[Service]
Type=notify
ExecStart=/usr/local/bin/buildkitd --addr unix://${BUILDKIT_SOCKET_PATH} --group ${BUILDKIT_GROUP_NAME}
Restart=always
RestartSec=5
LimitNOFILE=1048576
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
TimeoutStartSec=0
KillMode=process

[Install]
WantedBy=multi-user.target
EOF
    
    print_success "buildkit systemd service created"
}

# Start and enable containerd service
configure_containerd_service() {
    print_info "Starting ${CONTAINERD_SERVICE_NAME} service..."
    sudo systemctl start "${CONTAINERD_SERVICE_NAME}"

    print_info "Enabling ${CONTAINERD_SERVICE_NAME} service to start on boot..."
    sudo systemctl enable "${CONTAINERD_SERVICE_NAME}"

    print_success "${CONTAINERD_SERVICE_NAME} service is running and enabled"
}

# Start and enable buildkit service
configure_buildkit_service() {
    print_info "Reloading systemd daemon..."
    sudo systemctl daemon-reload
    
    print_info "Starting buildkit service..."
    sudo systemctl start buildkit
    
    print_info "Enabling buildkit service to start on boot..."
    sudo systemctl enable buildkit
    
    # Wait a moment for the socket to be created
    sleep 2
    
    print_success "buildkit service is running and enabled"
}

# Verify containerd installation
verify_containerd() {
    print_info "Verifying ${CONTAINERD_SERVICE_NAME} installation..."
    
    if command -v containerd &> /dev/null; then
        local version
        version=$(containerd --version 2>&1 | head -n 1)
        print_success "containerd is installed: ${version}"
        
        # Check service status
        if sudo systemctl is-active --quiet "${CONTAINERD_SERVICE_NAME}"; then
            print_success "${CONTAINERD_SERVICE_NAME} service is active and running"
        else
            print_error "${CONTAINERD_SERVICE_NAME} service is not running"
            return 1
        fi
    else
        print_error "${CONTAINERD_SERVICE_NAME} is not installed"
        return 1
    fi
    
    return 0
}

# Verify buildkit installation
verify_buildkit() {
    print_info "Verifying buildkit installation..."
    
    if command -v buildkitd &> /dev/null; then
        local version
        version=$(buildkitd --version 2>&1 | head -n 1)
        print_success "buildkit is installed: ${version}"
        
        # Check service status
        if sudo systemctl is-active --quiet buildkit; then
            print_success "buildkit service is active and running"
        else
            print_error "buildkit service is not running"
            return 1
        fi
        
        # Check socket existence
        if [ -S "${BUILDKIT_SOCKET_PATH}" ]; then
            print_success "buildkit socket exists at ${BUILDKIT_SOCKET_PATH}"
        else
            print_error "buildkit socket not found at ${BUILDKIT_SOCKET_PATH}"
            return 1
        fi
    else
        print_error "buildkit is not installed"
        return 1
    fi
    
    return 0
}

################################################################################
# Main Script
################################################################################

main() {
    print_info "Container Registry Service Installer - containerd and buildkit Setup"
    print_info "======================================================================"
    echo

    # ========== Install and configure containerd ==========
    print_info "Step 1: containerd installation"
    print_info "--------------------------------"
    
    # Check if containerd is already installed
    if check_containerd_installed; then
        local version
        version=$(containerd --version 2>&1 | head -n 1)
        print_info "${CONTAINERD_SERVICE_NAME} is already installed on this system"
        print_info "Current version: ${version}"
        
        # Check if service is running
        if sudo systemctl is-active --quiet "${CONTAINERD_SERVICE_NAME}"; then
            print_success "${CONTAINERD_SERVICE_NAME} service is already running"
        else
            print_info "${CONTAINERD_SERVICE_NAME} service is not running, starting it..."
            configure_containerd_service
        fi
    else
        print_info "${CONTAINERD_SERVICE_NAME} is not installed. Beginning installation..."
        echo
        
        # Install containerd
        install_containerd
        echo
        
        # Configure and start the service
        configure_containerd_service
        echo
    fi

    # Verify containerd installation
    echo
    verify_containerd
    echo

    # ========== Install and configure buildkit ==========
    print_info "Step 2: buildkit installation"
    print_info "------------------------------"
    
    # Ensure buildkit group access exists before starting/creating service
    ensure_buildkit_group_access
    echo

    # Check if buildkit is already installed
    if check_buildkit_installed; then
        local version
        version=$(buildkitd --version 2>&1 | head -n 1)
        print_info "buildkit is already installed on this system"
        print_info "Current version: ${version}"
        
        # Check if service is running
        if sudo systemctl is-active --quiet buildkit; then
            print_success "buildkit service is already running"
            
            # Check socket
            if [ -S "${BUILDKIT_SOCKET_PATH}" ]; then
                print_success "buildkit socket exists at ${BUILDKIT_SOCKET_PATH}"
            else
                print_info "buildkit socket not found, restarting service..."
                sudo systemctl restart buildkit
                sleep 2
            fi
        else
            print_info "buildkit service is not running, starting it..."
            
            # Check if service file exists
            if [ -f /etc/systemd/system/buildkit.service ]; then
                configure_buildkit_service
            else
                print_info "buildkit service file not found, creating it..."
                create_buildkit_service
                configure_buildkit_service
            fi
        fi
    else
        print_info "buildkit is not installed. Beginning installation..."
        echo
        
        # Install buildkit
        install_buildkit
        echo
        
        # Create and configure the service
        create_buildkit_service
        echo
        
        configure_buildkit_service
        echo
    fi

    # Verify buildkit installation
    echo
    verify_buildkit
    echo

    print_success "Container Registry Service setup completed successfully!"
    print_info "containerd socket: /run/containerd/containerd.sock"
    print_info "buildkit socket: ${BUILDKIT_SOCKET_PATH}"
    return 0
}

# Execute main function
main "$@"
