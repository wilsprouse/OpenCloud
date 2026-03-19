#!/bin/bash

################################################################################
# Containers Service Installer
#
# This script installs the dependencies required for the OpenCloud Containers
# service:
#   1. container_registry.sh — installs containerd and buildkit (container
#      runtime and image build infrastructure)
#   2. container_runtime.sh  — installs slirp4netns for rootless port
#      forwarding without requiring root or iptables
#
# Requirements:
#   - Ubuntu/Debian-based system
#   - sudo privileges
#   - See container_registry.sh and container_runtime.sh for full requirements
################################################################################

set -e
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "[INFO] Starting Containers Service Installer"
echo

# Install containerd and buildkit (required container runtime dependencies)
echo "[INFO] Installing container runtime dependencies via container_registry.sh..."
bash "${SCRIPT_DIR}/container_registry.sh"

echo

# Install slirp4netns for rootless port forwarding
echo "[INFO] Installing rootless networking dependencies via container_runtime.sh..."
bash "${SCRIPT_DIR}/container_runtime.sh"

echo
echo "[SUCCESS] Containers service dependencies installed successfully!"
