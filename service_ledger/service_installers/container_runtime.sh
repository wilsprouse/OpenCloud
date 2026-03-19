#!/bin/bash

################################################################################
# Container Runtime Service Installer - slirp4netns Networking Setup
#
# This script installs slirp4netns, a rootless user-space network stack for
# containers. slirp4netns enables port forwarding without requiring root
# privileges, kernel bridge interfaces, or iptables NAT rules — it runs
# entirely in user space using the SLIRP TCP/IP emulation library.
#
# Requirements:
#   - Ubuntu/Debian-based system (preferred) or a system with curl
#   - No root required at container runtime
#
# What this script installs:
#   - slirp4netns binary (via apt-get on Debian/Ubuntu, or directly from the
#     official GitHub release on other distros)
################################################################################

set -e
set -o pipefail

readonly SLIRP4NETNS_FALLBACK_VERSION="v1.3.1"
readonly SLIRP4NETNS_FALLBACK_URL="https://github.com/rootless-containers/slirp4netns/releases/download/${SLIRP4NETNS_FALLBACK_VERSION}/slirp4netns-x86_64"

print_info()    { echo "[INFO] $1"; }
print_success() { echo "[SUCCESS] $1"; }
print_error()   { echo "[ERROR] $1" >&2; }

is_slirp4netns_installed() {
    command -v slirp4netns &>/dev/null
}

install_slirp4netns_binary() {
    print_info "Installing slirp4netns binary from ${SLIRP4NETNS_FALLBACK_URL}..."
    sudo curl -sSL "${SLIRP4NETNS_FALLBACK_URL}" -o /usr/local/bin/slirp4netns
    sudo chmod +x /usr/local/bin/slirp4netns
    print_success "slirp4netns binary installed to /usr/local/bin/slirp4netns"
}

install_slirp4netns() {
    if command -v apt-get &>/dev/null; then
        print_info "Installing slirp4netns via apt-get..."
        if sudo apt-get install -y slirp4netns; then
            return 0
        fi
        print_info "apt-get install failed, falling back to binary download..."
    fi
    install_slirp4netns_binary
}

################################################################################
# Main Script
################################################################################

main() {
    print_info "Starting Container Runtime (slirp4netns) Service Installer"
    echo

    if is_slirp4netns_installed; then
        print_info "slirp4netns is already installed: $(command -v slirp4netns)"
        slirp4netns --version || true
    else
        install_slirp4netns
        print_success "slirp4netns installed: $(command -v slirp4netns)"
        slirp4netns --version || true
    fi

    echo
    print_success "Container Runtime (slirp4netns) setup completed!"
    print_info "Rootless port forwarding enabled via user-space SLIRP networking"
    print_info "No root, iptables, or kernel bridge interfaces required"
}

main "$@"
