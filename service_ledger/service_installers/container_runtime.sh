#!/bin/bash

################################################################################
# Container Runtime Service Installer - CNI Networking Setup
#
# This script installs the official CNI (Container Network Interface) plugins
# required for OpenCloud container port mapping. The CNI bridge and portmap
# plugins create veth pairs, attach containers to the ocni0 bridge, and install
# iptables NAT rules to forward host ports to container ports.
#
# Requirements:
#   - curl
#   - Root or sudo privileges (for /opt/cni/bin and iptables)
#
# What this script installs:
#   - Official CNI plugins v1.5.1 into /opt/cni/bin
#     (bridge, host-local, portmap, loopback, and others)
################################################################################

set -e
set -o pipefail

readonly CNI_VERSION="v1.5.1"
readonly CNI_PLUGINS_URL="https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-linux-amd64-${CNI_VERSION}.tgz"
readonly CNI_BIN_DIR="/opt/cni/bin"

print_info()    { echo "[INFO] $1"; }
print_success() { echo "[SUCCESS] $1"; }
print_error()   { echo "[ERROR] $1" >&2; }

is_cni_installed() {
    [ -x "${CNI_BIN_DIR}/bridge" ] && \
    [ -x "${CNI_BIN_DIR}/portmap" ] && \
    [ -x "${CNI_BIN_DIR}/host-local" ]
}

install_cni_plugins() {
    print_info "Creating CNI bin directory at ${CNI_BIN_DIR}..."
    sudo mkdir -p "${CNI_BIN_DIR}"

    print_info "Downloading CNI plugins ${CNI_VERSION} from GitHub..."
    curl -sSL "${CNI_PLUGINS_URL}" | sudo tar -C "${CNI_BIN_DIR}" -xz

    print_success "CNI plugins extracted to ${CNI_BIN_DIR}"
}

################################################################################
# Main Script
################################################################################

main() {
    print_info "Starting Container Runtime (CNI) Service Installer"
    echo

    if is_cni_installed; then
        print_info "CNI plugins already installed at ${CNI_BIN_DIR}"
        print_info "  bridge:    $(${CNI_BIN_DIR}/bridge 2>/dev/null | head -1 || true)"
    else
        install_cni_plugins
        print_success "CNI plugins installed at ${CNI_BIN_DIR}"
    fi

    echo
    print_success "Container Runtime (CNI) setup completed!"
    print_info "Port forwarding will be handled by the bridge + portmap CNI plugins"
    print_info "iptables NAT rules are created automatically on container start"
}

main "$@"
