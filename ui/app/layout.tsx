import type React from "react"
import "@/app/globals.css"
import { Inter } from "next/font/google"
import { ThemeProvider } from "@/components/theme-provider"
import { ConditionalNav } from "@/components/conditional-nav"
import { ConditionalFooter } from "@/components/conditional-footer"
import { Toaster } from "sonner"

const inter = Inter({ subsets: ["latin"] })

export const metadata = {
  title: "OpenCloud - Open Source Cloud Software",
  description: "A modern cloud computing platform for your infrastructure needs",
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en">
      <body className={inter.className}>
        <ThemeProvider attribute="class" defaultTheme="light" enableSystem>
          <div className="flex min-h-screen flex-col">
            <ConditionalNav />
            <main className="flex-1">{children}</main>
            <ConditionalFooter />
          </div>
          <Toaster />
        </ThemeProvider>
      </body>
    </html>
  )
}
