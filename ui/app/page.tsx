"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { Archive, ChevronDown, Container, GitBranch, HardDrive, Package, Zap } from "lucide-react"
import * as DropdownMenu from "@radix-ui/react-dropdown-menu"

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

type PipelineItem = {
  id: string
  name: string
  description: string
  status: string
  createdAt: string
  lastRun?: string
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

const deployServices = [
  {
    name: "Functions",
    description: "Run serverless functions at scale",
    icon: Zap,
    color: "text-green-600",
    href: "/compute/functions?create=true",
  },
  {
    name: "Container Runtime",
    description: "Deploy containerized applications",
    icon: Container,
    color: "text-cyan-600",
    href: "/compute/containers?create=true",
  },
  {
    name: "Blob Storage",
    description: "Store and retrieve unstructured data",
    icon: Archive,
    color: "text-purple-600",
    href: "/storage/blob?create=true",
  },
  {
    name: "Container Registry",
    description: "Manage and push container images",
    icon: Package,
    color: "text-blue-600",
    href: "/storage/containers?create=true",
  },
  {
    name: "CI/CD Pipelines",
    description: "Automate builds, tests, and deployments",
    icon: GitBranch,
    color: "text-orange-600",
    href: "/ci-cd/pipelines?create=true",
  },
]

export default function DashboardPage() {
  const router = useRouter()
  const [username, setUsername] = useState<string | null>(null)
  const [bucketCount, setBucketCount] = useState<number>(0)
  const [blobLastUsed, setBlobLastUsed] = useState<string>("Never")
  const [functionCount, setFunctionCount] = useState<number>(0)
  const [functionLastUsed, setFunctionLastUsed] = useState<string>("Never")
  const [containerCount, setContainerCount] = useState<number>(0)
  const [containerLastUsed, setContainerLastUsed] = useState<string>("Never")
  const [pipelineCount, setPipelineCount] = useState<number>(0)
  const [pipelineLastUsed, setPipelineLastUsed] = useState<string>("Never")

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

  useEffect(() => {
    const fetchPipelineStats = async () => {
      try {
        const res = await client.get<PipelineItem[]>("/get-pipelines")
        const pipelines: PipelineItem[] = res.data || []
        setPipelineCount(pipelines.length)
        if (pipelines.length > 0) {
          // Prefer lastRun time; fall back to createdAt for pipelines never run.
          const latest = pipelines.reduce((prev, curr) => {
            const prevTime = new Date(prev.lastRun ?? prev.createdAt).getTime()
            const currTime = new Date(curr.lastRun ?? curr.createdAt).getTime()
            return currTime > prevTime ? curr : prev
          })
          const activityTime = latest.lastRun ?? latest.createdAt
          setPipelineLastUsed(formatRelativeTime(activityTime))
        }
      } catch (err) {
        console.error("Failed to fetch pipeline stats:", err)
      }
    }
    fetchPipelineStats()
  }, [])

  return (
    <>
      <DashboardShell>
        <DashboardHeader heading={username ? `Welcome back, ${username}` : "Welcome back"} text="Here's what's happening with your services today.">
          <DropdownMenu.Root>
            <DropdownMenu.Trigger asChild>
              <Button>
                Deploy Service <ChevronDown className="ml-2 h-4 w-4" />
              </Button>
            </DropdownMenu.Trigger>
            <DropdownMenu.Portal>
              <DropdownMenu.Content
                className="min-w-[240px] rounded-md bg-white p-1 shadow-lg ring-1 ring-black/5 dark:bg-neutral-900"
                sideOffset={8}
                align="end"
              >
                {deployServices.map((service) => {
                  const IconComponent = service.icon
                  return (
                    <DropdownMenu.Item
                      key={service.name}
                      onSelect={() => router.push(service.href)}
                      className="flex cursor-pointer select-none items-center gap-3 rounded px-3 py-2 text-sm text-foreground hover:bg-neutral-100 dark:hover:bg-neutral-800 focus:outline-none"
                    >
                      <IconComponent className={`h-4 w-4 shrink-0 ${service.color}`} />
                      <div>
                        <div className="font-medium">{service.name}</div>
                        <div className="text-xs text-muted-foreground">{service.description}</div>
                      </div>
                    </DropdownMenu.Item>
                  )
                })}
              </DropdownMenu.Content>
            </DropdownMenu.Portal>
          </DropdownMenu.Root>
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
              <CardTitle className="text-sm font-medium">Pipelines</CardTitle>
              <GitBranch className="h-4 w-4 text-orange-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{pipelineCount}</div>
              <p className="text-xs text-muted-foreground">Pipelines deployed</p>
              <div className="mt-2">
                <Badge variant="outline" className="text-xs">
                  Last used: {pipelineLastUsed}
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
