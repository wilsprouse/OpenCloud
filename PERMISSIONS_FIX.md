# BuildKit Socket Permission Fix

## Problem
The OpenCloud service was encountering permission denied errors when trying to connect to the buildkit socket:

```
dial unix /run/buildkit/buildkitd.sock: connect: permission denied
```

This occurred because:
1. The opencloud service runs as the `ubuntu` user
2. The buildkit socket is owned by root and the `buildkit` group
3. The ubuntu user was not in the buildkit group
4. The systemd service file did not specify supplementary groups

## Solution

### 1. Updated Container Registry Installer Script
**File:** `service_ledger/service_installers/container_registry.sh`

Enhanced the `ensure_buildkit_group_access()` function to:
- Add the current user (whoever runs the installer) to the buildkit group
- Explicitly add the `ubuntu` user to the buildkit group
- Provide clear feedback about group membership changes

```bash
# Also add ubuntu user (used by opencloud service) to buildkit group
if id "ubuntu" &>/dev/null; then
    if ! id -nG "ubuntu" | grep -qw "${BUILDKIT_GROUP_NAME}"; then
        sudo usermod -aG "${BUILDKIT_GROUP_NAME}" "ubuntu"
        print_success "Added user ubuntu to ${BUILDKIT_GROUP_NAME} group"
    else
        print_info "User ubuntu is already in ${BUILDKIT_GROUP_NAME} group"
    fi
fi
```

### 2. Updated OpenCloud Service File
**File:** `utils/opencloud.service`

Added `SupplementaryGroups=buildkit` to the systemd service configuration:

```ini
[Service]
User=ubuntu
Group=ubuntu

# Add supplementary groups for buildkit access
SupplementaryGroups=buildkit
```

This ensures that when the opencloud service runs:
- It executes as the `ubuntu` user with primary group `ubuntu`
- It also has access to the `buildkit` group as a supplementary group
- It can connect to the buildkit socket at `/run/buildkit/buildkitd.sock`

Also added the `[Install]` section to properly enable the service:
```ini
[Install]
WantedBy=multi-user.target
```

## How It Works

### Group Membership
When the Container Registry service is enabled:
1. The buildkit group is created (if it doesn't exist)
2. The ubuntu user is added to the buildkit group
3. The buildkit socket directory `/run/buildkit` is owned by `root:buildkit` with permissions `2775`
4. The buildkit socket is created with group ownership allowing buildkit group members to connect

### Service Configuration
The opencloud service:
1. Runs as `User=ubuntu` with `Group=ubuntu`
2. Uses `SupplementaryGroups=buildkit` to gain access to the buildkit group
3. Can now connect to the buildkit socket because it has buildkit group membership

### Why Both Changes Are Needed
- **Group membership alone** is not enough because systemd services don't automatically pick up supplementary groups
- **SupplementaryGroups directive** tells systemd to explicitly grant the service access to the buildkit group
- Together, these ensure the service has the proper permissions

## Verification

After running the installer and restarting the opencloud service, you can verify:

```bash
# Check ubuntu user is in buildkit group
id ubuntu | grep buildkit

# Check service has supplementary groups configured
systemctl cat opencloud.service | grep SupplementaryGroups

# Check buildkit socket permissions
ls -l /run/buildkit/buildkitd.sock

# Expected output:
# srw-rw---- 1 root buildkit 0 ... /run/buildkit/buildkitd.sock

# Verify service can connect (check logs for no permission errors)
sudo journalctl -u opencloud.service -f
```

## Benefits

1. **Automatic Setup**: The installer handles all permission configuration
2. **Secure**: Uses standard Unix group-based permissions
3. **Maintainable**: Clear separation of concerns - installer manages groups, service file declares needs
4. **Works After Reboot**: Group membership and service configuration persist

## Files Modified

1. `service_ledger/service_installers/container_registry.sh` - Added ubuntu user to buildkit group
2. `utils/opencloud.service` - Added SupplementaryGroups directive and [Install] section

---

**Fixed by**: GitHub Copilot  
**Date**: 2026-02-10
