'use client'

import { useEffect, useRef, useState } from "react"
import { useTheme } from "next-themes"
import { Moon, Sun, Lock, CheckCircle, AlertCircle, ExternalLink } from "lucide-react"
import { Switch } from "@/components/ui/switch"
import { Label } from "@/components/ui/label"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { toast } from "sonner"

// Represents the SSL status returned by GET /api/ssl-status.
type SSLStatus = {
  configured: boolean
  domains?: string[]
}

export default function SettingsPage() {
  const { theme, setTheme } = useTheme()
  const isDark = theme === "dark"

  // ── SSL state ────────────────────────────────────────────────────────────
  const [sslStatus, setSslStatus] = useState<SSLStatus | null>(null)
  const [sslDialogOpen, setSslDialogOpen] = useState(false)
  const [sslDomain, setSslDomain] = useState("")
  const [sslEmail, setSslEmail] = useState("")
  const [sslAgreeTOS, setSslAgreeTOS] = useState(false)
  const [sslConfiguring, setSslConfiguring] = useState(false)
  const [sslOutput, setSslOutput] = useState<string[]>([])
  const [sslError, setSslError] = useState<string | null>(null)
  const outputEndRef = useRef<HTMLDivElement | null>(null)

  // Auto-scroll the output pane as new lines arrive.
  useEffect(() => {
    outputEndRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [sslOutput])

  // Fetch SSL status when the page loads.
  useEffect(() => {
    fetchSSLStatus()
  }, [])

  function handleThemeToggle(checked: boolean) {
    setTheme(checked ? "dark" : "light")
  }

  async function fetchSSLStatus() {
    try {
      const res = await fetch("/api/ssl-status")
      if (!res.ok) return
      const data: SSLStatus = await res.json()
      setSslStatus(data)
    } catch {
      // Silently ignore – the status badge simply won't appear.
    }
  }

  function handleOpenSSLDialog() {
    setSslOutput([])
    setSslError(null)
    setSslDialogOpen(true)
  }

  function handleCloseSSLDialog() {
    if (sslConfiguring) return // block close while running
    setSslDialogOpen(false)
    // Reset form fields between sessions.
    setSslDomain("")
    setSslEmail("")
    setSslAgreeTOS(false)
    setSslOutput([])
    setSslError(null)
  }

  async function handleConfigureSSL() {
    const domain = sslDomain.trim()
    const email = sslEmail.trim()

    if (!domain) {
      toast.error("Please enter a domain name.")
      return
    }
    if (!email) {
      toast.error("Please enter an email address.")
      return
    }
    if (!sslAgreeTOS) {
      toast.error("You must agree to the Let's Encrypt Terms of Service.")
      return
    }

    setSslConfiguring(true)
    setSslOutput([])
    setSslError(null)

    const appendLine = (line: string) =>
      setSslOutput(prev => [...prev, line])

    try {
      const response = await fetch("/api/configure-ssl-stream", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ domain, email, agree_tos: true }),
      })

      if (!response.ok) {
        const text = await response.text()
        throw new Error(text || `Server returned ${response.status}`)
      }

      if (!response.body) {
        throw new Error("No response body from server.")
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ""
      let succeeded = false

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split("\n")
        buffer = lines.pop() ?? ""

        for (const line of lines) {
          if (line.startsWith("data: ")) {
            const data = line.slice(6).trim()
            if (data) appendLine(data)
          } else if (line.startsWith("event: done")) {
            succeeded = true
          } else if (line.startsWith("event: error")) {
            // Error text arrives on the following data: line – handled above.
          }
        }
      }

      if (succeeded) {
        toast.success(`SSL configured successfully for ${domain}!`)
        await fetchSSLStatus()
        // Leave the dialog open briefly so the user can read the output.
        setTimeout(() => setSslDialogOpen(false), 2000)
      } else {
        setSslError("SSL configuration failed. Review the output above for details.")
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "SSL configuration failed."
      setSslError(msg)
    } finally {
      setSslConfiguring(false)
    }
  }

  return (
    <div className="container mx-auto max-w-2xl px-4 py-10">
      <h1 className="mb-8 text-2xl font-bold">Settings</h1>

      {/* ── Appearance ───────────────────────────────────────────────────── */}
      <section className="rounded-lg border p-6">
        <h2 className="mb-4 text-lg font-semibold">Appearance</h2>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            {isDark ? (
              <Moon className="h-5 w-5 text-muted-foreground" />
            ) : (
              <Sun className="h-5 w-5 text-muted-foreground" />
            )}
            <div>
              <Label htmlFor="dark-mode-toggle" className="text-sm font-medium">
                Dark Mode
              </Label>
              <p className="text-xs text-muted-foreground">
                {isDark ? "Currently using dark mode" : "Currently using light mode"}
              </p>
            </div>
          </div>
          <Switch
            id="dark-mode-toggle"
            checked={isDark}
            onCheckedChange={handleThemeToggle}
            aria-label="Toggle dark mode"
          />
        </div>
      </section>

      {/* ── Instance Management ──────────────────────────────────────────── */}
      <section className="mt-8 rounded-lg border p-6">
        <h2 className="mb-1 text-lg font-semibold">Instance Management</h2>
        <p className="mb-6 text-sm text-muted-foreground">
          Manage system-level settings for this OpenCloud instance.
        </p>

        {/* SSL / HTTPS configuration card */}
        <div className="flex items-start justify-between gap-4 rounded-lg border p-4">
          <div className="flex items-start gap-3">
            <Lock className="mt-0.5 h-5 w-5 shrink-0 text-muted-foreground" />
            <div>
              <p className="text-sm font-medium">SSL / HTTPS (Let&apos;s Encrypt)</p>
              <p className="text-xs text-muted-foreground">
                Secure your instance with a free TLS certificate from Let&apos;s Encrypt.
                Requires a publicly reachable domain name and port 80/443 open.
              </p>

              {/* Current SSL status badge */}
              {sslStatus && (
                <div className="mt-2 flex items-center gap-1.5">
                  {sslStatus.configured ? (
                    <>
                      <CheckCircle className="h-4 w-4 text-green-500" />
                      <span className="text-xs font-medium text-green-600 dark:text-green-400">
                        Active – {sslStatus.domains?.join(", ")}
                      </span>
                    </>
                  ) : (
                    <>
                      <AlertCircle className="h-4 w-4 text-yellow-500" />
                      <span className="text-xs text-muted-foreground">
                        Not configured
                      </span>
                    </>
                  )}
                </div>
              )}
            </div>
          </div>

          <Button
            variant="outline"
            size="sm"
            className="shrink-0"
            onClick={handleOpenSSLDialog}
          >
            Configure SSL
          </Button>
        </div>
      </section>

      {/* ── SSL Configuration Dialog ─────────────────────────────────────── */}
      <Dialog open={sslDialogOpen} onOpenChange={open => { if (!open) handleCloseSSLDialog() }}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Configure SSL with Let&apos;s Encrypt</DialogTitle>
            <DialogDescription>
              Enter your domain name and email to request a free TLS certificate.
              The domain must point to this server&apos;s public IP address before
              you proceed.{" "}
              <a
                href="https://letsencrypt.org/getting-started/"
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-0.5 underline"
              >
                Learn more
                <ExternalLink className="h-3 w-3" />
              </a>
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            {/* Domain field */}
            <div className="space-y-1.5">
              <Label htmlFor="ssl-domain">Domain Name</Label>
              <Input
                id="ssl-domain"
                placeholder="cloud.example.com"
                value={sslDomain}
                onChange={e => setSslDomain(e.target.value)}
                disabled={sslConfiguring}
              />
              <p className="text-xs text-muted-foreground">
                The fully-qualified domain name (FQDN) pointing to this server.
              </p>
            </div>

            {/* Email field */}
            <div className="space-y-1.5">
              <Label htmlFor="ssl-email">Email Address</Label>
              <Input
                id="ssl-email"
                type="email"
                placeholder="admin@example.com"
                value={sslEmail}
                onChange={e => setSslEmail(e.target.value)}
                disabled={sslConfiguring}
              />
              <p className="text-xs text-muted-foreground">
                Used by Let&apos;s Encrypt to send certificate expiry notifications.
              </p>
            </div>

            {/* Terms of Service agreement */}
            <div className="flex items-start gap-2">
              <input
                id="ssl-tos"
                type="checkbox"
                checked={sslAgreeTOS}
                onChange={e => setSslAgreeTOS(e.target.checked)}
                disabled={sslConfiguring}
                className="mt-0.5 h-4 w-4 cursor-pointer rounded border border-input"
              />
              <Label htmlFor="ssl-tos" className="cursor-pointer text-sm leading-snug">
                I agree to the{" "}
                <a
                  href="https://letsencrypt.org/documents/LE-SA-v1.4-April-3-2024.pdf"
                  target="_blank"
                  rel="noreferrer"
                  className="underline"
                >
                  Let&apos;s Encrypt Terms of Service
                </a>
                .
              </Label>
            </div>

            {/* Streaming output pane – shown once configuration begins */}
            {(sslConfiguring || sslOutput.length > 0) && (
              <div className="max-h-48 overflow-y-auto rounded-md bg-black p-3 font-mono text-xs text-green-400">
                {sslOutput.map((line, i) => (
                  <div key={i}>{line}</div>
                ))}
                <div ref={outputEndRef} />
              </div>
            )}

            {/* Error message */}
            {sslError && (
              <p className="text-sm text-red-500">{sslError}</p>
            )}
          </div>

          <DialogFooter>
            <Button
              variant="outline"
              onClick={handleCloseSSLDialog}
              disabled={sslConfiguring}
            >
              Cancel
            </Button>
            <Button
              onClick={handleConfigureSSL}
              disabled={sslConfiguring || !sslDomain || !sslEmail || !sslAgreeTOS}
            >
              {sslConfiguring ? "Configuring…" : "Configure SSL"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
