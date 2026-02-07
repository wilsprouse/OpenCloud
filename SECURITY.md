# Security Summary

## Known Vulnerabilities

### containerd v1.7.24 - Local Privilege Escalation

**Status**: Known vulnerability present, upgrade blocked by compatibility issues

**Severity**: Medium to High (local privilege escalation)

**CVE**: Not yet assigned / researching exact CVE identifier

**Description**: 
containerd versions < 1.7.29 are affected by a local privilege escalation vulnerability via wide permissions on the CRI directory.

**Affected Version**: 1.7.24 (currently in use)
**Patched Version**: 1.7.29+

**Why Not Upgraded**:
Upgrading to containerd v1.7.29 causes compatibility issues with buildkit v0.16.0:
- buildkit v0.16.0 requires containerd v1.7.x API that changed in v1.7.28+
- Newer buildkit versions (v0.17+) have breaking API changes with our code
- buildkit v0.27+ requires containerd v2.x and Go 1.25, which is a major migration

**Mitigation**:
1. This is a LOCAL privilege escalation vulnerability (requires local access)
2. OpenCloud is designed to run in trusted environments
3. containerd socket permissions should be properly configured (root-only access)
4. Regular security audits of system user permissions

**Remediation Plan**:
1. Monitor buildkit releases for compatibility with containerd v1.7.29+
2. Once compatible versions are available, upgrade both dependencies
3. Consider migrating to buildkit v0.27+ and containerd v2.x in a future major version

**Risk Assessment**:
- **Low** in single-user development environments
- **Medium** in multi-user systems with untrusted local users
- **High** in environments where local users should not have container access

**Recommended Actions**:
- Restrict local user access to the system
- Monitor containerd socket permissions (should be 660 or 600, root:root or root:docker)
- Deploy OpenCloud in containerized/isolated environments
- Keep OpenCloud updated for the security patch when dependencies allow

## Security Features Implemented

### Build Security
1. **Restricted Temp Directory Permissions**: Build contexts use 0700 permissions
2. **Dockerfile Validation**: Basic validation to prevent malformed Dockerfiles
3. **Image Name Validation**: Prevents path traversal and malicious image names
4. **Build Error Reporting**: Clear error messages returned to API consumers
5. **buildkit Requirement**: Requires separate buildkit daemon (not fallback to containerd)

### API Security
1. **CORS Protection**: Validates origin headers
2. **Input Validation**: All API endpoints validate input parameters
3. **Error Handling**: Proper error responses without leaking sensitive information

## Reporting Security Issues

If you discover a security vulnerability in OpenCloud, please email security@wavexsoftware.com or open a confidential security advisory on GitHub.
