"use client"

import { useEffect, useState } from "react"
import { ChevronRight, Container, Globe, HardDrive, Zap } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { DashboardHeader } from "@/components/dashboard-header"
import { DashboardShell } from "@/components/dashboard-shell"
import { RecentServices } from "@/components/recent-services"
import { ServiceQuickActions } from "@/components/service-quick-actions"
import { ServerMetrics } from "@/components/server-metrics"
import { getUsername } from "@/lib/auth"
import client from "@/app/utility/post"

type BlobContainer = {
  name: string
  objectCount: number
  totalSize: number
  lastModified: string
}

type FunctionItem = {
  id: string
  name: string
  runtime: string
  status: string
  lastModified: string
}

type ContainerItem = {
  Id: string
  Names: string[]
  Image: string
  State: string
  Status: string
  Created: number
}

// Returns a human-readable relative time string (e.g. "5 min ago") from an ISO date string.
function formatRelativeTime(isoString: string): string {
  const date = new Date(isoString)
  const now = new Date()
  const diffSec = Math.max(0, Math.floor((now.getTime() - date.getTime()) / 1000))
  if (diffSec < 60) return `${diffSec} sec ago`
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin} min ago`
  const diffHr = Math.floor(diffMin / 60)
  if (diffHr < 24) return `${diffHr} hour${diffHr !== 1 ? "s" : ""} ago`
  const diffDay = Math.floor(diffHr / 24)
  return `${diffDay} day${diffDay !== 1 ? "s" : ""} ago`
}

export default function DashboardPage() {
  const [username, setUsername] = useState<string | null>(null)
  const [bucketCount, setBucketCount] = useState<number>(0)
  const [blobLastUsed, setBlobLastUsed] = useState<string>("Never")
  const [functionCount, setFunctionCount] = useState<number>(0)
  const [functionLastUsed, setFunctionLastUsed] = useState<string>("Never")
  const [containerCount, setContainerCount] = useState<number>(0)
  const [containerLastUsed, setContainerLastUsed] = useState<string>("Never")

  useEffect(() => {
    setUsername(getUsername())
  }, [])

  useEffect(() => {
    const fetchBlobStats = async () => {
      try {
        const res = await client.get<BlobContainer[]>("/list-blob-containers")
        const containers: BlobContainer[] = res.data || []
        setBucketCount(containers.length)
        if (containers.length > 0) {
          const latest = containers.reduce((prev, curr) => {
            const prevTime = new Date(prev.lastModified).getTime()
            const currTime = new Date(curr.lastModified).getTime()
            return currTime > prevTime ? curr : prev
          })
          setBlobLastUsed(formatRelativeTime(latest.lastModified))
        }
      } catch (err) {
        console.error("Failed to fetch blob storage stats:", err)
      }
    }
    fetchBlobStats()
  }, [])

  useEffect(() => {
    const fetchFunctionStats = async () => {
      try {
        const res = await client.get<FunctionItem[]>("/list-functions")
        const functions: FunctionItem[] = res.data || []
        setFunctionCount(functions.length)
        if (functions.length > 0) {
          const latest = functions.reduce((prev, curr) => {
            const prevTime = new Date(prev.lastModified).getTime()
            const currTime = new Date(curr.lastModified).getTime()
            return currTime > prevTime ? curr : prev
          })
          setFunctionLastUsed(formatRelativeTime(latest.lastModified))
        }
      } catch (err) {
        console.error("Failed to fetch function stats:", err)
      }
    }
    fetchFunctionStats()
  }, [])

  useEffect(() => {
    const fetchContainerStats = async () => {
      try {
        const res = await client.get<ContainerItem[]>("/get-containers")
        const allContainers: ContainerItem[] = res.data || []
        const running = allContainers.filter((c) => c.State.toLowerCase() === "running")
        setContainerCount(running.length)
        // Use the most recently created running container for "last used" (creation time is
        // the closest proxy for last activity available from the container list API);
        // fall back to all containers if none are running.
        const candidates = running.length > 0 ? running : allContainers
        if (candidates.length > 0) {
          const latest = candidates.reduce((prev, curr) =>
            curr.Created > prev.Created ? curr : prev
          )
          const isoString = new Date(latest.Created * 1000).toISOString()
          setContainerLastUsed(formatRelativeTime(isoString))
        }
      } catch (err) {
        console.error("Failed to fetch container stats:", err)
      }
    }
    fetchContainerStats()
  }, [])

  return (
    <>
      <DashboardShell>
        <DashboardHeader heading={username ? `Welcome back, ${username}` : "Welcome back"} text="Here's what's happening with your services today.">
          <Button>
            Deploy Service <ChevronRight className="ml-2 h-4 w-4" />
          </Button>
        </DashboardHeader>

        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
          <Card className="border-l-4 border-l-blue-500">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Container Runtime</CardTitle>
              <Container className="h-4 w-4 text-blue-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{containerCount}</div>
              <p className="text-xs text-muted-foreground">Active containers</p>
              <div className="mt-2">
                <Badge variant="outline" className="text-xs">
                  Last used: {containerLastUsed}
                </Badge>
              </div>
            </CardContent>
          </Card>

          <Card className="border-l-4 border-l-green-500">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Functions</CardTitle>
              <Zap className="h-4 w-4 text-green-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{functionCount}</div>
              <p className="text-xs text-muted-foreground">Functions deployed</p>
              <div className="mt-2">
                <Badge variant="outline" className="text-xs">
                  Last used: {functionLastUsed}
                </Badge>
              </div>
            </CardContent>
          </Card>

          <Card className="border-l-4 border-l-purple-500">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Blob Storage</CardTitle>
              <HardDrive className="h-4 w-4 text-purple-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{bucketCount}</div>
              <p className="text-xs text-muted-foreground">Active Buckets</p>
              <div className="mt-2">
                <Badge variant="outline" className="text-xs">
                  Last used: {blobLastUsed}
                </Badge>
              </div>
            </CardContent>
          </Card>

          <Card className="border-l-4 border-l-orange-500">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">API Gateway</CardTitle>
              <Globe className="h-4 w-4 text-orange-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">156K</div>
              <p className="text-xs text-muted-foreground">Requests today</p>
              <div className="mt-2">
                <Badge variant="outline" className="text-xs">
                  Last used: 30 sec ago
                </Badge>
              </div>
            </CardContent>
          </Card>
        </div>

        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          <div className="lg:col-span-2">
            <RecentServices />
          </div>
          <div>
            <ServiceQuickActions />
          </div>
        </div>

        <ServerMetrics />
      </DashboardShell>
    </>
  )
}
