#!/bin/bash

################################################################################
# Container Registry Service Installer - containerd Setup
# 
# This script installs and configures containerd for the OpenCloud Container
# Registry Service. It checks if containerd is already installed and only
# performs installation if necessary.
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

readonly SERVICE_NAME="containerd"
readonly PACKAGE_NAME="containerd"

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

# Install containerd using apt-get
install_containerd() {
    print_info "Updating package index..."
    sudo apt-get update

    print_info "Installing ${PACKAGE_NAME}..."
    sudo apt-get install -y "${PACKAGE_NAME}"

    print_success "${PACKAGE_NAME} package installed successfully"
}

# Start and enable containerd service
configure_containerd_service() {
    print_info "Starting ${SERVICE_NAME} service..."
    sudo systemctl start "${SERVICE_NAME}"

    print_info "Enabling ${SERVICE_NAME} service to start on boot..."
    sudo systemctl enable "${SERVICE_NAME}"

    print_success "${SERVICE_NAME} service is running and enabled"
}

# Verify containerd installation
verify_installation() {
    print_info "Verifying ${SERVICE_NAME} installation..."
    
    if command -v containerd &> /dev/null; then
        local version
        version=$(containerd --version 2>&1 | head -n 1)
        print_success "containerd is installed: ${version}"
        
        # Check service status
        if sudo systemctl is-active --quiet "${SERVICE_NAME}"; then
            print_success "${SERVICE_NAME} service is active and running"
        else
            print_error "${SERVICE_NAME} service is not running"
            return 1
        fi
    else
        print_error "${SERVICE_NAME} is not installed"
        return 1
    fi
    
    return 0
}

################################################################################
# Main Script
################################################################################

main() {
    print_info "Container Registry Service Installer - containerd Setup"
    print_info "==========================================================="
    echo

    # Check if containerd is already installed
    if check_containerd_installed; then
        local version
        version=$(containerd --version 2>&1 | head -n 1)
        print_info "${SERVICE_NAME} is already installed on this system"
        print_info "Current version: ${version}"
        
        # Check if service is running
        if sudo systemctl is-active --quiet "${SERVICE_NAME}"; then
            print_success "${SERVICE_NAME} service is already running"
        else
            print_info "${SERVICE_NAME} service is not running, starting it..."
            configure_containerd_service
        fi
    else
        print_info "${SERVICE_NAME} is not installed. Beginning installation..."
        echo
        
        # Install containerd
        install_containerd
        echo
        
        # Configure and start the service
        configure_containerd_service
        echo
    fi

    # Verify the installation
    echo
    verify_installation
    echo

    print_success "Container Registry Service setup completed successfully!"
    return 0
}

# Execute main function
main "$@"
