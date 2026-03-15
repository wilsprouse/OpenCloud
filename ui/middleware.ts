import { NextResponse } from "next/server"
import type { NextRequest } from "next/server"

/**
 * Middleware that protects all application routes behind authentication.
 * Unauthenticated users are redirected to /login; authenticated users
 * attempting to access /login are redirected to the dashboard.
 */
export function middleware(request: NextRequest) {
  const session = request.cookies.get("opencloud_session")
  const { pathname } = request.nextUrl

  // Allow the login page through regardless of auth state
  if (pathname === "/login") {
    // Already logged in — send directly to the dashboard
    if (session) {
      return NextResponse.redirect(new URL("/", request.url))
    }
    return NextResponse.next()
  }

  // Not authenticated — redirect to login
  if (!session) {
    return NextResponse.redirect(new URL("/login", request.url))
  }

  return NextResponse.next()
}

export const config = {
  matcher: [
    // Run on all routes except Next.js internals, static assets, and API proxy
    "/((?!api|_next/static|_next/image|favicon.ico).*)",
  ],
}
