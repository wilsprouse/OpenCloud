#!/bin/bash

################################################################################
# Container Registry Service Installer
#
# This script installs and configures Podman for the OpenCloud Container
# Registry service. It delegates all Podman setup to the shared
# containers_base.sh library used by both container services.
################################################################################

set -e
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=containers_base.sh
source "${SCRIPT_DIR}/containers_base.sh"

################################################################################
# Main Script
################################################################################

main() {
    print_info "Starting Container Registry Service Installer"
    echo

    setup_podman
}

# Run main
main "$@"
