#!/bin/bash

################################################################################
# Blob Storage Service Installer
#
# This script sets up the directory structure for the OpenCloud Blob Storage
# service.
################################################################################

set -e
set -o pipefail

################################################################################
# Configuration Variables
################################################################################

readonly OPENCLOUD_DIR="${HOME}/.opencloud"
readonly BLOB_STORAGE_DIR="${OPENCLOUD_DIR}/blob_storage"

################################################################################
# Helper Functions
################################################################################

print_info()    { echo "[INFO] $1"; }
print_success() { echo "[SUCCESS] $1"; }

################################################################################
# Main Script
################################################################################

main() {
    print_info "Starting Blob Storage Service Installer"
    echo

    # Create the blob storage directory (idempotent)
    print_info "Creating blob storage directory at ${BLOB_STORAGE_DIR}..."
    mkdir -p "${BLOB_STORAGE_DIR}"
    print_success "Blob storage directory ready: ${BLOB_STORAGE_DIR}"

    echo

    print_success "Blob Storage Service installed successfully!"
    print_info "Blob storage location: ${BLOB_STORAGE_DIR}"
}

# Run main
main "$@"
