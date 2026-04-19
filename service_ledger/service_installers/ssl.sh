#!/bin/bash

################################################################################
# SSL Service Installer
#
# This script installs certbot (with the nginx plugin) and issues a Let's
# Encrypt TLS certificate for the specified domain, then configures nginx to
# serve the site over HTTPS.
#
# Required environment variables
# ─────────────────────────────
#   OPENCLOUD_SSL_DOMAIN  – the fully-qualified domain name (e.g. cloud.example.com)
#   OPENCLOUD_SSL_EMAIL   – contact address for Let's Encrypt expiry notifications
#
# The script is intentionally non-interactive: all prompts are answered via
# certbot's --non-interactive flag and the values supplied through the two
# environment variables above.
################################################################################

set -e
set -o pipefail

################################################################################
# Helper functions
################################################################################

print_info()    { echo "[INFO] $1"; }
print_success() { echo "[SUCCESS] $1"; }
print_error()   { echo "[ERROR] $1" >&2; }

################################################################################
# Validate required environment variables
################################################################################

DOMAIN="${OPENCLOUD_SSL_DOMAIN:-}"
EMAIL="${OPENCLOUD_SSL_EMAIL:-}"

if [ -z "${DOMAIN}" ]; then
    print_error "OPENCLOUD_SSL_DOMAIN environment variable is not set."
    exit 1
fi

if [ -z "${EMAIL}" ]; then
    print_error "OPENCLOUD_SSL_EMAIL environment variable is not set."
    exit 1
fi

################################################################################
# Install certbot and the nginx plugin if not already present
################################################################################

install_certbot() {
    print_info "Updating package index..."
    sudo apt-get update -qq

    print_info "Installing certbot and the nginx plugin..."
    sudo apt-get install -y certbot python3-certbot-nginx

    print_success "certbot installed successfully."
}

################################################################################
# Issue the certificate and configure nginx
################################################################################

configure_ssl() {
    print_info "Requesting Let's Encrypt certificate for: ${DOMAIN}"
    print_info "Contact email: ${EMAIL}"

    sudo certbot --nginx \
        --non-interactive \
        --agree-tos \
        --redirect \
        -d "${DOMAIN}" \
        -m "${EMAIL}"

    print_success "SSL certificate issued and nginx configured for ${DOMAIN}."
    print_info "Reloading nginx to apply the new configuration..."
    sudo systemctl reload nginx
    print_success "nginx reloaded successfully."
}

################################################################################
# Main
################################################################################

main() {
    print_info "Starting SSL configuration for domain: ${DOMAIN}"
    echo

    if command -v certbot &> /dev/null; then
        print_info "certbot is already installed: $(certbot --version 2>&1 | head -n 1)"
    else
        install_certbot
    fi
    echo

    configure_ssl
    echo

    print_success "SSL configuration complete!"
    print_info "Your site is now accessible over HTTPS at https://${DOMAIN}"
}

main "$@"
