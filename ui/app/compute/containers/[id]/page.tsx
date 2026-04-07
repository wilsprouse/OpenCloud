'use client'

import { use, useCallback, useEffect, useRef, useState } from "react"
import { useRouter } from "next/navigation"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { DashboardHeader } from "@/components/dashboard-header"
import { DashboardShell } from "@/components/dashboard-shell"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
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
  Container,
  Play,
  Square,
  Trash2,
  Activity,
  Clock,
  Tag,
  Network,
  HardDrive,
  Layers,
  RotateCcw,
  FileText,
} from "lucide-react"

// ContainerDetail mirrors the ContainerDetail struct returned by GET /get-container.
type ContainerDetail = {
  id: string
  name: string
  image: string
  state: string
  status: string
  created: number
  env: string[]
  ports: string[]
  binds: string[]
  restartPolicy: string
  autoRemove: boolean
  memoryUsageBytes: number
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

export default function ContainerDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const resolvedParams = use(params)
  const containerId = decodeURIComponent(resolvedParams.id)
  const router = useRouter()

  const [container, setContainer] = useState<ContainerDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [activeTab, setActiveTab] = useState("overview")

  // Logs state
  const [logs, setLogs] = useState<string>("")
  const [loadingLogs, setLoadingLogs] = useState(false)
  const logsEndRef = useRef<HTMLDivElement>(null)

  // Action state
  const [actionLoading, setActionLoading] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [isStopDialogOpen, setIsStopDialogOpen] = useState(false)
  const [stopError, setStopError] = useState("")

  const fetchContainer = useCallback(async () => {
    setLoading(true)
    try {
      const res = await client.get<ContainerDetail>(`/get-container?id=${encodeURIComponent(containerId)}`)
      setContainer(res.data)
    } catch (err) {
      console.error("Failed to fetch container details:", err)
      toast.error("Failed to load container details")
    } finally {
      setLoading(false)
    }
  }, [containerId])

  const fetchLogs = useCallback(async () => {
    setLoadingLogs(true)
    try {
      const res = await client.get<string>(`/container-logs?id=${encodeURIComponent(containerId)}&tail=200`)
      setLogs(typeof res.data === "string" ? res.data : "")
    } catch (err) {
      console.error("Failed to fetch container logs:", err)
      setLogs("")
    } finally {
      setLoadingLogs(false)
    }
  }, [containerId])

  useEffect(() => {
    fetchContainer()
  }, [fetchContainer])

  // Auto-scroll logs to bottom when logs tab is opened or new logs arrive.
  useEffect(() => {
    if (activeTab === "logs" && logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: "smooth" })
    }
  }, [logs, activeTab])

  // Fetch logs when the logs tab is first selected.
  const handleTabChange = (tab: string) => {
    setActiveTab(tab)
    if (tab === "logs" && logs === "") {
      fetchLogs()
    }
  }

  const handleAction = async (action: "start" | "stop") => {
    setActionLoading(true)
    try {
      await client.post(`/containers/${encodeURIComponent(containerId)}/${action}`)
      await fetchContainer()
      toast.success(`Container ${action === "start" ? "started" : "stopped"} successfully`)
      return true
    } catch (err) {
      console.error(`Failed to ${action} container:`, err)
      toast.error(`Failed to ${action} container`)
      return false
    } finally {
      setActionLoading(false)
    }
  }

  const handleStopContainer = async () => {
    const stopped = await handleAction("stop")
    if (!stopped) {
      setStopError("Failed to stop container. Please try again.")
      return
    }
    setIsStopDialogOpen(false)
    setStopError("")
  }

  const handleDeleteContainer = async () => {
    setActionLoading(true)
    try {
      await client.post("/delete-container", { containerId })
      toast.success("Container deleted successfully")
      router.push("/compute/containers")
    } catch (err) {
      console.error("Failed to delete container:", err)
      toast.error("Failed to delete container")
    } finally {
      setActionLoading(false)
      setIsDeleteDialogOpen(false)
    }
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

  if (!container) {
    return (
      <DashboardShell>
        <DashboardHeader heading="Container Not Found" text="The requested container could not be loaded." />
        <Button variant="outline" onClick={() => router.push("/compute/containers")}>
          <ArrowLeft className="mr-2 h-4 w-4" />
          Back to Containers
        </Button>
      </DashboardShell>
    )
  }

  const isRunning = container.state === "running"

  return (
    <DashboardShell>
      {/* Page header */}
      <DashboardHeader
        heading={container.name || container.id.slice(0, 12)}
        text={container.image}
      >
        <div className="flex items-center space-x-2">
          <Button variant="outline" size="sm" onClick={() => router.push("/compute/containers")}>
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={fetchContainer}
            disabled={actionLoading}
          >
            <RefreshCw className={`h-4 w-4 ${actionLoading ? "animate-spin" : ""}`} />
          </Button>
        </div>
      </DashboardHeader>

      {/* Status bar */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center space-x-3">
          <div className={`p-2 rounded-lg ${isRunning ? "bg-green-50" : "bg-gray-50"}`}>
            <Container className={`h-5 w-5 ${isRunning ? "text-green-600" : "text-gray-600"}`} />
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <span className="font-medium">{container.name || "Unnamed"}</span>
              <Badge variant={isRunning ? "default" : "secondary"}>
                {container.state}
              </Badge>
            </div>
            <p className="text-sm text-muted-foreground">{container.id.slice(0, 12)}</p>
          </div>
        </div>

        {/* Action buttons */}
        <div className="flex items-center space-x-2">
          {!isRunning && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleAction("start")}
              disabled={actionLoading}
            >
              {actionLoading ? (
                <RefreshCw className="h-4 w-4 mr-1 animate-spin" />
              ) : (
                <Play className="h-4 w-4 mr-1" />
              )}
              Start
            </Button>
          )}
          {isRunning && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setStopError("")
                setIsStopDialogOpen(true)
              }}
              disabled={actionLoading}
            >
              <Square className="h-4 w-4 mr-1" />
              Stop
            </Button>
          )}
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

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={handleTabChange}>
        <TabsList className="mb-4">
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
        </TabsList>

        {/* ── Overview Tab ── */}
        <TabsContent value="overview">
          <div className="grid gap-4 md:grid-cols-2">
            {/* Runtime info */}
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium flex items-center">
                  <Activity className="h-4 w-4 mr-2 text-muted-foreground" />
                  Runtime
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3 text-sm">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">State</span>
                  <Badge variant={isRunning ? "default" : "secondary"}>{container.state}</Badge>
                </div>
                {isRunning && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Memory</span>
                    <span>{formatBytes(container.memoryUsageBytes)}</span>
                  </div>
                )}
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Auto Remove</span>
                  <span>{container.autoRemove ? "Yes" : "No"}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground flex items-center">
                    <RotateCcw className="h-3 w-3 mr-1" />
                    Restart Policy
                  </span>
                  <span>{container.restartPolicy || "no"}</span>
                </div>
              </CardContent>
            </Card>

            {/* Image & creation */}
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium flex items-center">
                  <Layers className="h-4 w-4 mr-2 text-muted-foreground" />
                  Image
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3 text-sm">
                <div className="flex justify-between">
                  <span className="text-muted-foreground flex items-center">
                    <Tag className="h-3 w-3 mr-1" />
                    Image
                  </span>
                  <span className="text-right truncate max-w-[60%]">{container.image}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Full ID</span>
                  <span className="font-mono text-xs truncate max-w-[60%]">{container.id.slice(0, 12)}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground flex items-center">
                    <Clock className="h-3 w-3 mr-1" />
                    Created
                  </span>
                  <span>{formatDate(container.created)}</span>
                </div>
              </CardContent>
            </Card>

            {/* Port mappings */}
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium flex items-center">
                  <Network className="h-4 w-4 mr-2 text-muted-foreground" />
                  Port Mappings
                </CardTitle>
                <CardDescription>
                  {container.ports?.length ? `${container.ports.length} port mapping(s)` : "No port mappings"}
                </CardDescription>
              </CardHeader>
              <CardContent>
                {container.ports?.length ? (
                  <ul className="space-y-1 text-sm">
                    {container.ports.map((p, i) => (
                      <li key={i} className="flex items-center font-mono text-xs bg-muted rounded px-2 py-1">
                        {p}
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="text-sm text-muted-foreground">No ports exposed</p>
                )}
              </CardContent>
            </Card>

            {/* Volume mounts */}
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium flex items-center">
                  <HardDrive className="h-4 w-4 mr-2 text-muted-foreground" />
                  Volume Mounts
                </CardTitle>
                <CardDescription>
                  {container.binds?.length ? `${container.binds.length} volume mount(s)` : "No volume mounts"}
                </CardDescription>
              </CardHeader>
              <CardContent>
                {container.binds?.length ? (
                  <ul className="space-y-1 text-sm">
                    {container.binds.map((b, i) => (
                      <li key={i} className="flex items-center font-mono text-xs bg-muted rounded px-2 py-1 break-all">
                        {b}
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="text-sm text-muted-foreground">No volume mounts</p>
                )}
              </CardContent>
            </Card>

            {/* Environment variables */}
            <Card className="md:col-span-2">
              <CardHeader>
                <CardTitle className="text-sm font-medium flex items-center">
                  <FileText className="h-4 w-4 mr-2 text-muted-foreground" />
                  Environment Variables
                </CardTitle>
                <CardDescription>
                  {container.env?.length ? `${container.env.length} variable(s)` : "No environment variables"}
                </CardDescription>
              </CardHeader>
              <CardContent>
                {container.env?.length ? (
                  <ul className="grid gap-1 sm:grid-cols-2">
                    {container.env.map((e, i) => (
                      <li
                        key={i}
                        className="font-mono text-xs bg-muted rounded px-2 py-1 break-all"
                      >
                        {e}
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="text-sm text-muted-foreground">No environment variables set</p>
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        {/* ── Logs Tab ── */}
        <TabsContent value="logs">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <div>
                <CardTitle className="text-sm font-medium">Container Logs</CardTitle>
                <CardDescription>Last 200 log lines from stdout and stderr</CardDescription>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={fetchLogs}
                disabled={loadingLogs}
              >
                {loadingLogs ? (
                  <RefreshCw className="h-4 w-4 animate-spin" />
                ) : (
                  <RefreshCw className="h-4 w-4" />
                )}
                <span className="ml-2">Refresh</span>
              </Button>
            </CardHeader>
            <CardContent>
              <div
                className="bg-black rounded-lg p-4 h-96 overflow-y-auto font-mono text-xs text-green-400"
                aria-label="Container log output"
              >
                {loadingLogs ? (
                  <div className="flex items-center justify-center h-full">
                    <RefreshCw className="h-6 w-6 animate-spin text-green-400" />
                  </div>
                ) : logs ? (
                  <>
                    {logs.split("\n").map((line, i) => (
                      <div key={i} className="whitespace-pre-wrap break-all">
                        {line || "\u00A0"}
                      </div>
                    ))}
                    <div ref={logsEndRef} />
                  </>
                ) : (
                  <div className="flex flex-col items-center justify-center h-full text-muted-foreground">
                    <FileText className="h-8 w-8 mb-2 opacity-50" />
                    <p>No logs available</p>
                    {!isRunning && (
                      <p className="text-xs mt-1 opacity-75">Container is not running</p>
                    )}
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Stop Confirmation Dialog */}
      <Dialog open={isStopDialogOpen} onOpenChange={setIsStopDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Stop Container</DialogTitle>
            <DialogDescription>
              Are you sure you want to stop{" "}
              <span className="font-semibold">{container.name || container.id.slice(0, 12)}</span>?
              The container will remain available to restart.
            </DialogDescription>
          </DialogHeader>
          {stopError && (
            <p className="text-sm text-destructive">{stopError}</p>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsStopDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              variant="default"
              onClick={handleStopContainer}
              disabled={actionLoading}
            >
              {actionLoading ? <RefreshCw className="h-4 w-4 animate-spin mr-2" /> : null}
              Stop
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Container</DialogTitle>
            <DialogDescription>
              Are you sure you want to permanently delete{" "}
              <span className="font-semibold">{container.name || container.id.slice(0, 12)}</span>?
              This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsDeleteDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleDeleteContainer}
              disabled={actionLoading}
            >
              {actionLoading ? <RefreshCw className="h-4 w-4 animate-spin mr-2" /> : null}
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </DashboardShell>
  )
}
