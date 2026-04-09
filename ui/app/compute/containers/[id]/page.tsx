'use client'

import { use, useCallback, useEffect, useRef, useState } from "react"
import { useRouter } from "next/navigation"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
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
  Plus,
  X,
  Pencil,
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
  command: string
}

// A single port mapping entry used in the edit form
type PortMapping = {
  hostPort: string
  containerPort: string
}

// A single environment variable entry used in the edit form
type EnvVar = {
  key: string
  value: string
}

// A single volume mount entry used in the edit form
type VolumeMount = {
  hostPath: string
  containerPath: string
  Z: boolean
  U: boolean
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

// Parses a port string like "8080:80/tcp" or "0.0.0.0:8080:80/tcp" into a PortMapping.
function parsePortString(port: string): PortMapping | null {
  // Strip optional protocol suffix (e.g. "/tcp")
  const withoutProto = port.split("/")[0]
  const parts = withoutProto.split(":")
  if (parts.length === 2) {
    return { hostPort: parts[0], containerPort: parts[1] }
  }
  if (parts.length === 3) {
    // "hostIP:hostPort:containerPort"
    return { hostPort: parts[1], containerPort: parts[2] }
  }
  return null
}

// Parses an env string like "KEY=VALUE" into an EnvVar.
function parseEnvString(env: string): EnvVar {
  const idx = env.indexOf("=")
  if (idx === -1) return { key: env, value: "" }
  return { key: env.slice(0, idx), value: env.slice(idx + 1) }
}

// Parses a bind string like "hostPath:containerPath[:options]" into a VolumeMount.
function parseBindString(bind: string): VolumeMount {
  const parts = bind.split(":")
  const hostPath = parts[0] ?? ""
  const containerPath = parts[1] ?? ""
  const opts = parts[2] ?? ""
  const optList = opts.split(",")
  return {
    hostPath,
    containerPath,
    Z: optList.includes("Z") || optList.includes("z"),
    U: optList.includes("U"),
  }
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

  // Edit form state
  const [editImage, setEditImage] = useState("")
  const [editName, setEditName] = useState("")
  const [editPorts, setEditPorts] = useState<PortMapping[]>([{ hostPort: "", containerPort: "" }])
  const [editEnvVars, setEditEnvVars] = useState<EnvVar[]>([{ key: "", value: "" }])
  const [editVolumes, setEditVolumes] = useState<VolumeMount[]>([{ hostPath: "", containerPath: "", Z: false, U: false }])
  const [editRestartPolicy, setEditRestartPolicy] = useState("no")
  const [editAutoRemove, setEditAutoRemove] = useState(false)
  const [editCommand, setEditCommand] = useState("")
  const [isUpdating, setIsUpdating] = useState(false)

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

  // Pre-populate the edit form whenever the container data is loaded or the edit tab is selected.
  useEffect(() => {
    if (container && activeTab === "edit") {
      setEditImage(container.image)
      setEditName(container.name)
      setEditRestartPolicy(container.restartPolicy || "no")
      setEditAutoRemove(container.autoRemove)
      setEditCommand(container.command || "")

      const parsedPorts = (container.ports ?? [])
        .map(parsePortString)
        .filter((p): p is PortMapping => p !== null)
      setEditPorts(parsedPorts.length > 0 ? parsedPorts : [{ hostPort: "", containerPort: "" }])

      const parsedEnv = (container.env ?? []).map(parseEnvString)
      setEditEnvVars(parsedEnv.length > 0 ? parsedEnv : [{ key: "", value: "" }])

      const parsedVolumes = (container.binds ?? []).map(parseBindString)
      setEditVolumes(parsedVolumes.length > 0 ? parsedVolumes : [{ hostPath: "", containerPath: "", Z: false, U: false }])
    }
  }, [container, activeTab])

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

  // Submits the edit form by stopping the old container, removing it, and
  // recreating it with the updated configuration via POST /update-container.
  const handleUpdateContainer = async () => {
    if (!editImage.trim()) {
      toast.error("Image is required")
      return
    }

    setIsUpdating(true)
    try {
      const ports = editPorts
        .filter(p => p.hostPort && p.containerPort)
        .map(p => `${p.hostPort}:${p.containerPort}`)

      const env = editEnvVars
        .filter(e => e.key)
        // Env vars without a value are serialized as "KEY" (bare key); KEY= would be an explicit empty string.
        .map(e => (e.value !== "" ? `${e.key}=${e.value}` : e.key))

      const volumes = editVolumes
        .filter(v => v.hostPath && v.containerPath)
        .map(v => {
          const opts: string[] = []
          if (v.Z) opts.push("Z")
          if (v.U) opts.push("U")
          return opts.length > 0
            ? `${v.hostPath}:${v.containerPath}:${opts.join(",")}`
            : `${v.hostPath}:${v.containerPath}`
        })

      const res = await client.post<{ status: string; containerId: string }>("/update-container", {
        containerId,
        image: editImage.trim(),
        name: editName.trim() || undefined,
        ports,
        env,
        volumes,
        restartPolicy: editRestartPolicy,
        autoRemove: editAutoRemove,
        command: editCommand.trim() || undefined,
      })

      toast.success("Container updated and restarted successfully")
      // The container has been recreated with a new ID; navigate to the new container.
      const newId = res.data?.containerId
      if (newId && newId !== containerId) {
        router.push(`/compute/containers/${encodeURIComponent(newId)}`)
      } else {
        await fetchContainer()
        setActiveTab("overview")
      }
    } catch (err) {
      console.error("Failed to update container:", err)
      const msg = err instanceof Error ? err.message : "Failed to update container"
      toast.error(msg)
    } finally {
      setIsUpdating(false)
    }
  }

  // Edit form helpers for port mappings
  const addEditPort = () => setEditPorts(prev => [...prev, { hostPort: "", containerPort: "" }])
  const removeEditPort = (i: number) => setEditPorts(prev => prev.filter((_, idx) => idx !== i))
  const updateEditPort = (i: number, field: keyof PortMapping, value: string) =>
    setEditPorts(prev => prev.map((p, idx) => (idx === i ? { ...p, [field]: value } : p)))

  // Edit form helpers for environment variables
  const addEditEnvVar = () => setEditEnvVars(prev => [...prev, { key: "", value: "" }])
  const removeEditEnvVar = (i: number) => setEditEnvVars(prev => prev.filter((_, idx) => idx !== i))
  const updateEditEnvVar = (i: number, field: keyof EnvVar, value: string) =>
    setEditEnvVars(prev => prev.map((e, idx) => (idx === i ? { ...e, [field]: value } : e)))

  // Edit form helpers for volume mounts
  const addEditVolume = () => setEditVolumes(prev => [...prev, { hostPath: "", containerPath: "", Z: false, U: false }])
  const removeEditVolume = (i: number) => setEditVolumes(prev => prev.filter((_, idx) => idx !== i))
  const updateEditVolume = (i: number, field: keyof VolumeMount, value: string | boolean) =>
    setEditVolumes(prev => prev.map((v, idx) => (idx === i ? { ...v, [field]: value } : v)))

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
          <TabsTrigger value="edit">
            <Pencil className="h-3 w-3 mr-1" />
            Edit
          </TabsTrigger>
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

        {/* ── Edit Tab ── */}
        <TabsContent value="edit">
          <div className="space-y-6 max-w-2xl">
            <p className="text-sm text-muted-foreground">
              Update the container configuration below. Saving will stop the current container,
              remove it, and recreate it with the new settings.
            </p>

            {/* Image & Name */}
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium">Image &amp; Name</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="edit-image">Image</Label>
                  <Input
                    id="edit-image"
                    placeholder="e.g. nginx:latest"
                    value={editImage}
                    onChange={e => setEditImage(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="edit-name">Container Name <span className="text-muted-foreground">(optional)</span></Label>
                  <Input
                    id="edit-name"
                    placeholder="my-container"
                    value={editName}
                    onChange={e => setEditName(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="edit-command">Command Override <span className="text-muted-foreground">(optional)</span></Label>
                  <Input
                    id="edit-command"
                    placeholder="e.g. /bin/sh -c 'echo hello'"
                    value={editCommand}
                    onChange={e => setEditCommand(e.target.value)}
                  />
                </div>
              </CardContent>
            </Card>

            {/* Port Mappings */}
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium flex items-center justify-between">
                  <span className="flex items-center">
                    <Network className="h-4 w-4 mr-2 text-muted-foreground" />
                    Port Mappings
                  </span>
                  <Button variant="outline" size="sm" onClick={addEditPort} type="button">
                    <Plus className="h-3 w-3 mr-1" />
                    Add Port
                  </Button>
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                {editPorts.map((port, i) => (
                  <div key={i} className="flex items-center gap-2">
                    <Input
                      placeholder="Host port"
                      value={port.hostPort}
                      onChange={e => updateEditPort(i, "hostPort", e.target.value)}
                      className="w-32"
                    />
                    <span className="text-muted-foreground">:</span>
                    <Input
                      placeholder="Container port"
                      value={port.containerPort}
                      onChange={e => updateEditPort(i, "containerPort", e.target.value)}
                      className="w-36"
                    />
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => removeEditPort(i)}
                      type="button"
                      disabled={editPorts.length === 1}
                    >
                      <X className="h-3 w-3" />
                    </Button>
                  </div>
                ))}
              </CardContent>
            </Card>

            {/* Environment Variables */}
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium flex items-center justify-between">
                  <span className="flex items-center">
                    <FileText className="h-4 w-4 mr-2 text-muted-foreground" />
                    Environment Variables
                  </span>
                  <Button variant="outline" size="sm" onClick={addEditEnvVar} type="button">
                    <Plus className="h-3 w-3 mr-1" />
                    Add Variable
                  </Button>
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                {editEnvVars.map((env, i) => (
                  <div key={i} className="flex items-center gap-2">
                    <Input
                      placeholder="KEY"
                      value={env.key}
                      onChange={e => updateEditEnvVar(i, "key", e.target.value)}
                      className="w-40 font-mono text-sm"
                    />
                    <span className="text-muted-foreground">=</span>
                    <Input
                      placeholder="value"
                      value={env.value}
                      onChange={e => updateEditEnvVar(i, "value", e.target.value)}
                      className="flex-1 font-mono text-sm"
                    />
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => removeEditEnvVar(i)}
                      type="button"
                      disabled={editEnvVars.length === 1}
                    >
                      <X className="h-3 w-3" />
                    </Button>
                  </div>
                ))}
              </CardContent>
            </Card>

            {/* Volume Mounts */}
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium flex items-center justify-between">
                  <span className="flex items-center">
                    <HardDrive className="h-4 w-4 mr-2 text-muted-foreground" />
                    Volume Mounts
                  </span>
                  <Button variant="outline" size="sm" onClick={addEditVolume} type="button">
                    <Plus className="h-3 w-3 mr-1" />
                    Add Volume
                  </Button>
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                {editVolumes.map((vol, i) => (
                  <div key={i} className="flex items-center gap-2 flex-wrap">
                    <Input
                      placeholder="Host path"
                      value={vol.hostPath}
                      onChange={e => updateEditVolume(i, "hostPath", e.target.value)}
                      className="flex-1 min-w-[120px] font-mono text-sm"
                    />
                    <span className="text-muted-foreground">:</span>
                    <Input
                      placeholder="Container path"
                      value={vol.containerPath}
                      onChange={e => updateEditVolume(i, "containerPath", e.target.value)}
                      className="flex-1 min-w-[120px] font-mono text-sm"
                    />
                    <div className="flex items-center gap-1">
                      <Switch
                        id={`edit-vol-z-${i}`}
                        checked={vol.Z}
                        onCheckedChange={v => updateEditVolume(i, "Z", v)}
                      />
                      <Label htmlFor={`edit-vol-z-${i}`} className="text-xs">Z</Label>
                    </div>
                    <div className="flex items-center gap-1">
                      <Switch
                        id={`edit-vol-u-${i}`}
                        checked={vol.U}
                        onCheckedChange={v => updateEditVolume(i, "U", v)}
                      />
                      <Label htmlFor={`edit-vol-u-${i}`} className="text-xs">U</Label>
                    </div>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => removeEditVolume(i)}
                      type="button"
                      disabled={editVolumes.length === 1}
                    >
                      <X className="h-3 w-3" />
                    </Button>
                  </div>
                ))}
              </CardContent>
            </Card>

            {/* Options */}
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-medium">Options</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <Label>Restart Policy</Label>
                  <Select value={editRestartPolicy} onValueChange={setEditRestartPolicy}>
                    <SelectTrigger className="w-48">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="no">No</SelectItem>
                      <SelectItem value="always">Always</SelectItem>
                      <SelectItem value="on-failure">On Failure</SelectItem>
                      <SelectItem value="unless-stopped">Unless Stopped</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex items-center gap-3">
                  <Switch
                    id="edit-auto-remove"
                    checked={editAutoRemove}
                    onCheckedChange={setEditAutoRemove}
                    disabled={editRestartPolicy !== "no"}
                  />
                  <Label htmlFor="edit-auto-remove">
                    Auto Remove
                    <span className="text-xs text-muted-foreground ml-2">
                      (remove container on exit)
                    </span>
                  </Label>
                </div>
                {editRestartPolicy !== "no" && (
                  <p className="text-xs text-muted-foreground">
                    Auto Remove is unavailable when a restart policy other than &quot;no&quot; is selected.
                  </p>
                )}
              </CardContent>
            </Card>

            {/* Save Button */}
            <div className="flex justify-end">
              <Button
                onClick={handleUpdateContainer}
                disabled={isUpdating || !editImage.trim()}
              >
                {isUpdating ? (
                  <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                ) : (
                  <Pencil className="h-4 w-4 mr-2" />
                )}
                {isUpdating ? "Updating..." : "Save & Restart"}
              </Button>
            </div>
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
