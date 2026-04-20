'use client'

import { useTheme } from "next-themes"
import { Moon, Sun, Globe, Copy, Check } from "lucide-react"
import { Switch } from "@/components/ui/switch"
import { Label } from "@/components/ui/label"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { useState, useEffect } from "react"

export default function SettingsPage() {
  const { theme, setTheme } = useTheme()
  const isDark = theme === "dark"

  const [domain, setDomain] = useState("")
  const [savedDomain, setSavedDomain] = useState("")
  const [domainLoading, setDomainLoading] = useState(false)
  const [domainError, setDomainError] = useState("")

  // nginx instructions returned by the backend after a successful save
  const [nginxConfigLine, setNginxConfigLine] = useState("")
  const [nginxReloadCmd, setNginxReloadCmd] = useState("")

  // Track which code snippet was just copied
  const [copiedKey, setCopiedKey] = useState<string | null>(null)

  // Load the current configured domain on mount.
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
  }, [])

  function handleThemeToggle(checked: boolean) {
    setTheme(checked ? "dark" : "light")
  }

  function copyToClipboard(text: string, key: string) {
    navigator.clipboard.writeText(text).then(() => {
      setCopiedKey(key)
      setTimeout(() => setCopiedKey(null), 2000)
    })
  }

  async function handleDomainSave() {
    setDomainError("")
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
      setNginxConfigLine(data.nginxConfigLine ?? "")
      setNginxReloadCmd(data.nginxReloadCmd ?? "")
    } catch {
      setDomainError("Network error. Please try again.")
    } finally {
      setDomainLoading(false)
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
                    Domain saved! Apply it to nginx by following these steps on your server:
                  </p>

                  <div className="space-y-2">
                    <p className="text-xs text-muted-foreground">
                      1. Open <span className="font-mono">/etc/nginx/sites-available/opencloud</span> and
                      find the <span className="font-mono">server_name</span> line. Replace it with:
                    </p>
                    <div className="flex items-center gap-2 rounded bg-background border px-3 py-2">
                      <code className="flex-1 font-mono text-xs">{nginxConfigLine}</code>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6 shrink-0"
                        onClick={() => copyToClipboard(nginxConfigLine, "configLine")}
                        aria-label="Copy server_name line"
                      >
                        {copiedKey === "configLine"
                          ? <Check className="h-3.5 w-3.5 text-green-500" />
                          : <Copy className="h-3.5 w-3.5" />}
                      </Button>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <p className="text-xs text-muted-foreground">
                      2. Test the configuration and reload nginx:
                    </p>
                    <div className="flex items-center gap-2 rounded bg-background border px-3 py-2">
                      <code className="flex-1 font-mono text-xs">{nginxReloadCmd}</code>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6 shrink-0"
                        onClick={() => copyToClipboard(nginxReloadCmd, "reloadCmd")}
                        aria-label="Copy reload command"
                      >
                        {copiedKey === "reloadCmd"
                          ? <Check className="h-3.5 w-3.5 text-green-500" />
                          : <Copy className="h-3.5 w-3.5" />}
                      </Button>
                    </div>
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

