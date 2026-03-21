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

readonly PODMAN_PACKAGE_NAME="podman"
readonly PODMAN_SERVICE_NAME="podman.socket"
PODMAN_USER_NAME="${SUDO_USER:-$(id -un)}"
if [ "$(id -u)" -eq 0 ] && [ "${PODMAN_USER_NAME}" = "root" ]; then
    echo "[ERROR] Could not determine the target user. Run this script as the intended non-root application user, or invoke it with sudo from that user account." >&2
    exit 1
fi
PODMAN_USER_ID="$(id -u "${PODMAN_USER_NAME}")"
PODMAN_RUNTIME_DIR="/run/user/${PODMAN_USER_ID}"
PODMAN_SOCKET_PATH="${PODMAN_RUNTIME_DIR}/podman/podman.sock"

################################################################################
# Helper Functions
################################################################################

print_info() { echo "[INFO] $1"; }
print_success() { echo "[SUCCESS] $1"; }
print_error() { echo "[ERROR] $1" >&2; }

check_podman_installed() { command -v podman &> /dev/null; }

run_user_systemctl() {
    sudo -u "${PODMAN_USER_NAME}" env \
        XDG_RUNTIME_DIR="${PODMAN_RUNTIME_DIR}" \
        DBUS_SESSION_BUS_ADDRESS="unix:path=${PODMAN_RUNTIME_DIR}/bus" \
        systemctl --user "$@"
}

install_podman() {
    print_info "Updating package index..."
    sudo apt-get update
    print_info "Installing ${PODMAN_PACKAGE_NAME}..."
    sudo apt-get install -y "${PODMAN_PACKAGE_NAME}"
    print_success "${PODMAN_PACKAGE_NAME} installed successfully"
}

configure_podman_socket() {
    print_info "Configuring rootless Podman socket for ${PODMAN_USER_NAME}..."
    sudo loginctl enable-linger "${PODMAN_USER_NAME}"
    sudo systemctl start "user@${PODMAN_USER_ID}.service"
    run_user_systemctl daemon-reload
    run_user_systemctl enable "${PODMAN_SERVICE_NAME}"
    run_user_systemctl start "${PODMAN_SERVICE_NAME}"
    sleep 2
    print_success "Rootless Podman socket running and enabled"
}

verify_podman() {
    if command -v podman &> /dev/null; then
        print_success "podman installed: $(podman --version | head -n 1)"
        if [ -S "${PODMAN_SOCKET_PATH}" ]; then
            print_success "Podman socket exists at ${PODMAN_SOCKET_PATH}"
        else
            print_error "Rootless Podman socket missing at ${PODMAN_SOCKET_PATH}"
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

    configure_podman_socket
    verify_podman
    echo

    print_success "Podman setup completed successfully!"
    print_info "podman socket: ${PODMAN_SOCKET_PATH}"
}

# Run main
main "$@"
