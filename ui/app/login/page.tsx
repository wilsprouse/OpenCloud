'use client'

import { useState } from "react"
import { useRouter } from "next/navigation"
import { Cloud, Loader2 } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import client from "@/app/utility/post"

export default function LoginPage() {
  const router = useRouter()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError("")

    try {
      const res = await client.post("/user/login", { username, password })

      const accessToken: string | undefined = res.data?.access_token
      const refreshToken: string | undefined = res.data?.refresh_token

      if (!accessToken || !refreshToken) {
        throw new Error("Unexpected response from the server. Please try again.")
      }

      // Persist tokens and username for subsequent requests
      localStorage.setItem("access_token", accessToken)
      localStorage.setItem("refresh_token", refreshToken)
      localStorage.setItem("username", username)

      // Set a session cookie so the Next.js middleware can protect other routes.
      // The Secure flag is added on HTTPS origins to prevent transmission over HTTP.
      const secure = window.location.protocol === "https:" ? "; Secure" : ""
      document.cookie = `opencloud_session=${accessToken}; path=/; SameSite=Strict${secure}`

      router.push("/")
      router.refresh()
    } catch (err: any) {
      setError(
        err.message && !err.response
          ? err.message
          : err.response?.data?.message ||
              "Invalid username or password. Please try again."
      )
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex flex-col items-center justify-center px-4 py-16">
      {/* Branding */}
      <div className="mb-8 flex flex-col items-center gap-2 text-center">
        <div className="flex items-center gap-2">
          <Cloud className="h-8 w-8" />
          <span className="text-2xl font-bold">OpenCloud</span>
        </div>
        <p className="text-sm text-muted-foreground">
          Open Source Cloud Infrastructure Management
        </p>
      </div>

      {/* Login card */}
      <div className="w-full max-w-sm">
        <Card>
          <CardHeader>
            <CardTitle>Sign in to your account</CardTitle>
            <CardDescription>
              Enter your credentials to access your cloud dashboard
            </CardDescription>
          </CardHeader>

          <form onSubmit={handleLogin}>
            <CardContent className="space-y-4">
              {/* Error banner */}
              {error && (
                <div className="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
                  {error}
                </div>
              )}

              <div className="space-y-2">
                <Label htmlFor="username">Username</Label>
                <Input
                  id="username"
                  type="text"
                  placeholder="Enter your username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                  autoComplete="username"
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="password">Password</Label>
                <Input
                  id="password"
                  type="password"
                  placeholder="Enter your password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  autoComplete="current-password"
                />
              </div>
            </CardContent>

            <CardFooter>
              <Button type="submit" className="w-full" disabled={loading}>
                {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {loading ? "Signing in…" : "Sign in"}
              </Button>
            </CardFooter>
          </form>
        </Card>
      </div>
    </div>
  )
}
