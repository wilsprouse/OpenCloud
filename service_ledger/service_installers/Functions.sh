#!/bin/bash

################################################################################
# Functions Service Installer
#
# This script sets up the directory structure and verifies available runtimes
# for the OpenCloud Functions (serverless compute) service.
################################################################################

set -e
set -o pipefail

################################################################################
# Configuration Variables
################################################################################

readonly OPENCLOUD_DIR="${HOME}/.opencloud"
readonly FUNCTIONS_DIR="${OPENCLOUD_DIR}/functions"
readonly LOGS_DIR="${FUNCTIONS_DIR}/logs"

################################################################################
# Helper Functions
################################################################################

print_info()    { echo "[INFO] $1"; }
print_success() { echo "[SUCCESS] $1"; }
print_warning() { echo "[WARNING] $1"; }
print_error()   { echo "[ERROR] $1" >&2; }

################################################################################
# Main Script
################################################################################

main() {
    print_info "Starting Functions Service Installer"
    echo

    # Create the functions directory
    print_info "Creating functions directory at ${FUNCTIONS_DIR}..."
    mkdir -p "${FUNCTIONS_DIR}"
    print_success "Functions directory ready: ${FUNCTIONS_DIR}"

    # Create the logs directory
    print_info "Creating logs directory at ${LOGS_DIR}..."
    mkdir -p "${LOGS_DIR}"
    print_success "Logs directory ready: ${LOGS_DIR}"

    echo

    # Check available runtimes
    print_info "Checking available runtimes..."

    if command -v python3 &> /dev/null; then
        print_success "Python3 runtime available: $(python3 --version 2>&1)"
    else
        print_warning "Python3 not found. Python functions will not be available."
    fi

    if command -v node &> /dev/null; then
        print_success "Node.js runtime available: $(node --version 2>&1)"
    else
        print_warning "Node.js not found. Node.js functions will not be available."
    fi

    echo

    print_success "Functions Service installed successfully!"
    print_info "Functions storage: ${FUNCTIONS_DIR}"
}

# Run main
main "$@"
