'use client'

import { use, useCallback, useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { DashboardHeader } from "@/components/dashboard-header"
import { DashboardShell } from "@/components/dashboard-shell"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { toast } from "sonner"
import client from "@/app/utility/post"
import {
  ArrowLeft,
  RefreshCw,
  Layers,
  Trash2,
  Play,
  Tag,
  HardDrive,
  Clock,
  Cpu,
  BookOpen,
  Hash,
} from "lucide-react"

// ImageDetail mirrors the ImageDetail struct returned by GET /get-image.
type ImageDetail = {
  id: string
  repoTags: string[]
  repoDigests: string[]
  created: number
  size: number
  virtualSize: number
  labels: Record<string, string>
  architecture: string
  os: string
  author: string
  comment: string
  namesHistory: string[]
}

// Formats a byte count into a human-readable string (e.g. 1048576 → "1.0 MB").
function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return "—"
  const units = ["B", "KB", "MB", "GB", "TB"]
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

// Formats a Unix timestamp as a locale date/time string.
function formatDate(ts: number): string {
  if (!ts) return "—"
  return new Date(ts * 1000).toLocaleString()
}

export default function ImageDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const resolvedParams = use(params)
  const imageName = decodeURIComponent(resolvedParams.id)
  const router = useRouter()

  const [image, setImage] = useState<ImageDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [actionLoading, setActionLoading] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)

  const fetchImage = useCallback(async () => {
    setLoading(true)
    try {
      const res = await client.get<ImageDetail>(`/get-image?name=${encodeURIComponent(imageName)}`)
      setImage(res.data)
    } catch (err) {
      console.error("Failed to fetch image details:", err)
      toast.error("Failed to load image details")
    } finally {
      setLoading(false)
    }
  }, [imageName])

  useEffect(() => {
    fetchImage()
  }, [fetchImage])

  const handleDeleteImage = async () => {
    setActionLoading(true)
    try {
      await client.post("/delete-image", { imageName })
      toast.success("Image deleted successfully")
      router.push("/storage/containers")
    } catch (err: unknown) {
      const axiosErr = err as { response?: { status?: number; data?: unknown } }
      if (axiosErr?.response?.status === 409) {
        const rawData = axiosErr.response?.data
        const message =
          typeof rawData === "string" && rawData.trim()
            ? rawData.trim()
            : "This image is in use by a container. Remove the container before deleting the image."
        toast.error(message)
      } else {
        toast.error("Failed to delete image. Please try again.")
        console.error("Failed to delete image:", err)
      }
    } finally {
      setActionLoading(false)
      setIsDeleteDialogOpen(false)
    }
  }

  const handleDeployToCompute = () => {
    const tag = image?.repoTags?.[0] ?? imageName
    router.push(`/compute/containers?create=true&image=${encodeURIComponent(tag)}`)
  }

  if (loading) {
    return (
      <DashboardShell>
        <div className="flex items-center justify-center h-64">
          <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      </DashboardShell>
    )
  }

  if (!image) {
    return (
      <DashboardShell>
        <DashboardHeader heading="Image Not Found" text="The requested container image could not be loaded." />
        <Button variant="outline" onClick={() => router.push("/storage/containers")}>
          <ArrowLeft className="mr-2 h-4 w-4" />
          Back to Container Registry
        </Button>
      </DashboardShell>
    )
  }

  const displayName = image.repoTags?.[0] ?? imageName

  return (
    <DashboardShell>
      {/* Page header */}
      <DashboardHeader
        heading={displayName}
        text={image.id.startsWith("sha256:") ? image.id.slice(7, 19) : image.id.slice(0, 12)}
      >
        <div className="flex items-center space-x-2">
          <Button variant="outline" size="sm" onClick={() => router.push("/storage/containers")}>
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={fetchImage}
            disabled={actionLoading}
          >
            <RefreshCw className={`h-4 w-4 ${actionLoading ? "animate-spin" : ""}`} />
          </Button>
        </div>
      </DashboardHeader>

      {/* Status / action bar */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center space-x-3">
          <div className="p-2 rounded-lg bg-blue-50 dark:bg-blue-950/40">
            <Layers className="h-5 w-5 text-blue-600 dark:text-blue-400" />
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <span className="font-medium">{displayName}</span>
              <Badge variant="secondary">available</Badge>
            </div>
            <p className="text-sm text-muted-foreground font-mono">
              {image.id.startsWith("sha256:") ? image.id.slice(7, 19) : image.id.slice(0, 12)}
            </p>
          </div>
        </div>

        {/* Action buttons */}
        <div className="flex items-center space-x-2">
          <Button
            variant="outline"
            size="sm"
            onClick={handleDeployToCompute}
            disabled={actionLoading}
          >
            <Play className="h-4 w-4 mr-1" />
            Deploy to Compute
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={() => setIsDeleteDialogOpen(true)}
            disabled={actionLoading}
          >
            <Trash2 className="h-4 w-4 mr-1" />
            Delete
          </Button>
        </div>
      </div>

      {/* Detail cards */}
      <div className="grid gap-4 md:grid-cols-2">
        {/* Image metadata */}
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium flex items-center">
              <Layers className="h-4 w-4 mr-2 text-muted-foreground" />
              Image Info
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <div className="flex justify-between">
              <span className="text-muted-foreground flex items-center">
                <HardDrive className="h-3 w-3 mr-1" />
                Size
              </span>
              <span>{formatBytes(image.size)}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground flex items-center">
                <HardDrive className="h-3 w-3 mr-1" />
                Virtual Size
              </span>
              <span>{formatBytes(image.virtualSize)}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground flex items-center">
                <Clock className="h-3 w-3 mr-1" />
                Created
              </span>
              <span>{formatDate(image.created)}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground flex items-center">
                <Cpu className="h-3 w-3 mr-1" />
                Architecture
              </span>
              <span>{image.architecture || "—"}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">OS</span>
              <span>{image.os || "—"}</span>
            </div>
            {image.author && (
              <div className="flex justify-between">
                <span className="text-muted-foreground flex items-center">
                  <BookOpen className="h-3 w-3 mr-1" />
                  Author
                </span>
                <span className="text-right truncate max-w-[60%]">{image.author}</span>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Tags */}
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium flex items-center">
              <Tag className="h-4 w-4 mr-2 text-muted-foreground" />
              Tags
            </CardTitle>
            <CardDescription>
              {image.repoTags?.length
                ? `${image.repoTags.length} tag(s)`
                : "No tags"}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {image.repoTags?.length ? (
              <ul className="space-y-1">
                {image.repoTags.map((tag, i) => (
                  <li
                    key={i}
                    className="font-mono text-xs bg-muted rounded px-2 py-1 break-all"
                  >
                    {tag}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-muted-foreground">No tags associated with this image</p>
            )}
          </CardContent>
        </Card>

        {/* Digests */}
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium flex items-center">
              <Hash className="h-4 w-4 mr-2 text-muted-foreground" />
              Content Digests
            </CardTitle>
            <CardDescription>
              {image.repoDigests?.length
                ? `${image.repoDigests.length} digest(s)`
                : "No digests"}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {image.repoDigests?.length ? (
              <ul className="space-y-1">
                {image.repoDigests.map((d, i) => (
                  <li
                    key={i}
                    className="font-mono text-xs bg-muted rounded px-2 py-1 break-all"
                  >
                    {d}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-muted-foreground">No digests available</p>
            )}
          </CardContent>
        </Card>

        {/* Full image ID */}
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium flex items-center">
              <Hash className="h-4 w-4 mr-2 text-muted-foreground" />
              Full Image ID
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="font-mono text-xs bg-muted rounded px-2 py-2 break-all">{image.id}</p>
          </CardContent>
        </Card>

        {/* Labels */}
        {image.labels && Object.keys(image.labels).length > 0 && (
          <Card className="md:col-span-2">
            <CardHeader>
              <CardTitle className="text-sm font-medium flex items-center">
                <BookOpen className="h-4 w-4 mr-2 text-muted-foreground" />
                Labels
              </CardTitle>
              <CardDescription>{Object.keys(image.labels).length} label(s)</CardDescription>
            </CardHeader>
            <CardContent>
              <ul className="grid gap-1 sm:grid-cols-2">
                {Object.entries(image.labels).map(([k, v]) => (
                  <li
                    key={k}
                    className="font-mono text-xs bg-muted rounded px-2 py-1 break-all"
                  >
                    <span className="font-semibold">{k}</span>
                    {v ? `=${v}` : ""}
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
        )}

        {/* Names history */}
        {image.namesHistory && image.namesHistory.length > 0 && (
          <Card className="md:col-span-2">
            <CardHeader>
              <CardTitle className="text-sm font-medium flex items-center">
                <BookOpen className="h-4 w-4 mr-2 text-muted-foreground" />
                Names History
              </CardTitle>
              <CardDescription>{image.namesHistory.length} historical name(s)</CardDescription>
            </CardHeader>
            <CardContent>
              <ul className="space-y-1">
                {image.namesHistory.map((name, i) => (
                  <li
                    key={i}
                    className="font-mono text-xs bg-muted rounded px-2 py-1 break-all"
                  >
                    {name}
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
        )}
      </div>

      {/* Delete Image Confirmation Dialog */}
      <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Container Image</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete{" "}
              <span className="font-semibold">{displayName}</span>? This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setIsDeleteDialogOpen(false)}
              disabled={actionLoading}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleDeleteImage}
              disabled={actionLoading}
            >
              {actionLoading ? (
                <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <Trash2 className="mr-2 h-4 w-4" />
              )}
              Delete Image
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </DashboardShell>
  )
}
