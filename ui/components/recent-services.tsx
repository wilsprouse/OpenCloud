"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Container, HardDrive, Zap, GitBranch, ExternalLink } from "lucide-react"
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
  invocations: number
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

type RecentServiceItem = {
  name: string
  type: string
  icon: React.ElementType
  status: string
  statusColor: string
  lastUsed: string
  description: string
  metrics: string
  href: string
  // Unix milliseconds used for sorting; higher = more recent.
  lastActivityTime: number
}

// Number of recent service items to display on the dashboard.
const MAX_RECENT_SERVICES = 6

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

// Formats a byte count into a human-readable size string.
function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function RecentServices() {
  const [services, setServices] = useState<RecentServiceItem[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchAllServices = async () => {
      const items: RecentServiceItem[] = []

      // Fetch blob storage buckets.
      try {
        const res = await client.get<BlobContainer[]>("/list-blob-buckets")
        for (const c of res.data ?? []) {
          const t = new Date(c.lastModified).getTime()
          items.push({
            name: c.name,
            type: "Blob Storage",
            icon: HardDrive,
            status: "Active",
            statusColor: "bg-purple-100 text-purple-800",
            lastUsed: formatRelativeTime(c.lastModified),
            description: `${c.objectCount} object${c.objectCount !== 1 ? "s" : ""}`,
            metrics: `Size: ${formatBytes(c.totalSize)}`,
            href: `/storage/blob/${encodeURIComponent(c.name)}`,
            lastActivityTime: isNaN(t) ? 0 : t,
          })
        }
      } catch {
        // Blob storage unavailable; skip.
      }

      // Fetch serverless functions.
      try {
        const res = await client.get<FunctionItem[]>("/list-functions")
        for (const f of res.data ?? []) {
          const t = new Date(f.lastModified).getTime()
          items.push({
            name: f.name,
            type: "Serverless Function",
            icon: Zap,
            status: f.status === "active" ? "Active" : f.status.charAt(0).toUpperCase() + f.status.slice(1),
            statusColor: "bg-green-100 text-green-800",
            lastUsed: formatRelativeTime(f.lastModified),
            description: `Runtime: ${f.runtime}`,
            metrics: `Invocations: ${f.invocations ?? 0}`,
            href: `/compute/functions/${encodeURIComponent(f.id)}`,
            lastActivityTime: isNaN(t) ? 0 : t,
          })
        }
      } catch {
        // Functions service unavailable; skip.
      }

      // Fetch containers.
      try {
        const res = await client.get<ContainerItem[]>("/get-containers")
        for (const c of res.data ?? []) {
          const name = (c.Names?.[0] ?? c.Id).replace(/^\//, "")
          const createdMs = c.Created * 1000
          const isoString = new Date(createdMs).toISOString()
          const isRunning = c.State?.toLowerCase() === "running"
          items.push({
            name,
            type: "Container Runtime",
            icon: Container,
            status: isRunning ? "Running" : c.State,
            statusColor: isRunning
              ? "bg-green-100 text-green-800"
              : "bg-gray-100 text-gray-800",
            lastUsed: formatRelativeTime(isoString),
            description: c.Image,
            metrics: c.Status,
            href: `/compute/containers`,
            lastActivityTime: createdMs,
          })
        }
      } catch {
        // Container runtime unavailable; skip.
      }

      // Fetch pipelines.
      try {
        const res = await client.get<PipelineItem[]>("/get-pipelines")
        for (const p of res.data ?? []) {
          const activityTime = p.lastRun ?? p.createdAt
          const t = new Date(activityTime).getTime()
          const statusColors: Record<string, string> = {
            running: "bg-blue-100 text-blue-800",
            success: "bg-green-100 text-green-800",
            failed: "bg-red-100 text-red-800",
          }
          items.push({
            name: p.name,
            type: "Pipeline",
            icon: GitBranch,
            status: p.status.charAt(0).toUpperCase() + p.status.slice(1),
            statusColor: statusColors[p.status] ?? "bg-gray-100 text-gray-800",
            lastUsed: formatRelativeTime(activityTime),
            description: p.description || `Pipeline ${p.name}`,
            metrics: p.lastRun ? `Last run: ${formatRelativeTime(p.lastRun)}` : "Never run",
            href: `/ci-cd/pipelines/${p.id}`,
            lastActivityTime: isNaN(t) ? 0 : t,
          })
        }
      } catch {
        // CI/CD service unavailable; skip.
      }

      // Sort by most recent activity and take the top MAX_RECENT_SERVICES entries.
      items.sort((a, b) => b.lastActivityTime - a.lastActivityTime)
      setServices(items.slice(0, MAX_RECENT_SERVICES))
      setLoading(false)
    }

    fetchAllServices()
  }, [])

  return (
    <Card>
      <CardHeader>
        <CardTitle>Recent Services</CardTitle>
        <CardDescription>Services you've used recently, sorted by last activity</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {loading ? (
          <p className="text-sm text-muted-foreground">Loading recent services…</p>
        ) : services.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No services found. Deploy a service to get started.
          </p>
        ) : (
          services.map((service) => {
            const IconComponent = service.icon
            return (
              <div
                key={`${service.type}-${service.name}`}
                className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
              >
                <div className="flex items-center space-x-4">
                  <div className="bg-muted p-2 rounded-lg">
                    <IconComponent className="h-5 w-5" />
                  </div>
                  <div className="space-y-1">
                    <div className="flex items-center space-x-2">
                      <h4 className="font-medium">{service.name}</h4>
                      <Badge variant="outline" className={service.statusColor}>
                        {service.status}
                      </Badge>
                    </div>
                    <p className="text-sm text-muted-foreground">{service.description}</p>
                    <div className="flex items-center space-x-4 text-xs text-muted-foreground">
                      <span>{service.type}</span>
                      <span>•</span>
                      <span>{service.lastUsed}</span>
                      <span>•</span>
                      <span>{service.metrics}</span>
                    </div>
                  </div>
                </div>
                <div className="flex items-center space-x-2">
                  <Button variant="ghost" size="icon" asChild>
                    <Link href={service.href}>
                      <ExternalLink className="h-4 w-4" />
                    </Link>
                  </Button>
                </div>
              </div>
            )
          })
        )}
      </CardContent>
    </Card>
  )
}
