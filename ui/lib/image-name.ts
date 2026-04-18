/**
 * Strips the registry hostname prefix from a container image name for display.
 *
 * A registry prefix is identified as the leading path segment that contains
 * a '.' or ':' (indicating a hostname or hostname:port), or is "localhost".
 *
 * Examples:
 *   "docker.io/library/nginx:latest"            → "library/nginx:latest"
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
    return name.slice(slashIndex + 1)
  }
  return name
}
