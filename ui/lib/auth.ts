/**
 * Logs the user out by clearing authentication data from localStorage and cookies,
 * then redirecting to the login page.
 */
export function logout(): void {
  localStorage.removeItem("access_token")
  localStorage.removeItem("refresh_token")
  localStorage.removeItem("username")

  // Expire the session cookie used by the Next.js middleware for route protection.
  // The Secure flag is conditionally added to match how the cookie was originally set.
  const secure = window.location.protocol === "https:" ? "; Secure" : ""
  document.cookie =
    `opencloud_session=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT; SameSite=Strict${secure}`

  window.location.href = "/login"
}

/**
 * Returns the username of the currently logged-in user, or null if not logged in.
 */
export function getUsername(): string | null {
  if (typeof window === "undefined") return null
  return localStorage.getItem("username")
}

/**
 * Returns true if the user is currently authenticated.
 */
export function isAuthenticated(): boolean {
  if (typeof window === "undefined") return false
  return !!localStorage.getItem("access_token")
}

/**
 * Returns the access token for the current session, or null if not logged in.
 */
export function getAccessToken(): string | null {
  if (typeof window === "undefined") return null
  return localStorage.getItem("access_token")
}
