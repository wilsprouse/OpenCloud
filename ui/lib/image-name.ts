/**
 * Strips the registry hostname prefix (and Docker Hub's "library/" namespace)
 * from a container image name for display purposes.
 *
 * A registry prefix is identified as the leading path segment that contains
 * a '.' or ':' (indicating a hostname or hostname:port), or is "localhost".
 * After stripping the registry host, the Docker Hub official-images namespace
 * "library/" is also stripped, since it is implicit and adds no user-facing info.
 *
 * Examples:
 *   "docker.io/library/nginx:latest"            → "nginx:latest"
 *   "docker.io/myuser/app:latest"               → "myuser/app:latest"
 *   "quay.io/prometheus/prometheus:v2.0"        → "prometheus/prometheus:v2.0"
 *   "ghcr.io/owner/image:tag"                   → "owner/image:tag"
 *   "myregistry.com:5000/myimage:tag"           → "myimage:tag"
 *   "localhost/myimage:tag"                     → "myimage:tag"
 *   "nginx:latest"                              → "nginx:latest"
 */
export function stripRegistryPrefix(name: string): string {
  if (!name) return name
  const slashIndex = name.indexOf("/")
  if (slashIndex === -1) return name
  const firstSegment = name.slice(0, slashIndex)
  if (
    firstSegment.includes(".") ||
    firstSegment.includes(":") ||
    firstSegment === "localhost"
  ) {
    const withoutRegistry = name.slice(slashIndex + 1)
    // Strip the Docker Hub official-images namespace "library/" — it is implicit
    // and carries no meaningful information for the user.
    return withoutRegistry.startsWith("library/")
      ? withoutRegistry.slice("library/".length)
      : withoutRegistry
  }
  return name
}
