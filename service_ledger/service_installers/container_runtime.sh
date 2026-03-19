#!/bin/bash

################################################################################
# Container Runtime Service Installer - CNI Networking Setup
#
# This script installs and configures the Container Network Interface (CNI)
# plugins for the OpenCloud Container Runtime Service. CNI is used to provide
# network namespace isolation and port mapping (iptables NAT rules) for
# containers managed by containerd.
#
# Requirements:
#   - Ubuntu/Debian-based system
#   - sudo privileges for package/directory operations
#   - containerd already installed (see container_registry.sh)
#   - curl available for downloading CNI plugins
#   - iptables/nftables for portmap plugin NAT rules
#
# What this script installs:
#   - CNI plugins (bridge, host-local, portmap, loopback, etc.)
#     at /opt/cni/bin
#   - OpenCloud CNI network configuration at /etc/cni/net.d/opencloud.conflist
#
# The "opencloud" CNI network uses:
#   - bridge plugin: creates a virtual bridge (ocni0) and veth pairs
#   - host-local IPAM: assigns IPs from 10.88.0.0/16
#   - portmap plugin: installs iptables NAT rules for -p host:container mappings
#   - loopback plugin: sets up the loopback interface inside each container
################################################################################

set -e
set -o pipefail

################################################################################
# Configuration Variables
################################################################################

readonly CNI_VERSION="v1.5.1"
readonly CNI_PLUGINS_URL="https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-linux-amd64-${CNI_VERSION}.tgz"
readonly CNI_BIN_DIR="/opt/cni/bin"
readonly CNI_CONF_DIR="/etc/cni/net.d"
readonly CNI_CONF_FILE="${CNI_CONF_DIR}/opencloud.conflist"

################################################################################
# Helper Functions
################################################################################

print_info()    { echo "[INFO] $1"; }
print_success() { echo "[SUCCESS] $1"; }
print_error()   { echo "[ERROR] $1" >&2; }

check_cni_installed() {
    [ -x "${CNI_BIN_DIR}/bridge" ] && \
    [ -x "${CNI_BIN_DIR}/portmap" ] && \
    [ -x "${CNI_BIN_DIR}/host-local" ] && \
    [ -x "${CNI_BIN_DIR}/loopback" ]
}

# Install CNI plugins from the official containernetworking/plugins release
install_cni_plugins() {
    print_info "Installing CNI plugins ${CNI_VERSION}..."

    local temp_dir
    temp_dir=$(mktemp -d)
    trap "rm -rf ${temp_dir}" EXIT

    sudo mkdir -p "${CNI_BIN_DIR}"

    print_info "Downloading CNI plugins from ${CNI_PLUGINS_URL}..."
    curl -sSL "${CNI_PLUGINS_URL}" -o "${temp_dir}/cni-plugins.tgz"

    print_info "Extracting CNI plugins to ${CNI_BIN_DIR}..."
    sudo tar -C "${CNI_BIN_DIR}" -xzf "${temp_dir}/cni-plugins.tgz"

    print_success "CNI plugins installed to ${CNI_BIN_DIR}"
}

# Create the OpenCloud CNI network configuration.
# The "opencloud" network provides:
#   - A bridge (ocni0) with IP masquerading for container internet access
#   - host-local IPAM from 10.88.0.0/16
#   - portmap plugin for host-to-container port forwarding via iptables NAT
#   - loopback for the container lo interface
create_cni_config() {
    print_info "Creating OpenCloud CNI network config at ${CNI_CONF_FILE}..."

    sudo mkdir -p "${CNI_CONF_DIR}"

    sudo tee "${CNI_CONF_FILE}" > /dev/null <<'EOF'
{
  "cniVersion": "1.0.0",
  "name": "opencloud",
  "plugins": [
    {
      "type": "bridge",
      "bridge": "ocni0",
      "isGateway": true,
      "ipMasq": true,
      "ipam": {
        "type": "host-local",
        "subnet": "10.88.0.0/16",
        "routes": [{ "dst": "0.0.0.0/0" }]
      }
    },
    {
      "type": "portmap",
      "capabilities": {
        "portMappings": true
      }
    },
    {
      "type": "loopback"
    }
  ]
}
EOF
    sudo chmod 755 "${CNI_CONF_DIR}"
    sudo chmod 644 "${CNI_CONF_FILE}"

    print_success "CNI config written to ${CNI_CONF_FILE}"
}

# Verify that the required CNI plugins and config are in place
verify_cni() {
    local ok=true

    for plugin in bridge portmap host-local loopback; do
        if [ -x "${CNI_BIN_DIR}/${plugin}" ]; then
            print_success "CNI plugin present: ${plugin}"
        else
            print_error "CNI plugin missing: ${plugin}"
            ok=false
        fi
    done

    if sudo test -f "${CNI_CONF_FILE}"; then
        print_success "CNI config present: ${CNI_CONF_FILE}"
    else
        print_error "CNI config missing: ${CNI_CONF_FILE}"
        ok=false
    fi

    if [ "$ok" = false ]; then
        print_error "CNI verification failed"
        return 1
    fi
}

################################################################################
# Main Script
################################################################################

main() {
    print_info "Starting Container Runtime (CNI) Service Installer"
    echo

    # Step 1: CNI plugins
    if check_cni_installed; then
        print_info "CNI plugins already installed in ${CNI_BIN_DIR}"
    else
        install_cni_plugins
    fi
    echo

    # Step 2: CNI network configuration
    if sudo test -f "${CNI_CONF_FILE}"; then
        print_info "CNI config already exists at ${CNI_CONF_FILE}"
    else
        create_cni_config
    fi
    echo

    # Step 3: Verify
    verify_cni
    echo

    print_success "Container Runtime (CNI) setup completed successfully!"
    print_info "CNI plugins directory: ${CNI_BIN_DIR}"
    print_info "CNI config file:       ${CNI_CONF_FILE}"
    print_info "Container network:     10.88.0.0/16 via bridge ocni0"
    print_info "Port mapping:          iptables NAT via portmap plugin"
}

# Run main
main "$@"
