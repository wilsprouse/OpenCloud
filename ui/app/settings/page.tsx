'use client'

import { useTheme } from "next-themes"
import { Moon, Sun, Globe, Lock, Copy, Check, AlertCircle } from "lucide-react"
import { Switch } from "@/components/ui/switch"
import { Label } from "@/components/ui/label"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { useState, useEffect } from "react"

export default function SettingsPage() {
  const { theme, setTheme } = useTheme()
  const isDark = theme === "dark"

  const [domain, setDomain] = useState("")
  const [savedDomain, setSavedDomain] = useState("")
  const [domainLoading, setDomainLoading] = useState(false)
  const [domainError, setDomainError] = useState("")

  // nginx instructions returned by the backend after a successful save
  const [nginxEditCmd, setNginxEditCmd] = useState("")
  const [nginxConfigLine, setNginxConfigLine] = useState("")
  const [nginxReloadCmd, setNginxReloadCmd] = useState("")

  // SSL certificate state
  const [sslEmail, setSSLEmail] = useState("")
  const [savedSSLEmail, setSavedSSLEmail] = useState("")
  const [sslAgreeToTos, setSSLAgreeToTos] = useState(false)
  const [sslLoading, setSSLLoading] = useState(false)
  const [sslError, setSSLError] = useState("")
  const [sslCertbotInstallCmd, setSSLCertbotInstallCmd] = useState("")
  const [sslCertbotCmd, setSSLCertbotCmd] = useState("")
  const [sslAutoRenewCmd, setSSLAutoRenewCmd] = useState("")

  // Track which code snippet was just copied
  const [copiedKey, setCopiedKey] = useState<string | null>(null)

  // Load the current configured domain and SSL email on mount.
  useEffect(() => {
    fetch("/api/get-instance-domain")
      .then((res) => res.json())
      .then((data) => {
        if (data.domain) {
          setSavedDomain(data.domain)
          setDomain(data.domain)
        }
      })
      .catch(() => {
        // Non-fatal: domain may not be configured yet.
      })
    fetch("/api/get-ssl-status")
      .then((res) => res.json())
      .then((data) => {
        if (data.email) {
          setSavedSSLEmail(data.email)
          setSSLEmail(data.email)
        }
      })
      .catch(() => {
        // Non-fatal: SSL may not be configured yet.
      })
  }, [])

  function handleThemeToggle(checked: boolean) {
    setTheme(checked ? "dark" : "light")
  }

  function copyToClipboard(text: string, key: string) {
    // navigator.clipboard requires a secure context (HTTPS / localhost).
    // Fall back to the legacy execCommand approach on plain HTTP.
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text).then(() => {
        setCopiedKey(key)
        setTimeout(() => setCopiedKey(null), 2000)
      }).catch(() => execCommandCopy(text, key))
    } else {
      execCommandCopy(text, key)
    }
  }

  function execCommandCopy(text: string, key: string) {
    const textarea = document.createElement("textarea")
    textarea.value = text
    textarea.style.position = "fixed"
    textarea.style.opacity = "0"
    document.body.appendChild(textarea)
    textarea.focus()
    textarea.select()
    try {
      const ok = document.execCommand("copy")
      if (ok) {
        setCopiedKey(key)
        setTimeout(() => setCopiedKey(null), 2000)
      }
    } finally {
      document.body.removeChild(textarea)
    }
  }

  async function handleDomainSave() {
    setDomainError("")
    setNginxEditCmd("")
    setNginxConfigLine("")
    setNginxReloadCmd("")

    if (!domain.trim()) {
      setDomainError("Domain cannot be empty.")
      return
    }

    setDomainLoading(true)
    try {
      const res = await fetch("/api/set-instance-domain", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ domain: domain.trim() }),
      })

      if (!res.ok) {
        const text = await res.text()
        setDomainError(text || "Failed to configure domain.")
        return
      }

      const data = await res.json()
      setSavedDomain(data.domain)
      setNginxEditCmd(data.nginxEditCmd ?? "")
      setNginxConfigLine(data.nginxConfigLine ?? "")
      setNginxReloadCmd(data.nginxReloadCmd ?? "")
    } catch {
      setDomainError("Network error. Please try again.")
    } finally {
      setDomainLoading(false)
    }
  }

  async function handleSSLConfigure() {
    setSSLError("")
    setSSLCertbotInstallCmd("")
    setSSLCertbotCmd("")
    setSSLAutoRenewCmd("")

    if (!domain.trim() && !savedDomain) {
      setSSLError("Please configure an instance domain before setting up SSL.")
      return
    }

    if (!sslEmail.trim()) {
      setSSLError("Email address cannot be empty.")
      return
    }

    if (!sslAgreeToTos) {
      setSSLError("You must agree to the Let's Encrypt Terms of Service.")
      return
    }

    setSSLLoading(true)
    try {
      const res = await fetch("/api/configure-ssl", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          domain: (domain.trim() || savedDomain),
          email: sslEmail.trim(),
          agreeToTos: sslAgreeToTos,
        }),
      })

      if (!res.ok) {
        const text = await res.text()
        setSSLError(text || "Failed to configure SSL.")
        return
      }

      const data = await res.json()
      setSavedSSLEmail(data.email)
      setSSLCertbotInstallCmd(data.certbotInstallCmd ?? "")
      setSSLCertbotCmd(data.certbotCmd ?? "")
      setSSLAutoRenewCmd(data.autoRenewCmd ?? "")
    } catch {
      setSSLError("Network error. Please try again.")
    } finally {
      setSSLLoading(false)
    }
  }

  return (
    <div className="container mx-auto max-w-2xl px-4 py-10">
      <h1 className="mb-8 text-2xl font-bold">Settings</h1>

      <section className="rounded-lg border p-6 mb-6">
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

      <section className="rounded-lg border p-6">
        <h2 className="mb-4 text-lg font-semibold">Instance Management</h2>

        {/* Root-permission notice */}
        <div className="flex items-start gap-2 rounded-md border border-yellow-300 bg-yellow-50 dark:border-yellow-700 dark:bg-yellow-950/30 px-3 py-2 mb-4 text-xs text-yellow-800 dark:text-yellow-300">
          <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" />
          <span>
            OpenCloud does not have root permissions and cannot modify nginx configurations
            directly. After saving, follow the instructions below to apply the change manually.
          </span>
        </div>

        <div className="space-y-4">
          <div className="flex items-start gap-3">
            <Globe className="h-5 w-5 text-muted-foreground mt-0.5 shrink-0" />
            <div className="flex-1 space-y-3">
              <div>
                <Label htmlFor="instance-domain" className="text-sm font-medium">
                  Instance Domain
                </Label>
                <p className="text-xs text-muted-foreground mt-0.5">
                  Set the domain that nginx should route to this OpenCloud instance.
                  {savedDomain && (
                    <span className="ml-1 font-medium text-foreground">
                      Current: <span className="font-mono">{savedDomain}</span>
                    </span>
                  )}
                </p>
              </div>

              <div className="flex gap-2">
                <Input
                  id="instance-domain"
                  placeholder="e.g. cloud.example.com"
                  value={domain}
                  onChange={(e) => {
                    setDomain(e.target.value)
                    setDomainError("")
                    setNginxEditCmd("")
                    setNginxConfigLine("")
                    setNginxReloadCmd("")
                  }}
                  className="font-mono"
                  disabled={domainLoading}
                />
                <Button onClick={handleDomainSave} disabled={domainLoading}>
                  {domainLoading ? "Saving…" : "Save"}
                </Button>
              </div>

              {domainError && (
                <p className="text-xs text-destructive" role="alert">{domainError}</p>
              )}

              {/* Nginx instructions shown after a successful save */}
              {nginxConfigLine && (
                <div className="rounded-md border bg-muted/50 p-4 space-y-4 text-sm">
                  <p className="font-medium">
                    Domain saved! Apply it to nginx by running these commands on your server:
                  </p>

                  <div className="space-y-2">
                    <p className="text-xs text-muted-foreground">
                      1. Open the nginx config file with your preferred editor (requires <span className="font-mono">sudo</span>):
                    </p>
                    <CodeRow
                      code={nginxEditCmd}
                      copyKey="editCmd"
                      copiedKey={copiedKey}
                      onCopy={copyToClipboard}
                    />
                  </div>

                  <div className="space-y-2">
                    <p className="text-xs text-muted-foreground">
                      2. Find the <span className="font-mono">server_name</span> line and replace it with:
                    </p>
                    <CodeRow
                      code={nginxConfigLine}
                      copyKey="configLine"
                      copiedKey={copiedKey}
                      onCopy={copyToClipboard}
                    />
                  </div>

                  <div className="space-y-2">
                    <p className="text-xs text-muted-foreground">
                      3. Test the configuration and reload nginx:
                    </p>
                    <CodeRow
                      code={nginxReloadCmd}
                      copyKey="reloadCmd"
                      copiedKey={copiedKey}
                      onCopy={copyToClipboard}
                    />
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Divider */}
          <div className="border-t pt-4" />

          {/* SSL Certificate section */}
          <div className="flex items-start gap-3">
            <Lock className="h-5 w-5 text-muted-foreground mt-0.5 shrink-0" />
            <div className="flex-1 space-y-3">
              <div>
                <Label className="text-sm font-medium">SSL Certificate (Let&apos;s Encrypt)</Label>
                <p className="text-xs text-muted-foreground mt-0.5">
                  Obtain a free SSL certificate from Let&apos;s Encrypt for your instance domain.
                  {savedSSLEmail && (
                    <span className="ml-1 font-medium text-foreground">
                      Configured for: <span className="font-mono">{savedSSLEmail}</span>
                    </span>
                  )}
                </p>
              </div>

              <div className="space-y-3">
                <div>
                  <Label htmlFor="ssl-email" className="text-xs text-muted-foreground mb-1 block">
                    Email address
                  </Label>
                  <Input
                    id="ssl-email"
                    type="email"
                    placeholder="e.g. admin@example.com"
                    value={sslEmail}
                    onChange={(e) => {
                      setSSLEmail(e.target.value)
                      setSSLError("")
                      setSSLCertbotInstallCmd("")
                      setSSLCertbotCmd("")
                      setSSLAutoRenewCmd("")
                    }}
                    className="font-mono"
                    disabled={sslLoading}
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    Used by Let&apos;s Encrypt to send certificate expiry notices and recovery emails.
                  </p>
                </div>

                <div className="flex items-start gap-2">
                  <Checkbox
                    id="ssl-tos"
                    checked={sslAgreeToTos}
                    onCheckedChange={(checked: boolean | "indeterminate") => {
                      setSSLAgreeToTos(checked === true)
                      setSSLError("")
                    }}
                    disabled={sslLoading}
                  />
                  <Label htmlFor="ssl-tos" className="text-xs leading-relaxed cursor-pointer">
                    I agree to the{" "}
                    <a
                      href="https://letsencrypt.org/documents/LE-SA-v1.3-September-21-2022.pdf"
                      target="_blank"
                      rel="noopener noreferrer"
                      className="underline text-primary"
                    >
                      Let&apos;s Encrypt Terms of Service
                    </a>
                  </Label>
                </div>

                <Button
                  onClick={handleSSLConfigure}
                  disabled={sslLoading || !sslAgreeToTos}
                  className="w-full sm:w-auto"
                >
                  {sslLoading ? "Generating commands…" : "Configure SSL"}
                </Button>
              </div>

              {sslError && (
                <p className="text-xs text-destructive" role="alert">{sslError}</p>
              )}

              {/* Certbot instructions shown after a successful configure */}
              {sslCertbotCmd && (
                <div className="rounded-md border bg-muted/50 p-4 space-y-4 text-sm">
                  <p className="font-medium">
                    Ready to install! Run these commands on your server to obtain the SSL certificate:
                  </p>

                  <div className="space-y-2">
                    <p className="text-xs text-muted-foreground">
                      1. Install certbot and the nginx plugin (if not already installed):
                    </p>
                    <CodeRow
                      code={sslCertbotInstallCmd}
                      copyKey="sslInstall"
                      copiedKey={copiedKey}
                      onCopy={copyToClipboard}
                    />
                  </div>

                  <div className="space-y-2">
                    <p className="text-xs text-muted-foreground">
                      2. Obtain and install the SSL certificate:
                    </p>
                    <CodeRow
                      code={sslCertbotCmd}
                      copyKey="sslCertbot"
                      copiedKey={copiedKey}
                      onCopy={copyToClipboard}
                    />
                  </div>

                  <div className="space-y-2">
                    <p className="text-xs text-muted-foreground">
                      3. Verify that automatic renewal is configured:
                    </p>
                    <CodeRow
                      code={sslAutoRenewCmd}
                      copyKey="sslAutoRenew"
                      copiedKey={copiedKey}
                      onCopy={copyToClipboard}
                    />
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </section>
    </div>
  )
}

interface CodeRowProps {
  code: string
  copyKey: string
  copiedKey: string | null
  onCopy: (text: string, key: string) => void
}

function CodeRow({ code, copyKey, copiedKey, onCopy }: CodeRowProps) {
  return (
    <div className="flex items-center gap-2 rounded bg-background border px-3 py-2">
      <code className="flex-1 font-mono text-xs break-all">{code}</code>
      <Button
        variant="ghost"
        size="icon"
        className="h-6 w-6 shrink-0"
        onClick={() => onCopy(code, copyKey)}
        aria-label="Copy to clipboard"
      >
        {copiedKey === copyKey
          ? <Check className="h-3.5 w-3.5 text-green-500" />
          : <Copy className="h-3.5 w-3.5" />}
      </Button>
    </div>
  )
}


