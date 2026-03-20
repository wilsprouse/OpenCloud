#!/bin/bash

################################################################################
# Container Registry Service Installer - Podman Setup
#
# This script installs and configures Podman for the OpenCloud Container
# Registry and Container Runtime services. It ensures the Podman API socket is
# accessible to the current user to prevent permission errors.
################################################################################

set -e
set -o pipefail

################################################################################
# Configuration Variables
################################################################################

readonly PODMAN_SERVICE_NAME="podman.socket"
readonly PODMAN_PACKAGE_NAME="podman"
readonly PODMAN_SOCKET_DIR="/run/podman"
readonly PODMAN_SOCKET_PATH="${PODMAN_SOCKET_DIR}/podman.sock"
readonly PODMAN_GROUP_NAME="podman"

################################################################################
# Helper Functions
################################################################################

print_info() { echo "[INFO] $1"; }
print_success() { echo "[SUCCESS] $1"; }
print_error() { echo "[ERROR] $1" >&2; }

check_podman_installed() { command -v podman &> /dev/null; }

ensure_podman_group_access() {
    print_info "Ensuring ${PODMAN_GROUP_NAME} group exists..."
    if ! getent group "${PODMAN_GROUP_NAME}" > /dev/null; then
        sudo groupadd --system "${PODMAN_GROUP_NAME}"
        print_success "Created system group: ${PODMAN_GROUP_NAME}"
    else
        print_info "Group ${PODMAN_GROUP_NAME} already exists"
    fi

    local current_user
    current_user="$(id -un)"
    if ! id -nG "$current_user" | grep -qw "${PODMAN_GROUP_NAME}"; then
        sudo usermod -aG "${PODMAN_GROUP_NAME}" "$current_user"
        print_success "Added user $current_user to ${PODMAN_GROUP_NAME} group"
    else
        print_info "User $current_user is already in ${PODMAN_GROUP_NAME} group"
    fi

    if id "ubuntu" &>/dev/null; then
        if ! id -nG "ubuntu" | grep -qw "${PODMAN_GROUP_NAME}"; then
            sudo usermod -aG "${PODMAN_GROUP_NAME}" "ubuntu"
            print_success "Added user ubuntu to ${PODMAN_GROUP_NAME} group"
        else
            print_info "User ubuntu is already in ${PODMAN_GROUP_NAME} group"
        fi
    fi

    print_info "NOTE: Services may need to be restarted for group membership to apply"
}

install_podman() {
    print_info "Updating package index..."
    sudo apt-get update
    print_info "Installing ${PODMAN_PACKAGE_NAME}..."
    sudo apt-get install -y "${PODMAN_PACKAGE_NAME}"
    print_success "${PODMAN_PACKAGE_NAME} installed successfully"
}

configure_podman_socket() {
    print_info "Configuring Podman socket permissions..."
    sudo mkdir -p /etc/systemd/system/podman.socket.d
    sudo tee /etc/systemd/system/podman.socket.d/opencloud-group.conf > /dev/null <<EOF
[Socket]
SocketGroup=${PODMAN_GROUP_NAME}
SocketMode=0660
EOF

    sudo systemctl daemon-reload
    sudo systemctl start "${PODMAN_SERVICE_NAME}"
    sudo systemctl enable "${PODMAN_SERVICE_NAME}"
    sleep 2
    print_success "Podman socket running and enabled"
}

verify_podman() {
    if command -v podman &> /dev/null; then
        print_success "podman installed: $(podman --version | head -n 1)"
        if [ -S "${PODMAN_SOCKET_PATH}" ]; then
            print_success "Podman socket exists at ${PODMAN_SOCKET_PATH}"
        else
            print_error "Podman socket missing or user not in podman group"
            return 1
        fi
    else
        print_error "podman not installed"
        return 1
    fi
}

################################################################################
# Main Script
################################################################################

main() {
    print_info "Starting Container Registry Service Installer"
    echo

    if check_podman_installed; then
        print_info "podman already installed"
    else
        install_podman
    fi
    verify_podman || true
    echo

    ensure_podman_group_access
    echo

    configure_podman_socket
    verify_podman
    echo

    print_success "Podman setup completed successfully!"
    print_info "podman socket: ${PODMAN_SOCKET_PATH}"
}

# Run main
main "$@"
