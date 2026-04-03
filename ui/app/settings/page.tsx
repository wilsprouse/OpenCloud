'use client'

import { useTheme } from "next-themes"
import { Moon, Sun } from "lucide-react"
import { Switch } from "@/components/ui/switch"
import { Label } from "@/components/ui/label"

export default function SettingsPage() {
  const { theme, setTheme } = useTheme()
  const isDark = theme === "dark"

  function handleToggle(checked: boolean) {
    setTheme(checked ? "dark" : "light")
  }

  return (
    <div className="container mx-auto max-w-2xl px-4 py-10">
      <h1 className="mb-8 text-2xl font-bold">Settings</h1>

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
            onCheckedChange={handleToggle}
            aria-label="Toggle dark mode"
          />
        </div>
      </section>
    </div>
  )
}
