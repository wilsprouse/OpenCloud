'use client'

import { usePathname } from "next/navigation"
import { Footer } from "@/components/footer"

/**
 * Renders the footer on every page except the login page.
 * Using a client component here lets us call usePathname() while keeping the
 * root layout as a server component (which is required for metadata exports).
 */
export function ConditionalFooter() {
  const pathname = usePathname()

  if (pathname === "/login") {
    return null
  }

  return <Footer />
}
