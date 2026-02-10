#!/bin/bash

################################################################################
# Container Registry Service Installer - containerd and buildkit Setup
# 
# This script installs and configures containerd and buildkit for the OpenCloud
# Container Registry Service. It ensures the BuildKit socket is accessible to
# the current user to prevent permission errors.
#
# Requirements:
#   - Ubuntu/Debian-based system
#   - sudo privileges for package installation
#   - systemd for service management
################################################################################

set -e
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
# Helper Functions
################################################################################

print_info() { echo "[INFO] $1"; }
print_success() { echo "[SUCCESS] $1"; }
print_error() { echo "[ERROR] $1" >&2; }

check_containerd_installed() { command -v containerd &> /dev/null; }
check_buildkit_installed() { command -v buildkitd &> /dev/null; }

# Ensure BuildKit group exists and current user is in it
ensure_buildkit_group_access() {
    print_info "Ensuring ${BUILDKIT_GROUP_NAME} group exists..."
    if ! getent group "${BUILDKIT_GROUP_NAME}" > /dev/null; then
        sudo groupadd --system "${BUILDKIT_GROUP_NAME}"
        print_success "Created system group: ${BUILDKIT_GROUP_NAME}"
    else
        print_info "Group ${BUILDKIT_GROUP_NAME} already exists"
    fi

    local current_user
    current_user="$(id -un)"
    if ! id -nG "$current_user" | grep -qw "${BUILDKIT_GROUP_NAME}"; then
        sudo usermod -aG "${BUILDKIT_GROUP_NAME}" "$current_user"
        print_success "Added user $current_user to ${BUILDKIT_GROUP_NAME} group"
        print_info "NOTE: You may need to log out and back in for group membership to apply"
    else
        print_info "User $current_user is already in ${BUILDKIT_GROUP_NAME} group"
    fi
}

# Install containerd
install_containerd() {
    print_info "Updating package index..."
    sudo apt-get update
    print_info "Installing ${CONTAINERD_PACKAGE_NAME}..."
    sudo apt-get install -y "${CONTAINERD_PACKAGE_NAME}"
    print_success "${CONTAINERD_PACKAGE_NAME} installed successfully"
}

# Install BuildKit
install_buildkit() {
    print_info "Installing buildkit ${BUILDKIT_VERSION}..."
    local temp_dir
    temp_dir=$(mktemp -d)
    trap "rm -rf ${temp_dir}" EXIT

    print_info "Downloading buildkit from ${BUILDKIT_BINARY_URL}..."
    curl -sSL "${BUILDKIT_BINARY_URL}" -o "${temp_dir}/buildkit.tar.gz"

    print_info "Extracting buildkit binaries..."
    sudo tar -C "${BUILDKIT_INSTALL_DIR}" -xzf "${temp_dir}/buildkit.tar.gz"

    if command -v buildkitd &> /dev/null; then
        print_success "buildkit installed successfully"
        print_info "Version: $(buildkitd --version | head -n 1)"
    else
        print_error "buildkit installation failed"
        return 1
    fi
}

# Create BuildKit systemd service
create_buildkit_service() {
    print_info "Creating buildkit systemd service..."

    # Ensure socket directory exists and has proper permissions
    sudo mkdir -p "${BUILDKIT_SOCKET_DIR}"
    sudo chown root:${BUILDKIT_GROUP_NAME} "${BUILDKIT_SOCKET_DIR}"
    sudo chmod 2775 "${BUILDKIT_SOCKET_DIR}"

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

# Start & enable containerd
configure_containerd_service() {
    sudo systemctl start "${CONTAINERD_SERVICE_NAME}"
    sudo systemctl enable "${CONTAINERD_SERVICE_NAME}"
    print_success "${CONTAINERD_SERVICE_NAME} service running and enabled"
}

# Start & enable BuildKit
configure_buildkit_service() {
    sudo systemctl daemon-reload
    sudo systemctl start buildkit
    sudo systemctl enable buildkit
    sleep 2
    print_success "buildkit service running and enabled"
}

# Verify containerd
verify_containerd() {
    if command -v containerd &> /dev/null; then
        print_success "containerd installed: $(containerd --version | head -n 1)"
    else
        print_error "containerd not installed"
        return 1
    fi
}

# Verify BuildKit
verify_buildkit() {
    if command -v buildkitd &> /dev/null; then
        print_success "buildkit installed: $(buildkitd --version | head -n 1)"
        if [ -S "${BUILDKIT_SOCKET_PATH}" ]; then
            print_success "buildkit socket exists at ${BUILDKIT_SOCKET_PATH}"
        else
            print_error "buildkit socket missing or user not in buildkit group"
            return 1
        fi
    else
        print_error "buildkit not installed"
        return 1
    fi
}

################################################################################
# Main Script
################################################################################

main() {
    print_info "Starting Container Registry Service Installer"
    echo

    # Step 1: containerd
    if check_containerd_installed; then
        print_info "containerd already installed"
        configure_containerd_service
    else
        install_containerd
        configure_containerd_service
    fi
    verify_containerd
    echo

    # Step 2: BuildKit
    ensure_buildkit_group_access
    echo

    if check_buildkit_installed; then
        print_info "buildkit already installed"
    else
        install_buildkit
    fi

    if [ -f /etc/systemd/system/buildkit.service ]; then
        print_info "buildkit service file exists"
    else
        create_buildkit_service
    fi

    configure_buildkit_service
    verify_buildkit
    echo

    print_success "Container Registry Service setup completed successfully!"
    print_info "containerd socket: /run/containerd/containerd.sock"
    print_info "buildkit socket: ${BUILDKIT_SOCKET_PATH}"
}

# Run main
main "$@"
