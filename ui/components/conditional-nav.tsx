'use client'

import { usePathname } from "next/navigation"
import { MainNav } from "@/components/main-nav"

/**
 * Renders the sticky top navigation header for every page except the login page.
 * Using a client component here lets us call usePathname() while keeping the
 * root layout as a server component (which is required for metadata exports).
 */
export function ConditionalNav() {
  const pathname = usePathname()

  if (pathname === "/login") {
    return null
  }

  return (
    <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="container flex h-14 items-center">
        <MainNav />
      </div>
    </header>
  )
}
