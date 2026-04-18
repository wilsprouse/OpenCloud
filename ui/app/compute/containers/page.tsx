'use client'

import { Suspense, useEffect, useRef, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { DashboardHeader } from "@/components/dashboard-header"
import { DashboardShell } from "@/components/dashboard-shell"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { toast } from "sonner"
import client from "@/app/utility/post"
import { FUNCTION_NAME_MAX_LENGTH, isValidFunctionName } from "@/lib/function-name"
import { useFunctionNameWarning } from "@/lib/use-function-name-warning"
import { stripRegistryPrefix } from "@/lib/image-name"
import { 
  RefreshCw, 
  Search,
  Container,
  Play,
  Square,
  Trash2,
  Activity,
  Package,
  Image as ImageIcon,
  Download,
  Plus,
  X,
  Power,
} from "lucide-react"

type ContainerItem = {
  Id: string
  Names: string[]
  Image: string
  State: string
  Status: string
  Pid: number
  MemoryUsageBytes: number
}

/** Base path prefix for blob storage buckets used as container volume mounts. */
const BLOB_STORAGE_MOUNT_PREFIX = "~/.opencloud/blob_storage"

// Represents an image returned by /get-images (Container Registry)
type AvailableImage = {
  Id: string
  RepoTags: string[]
  Image: string
}

// A single port mapping entry: hostPort -> containerPort
type PortMapping = {
  hostPort: string
  containerPort: string
}

// A single environment variable entry
type EnvVar = {
  key: string
  value: string
}

// A single volume mount entry
type VolumeMount = {
  hostPath: string
  containerPath: string
}

// Sentinel values used in the image Select dropdown
const CUSTOM_IMAGE_VALUE = "__custom__"
const NO_IMAGES_VALUE = "__no_images__"

// formatBytes converts a byte count into a human-readable string
// (e.g. 1048576 → "1.0 MB").
function formatBytes(bytes: number): string {
  if (bytes == null || bytes <= 0) return "—"
  const units = ["B", "KB", "MB", "GB", "TB"]
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

function SearchParamsReader({ onCreateRequested }: { onCreateRequested: (image?: string) => void }) {
  const searchParams = useSearchParams()
  const handled = useRef(false)
  useEffect(() => {
    if (!handled.current && searchParams.get("create") === "true") {
      handled.current = true
      onCreateRequested(searchParams.get("image") ?? undefined)
    }
  }, [searchParams, onCreateRequested])
  return null
}

export default function ContainersPage() {
  const router = useRouter()
  const [containers, setContainers] = useState<ContainerItem[]>([])
  const [loading, setLoading] = useState(false)
  const [searchTerm, setSearchTerm] = useState("")

  // Service enabled state
  const [serviceEnabled, setServiceEnabled] = useState<boolean | null>(null)
  const [enablingService, setEnablingService] = useState(false)
  const [enableOutput, setEnableOutput] = useState<string[]>([])
  const [enableError, setEnableError] = useState<string | null>(null)
  const outputBoxRef = useRef<HTMLDivElement>(null)
  const [isStopDialogOpen, setIsStopDialogOpen] = useState(false)
  const [containerToStop, setContainerToStop] = useState<ContainerItem | null>(null)
  const [stopError, setStopError] = useState("")
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [containerToDelete, setContainerToDelete] = useState<ContainerItem | null>(null)
  // Tracks the container ID currently undergoing a start or stop action
  const [actionLoadingId, setActionLoadingId] = useState<string | null>(null)

  // Available images fetched from the Container Registry for the dropdown
  const [availableImages, setAvailableImages] = useState<AvailableImage[]>([])
  const [loadingImages, setLoadingImages] = useState(false)

  // Pull and run dialog state
  const [isPullRunDialogOpen, setIsPullRunDialogOpen] = useState(false)
  const [isPullingAndRunning, setIsPullingAndRunning] = useState(false)
  // runImage holds the selected dropdown value; "__custom__" means the user typed a custom name
  const [runImage, setRunImage] = useState("")
  const [runCustomImage, setRunCustomImage] = useState("")
  const [runContainerName, setRunContainerName] = useState("")
  // Container name is optional (empty string is valid), but if provided it must follow
  // the same naming rules as function names: no spaces, max 50 characters.
  const isContainerNameValid = runContainerName === "" || isValidFunctionName(runContainerName)
  const {
    handleBeforeInput: handleContainerNameBeforeInput,
    handleChange: handleContainerNameChange,
    handlePaste: handleContainerNamePaste,
    resetWarning: resetContainerNameWarning,
  } = useFunctionNameWarning(setRunContainerName)
  const [runPorts, setRunPorts] = useState<PortMapping[]>([{ hostPort: "", containerPort: "" }])
  const [runEnvVars, setRunEnvVars] = useState<EnvVar[]>([{ key: "", value: "" }])
  const [runVolumes, setRunVolumes] = useState<VolumeMount[]>([{ hostPath: "", containerPath: "" }])
  // Blob storage buckets available as container volume mounts
  const [mountBuckets, setMountBuckets] = useState<{ name: string; volumeName?: string }[]>([])
  const [loadingMountBuckets, setLoadingMountBuckets] = useState(false)
  const [runRestartPolicy, setRunRestartPolicy] = useState("no")
  const [runAutoRemove, setRunAutoRemove] = useState(false)
  const [runCommand, setRunCommand] = useState("")
  // Fully-custom command mode: when enabled the user types raw "podman run" args
  const [runFullCustom, setRunFullCustom] = useState(false)
  const [runCustomCommand, setRunCustomCommand] = useState("")
  // Streaming progress output for pull-and-run
  const [pullRunOutput, setPullRunOutput] = useState<string[]>([])
  const [pullRunError, setPullRunError] = useState<string | null>(null)
  const pullRunOutputEndRef = useRef<HTMLDivElement>(null)

  // Check if service is enabled
  const checkServiceStatus = async () => {
    try {
      const res = await client.get<{ service: string; enabled: boolean }>("/get-service-status?service=containers")
      setServiceEnabled(res.data.enabled)
    } catch (err) {
      console.error("Failed to check service status:", err)
      setServiceEnabled(false)
    }
  }

  // Auto-scroll the output box whenever a new line is appended
  useEffect(() => {
    if (outputBoxRef.current) {
      outputBoxRef.current.scrollTop = outputBoxRef.current.scrollHeight
    }
  }, [enableOutput])

  // Auto-scroll the pull-and-run output box whenever a new line is appended
  useEffect(() => {
    if (pullRunOutputEndRef.current) {
      pullRunOutputEndRef.current.scrollIntoView({ behavior: "smooth" })
    }
  }, [pullRunOutput])

  // Enable the service with streaming output
  const handleEnableService = async () => {
    setEnablingService(true)
    setEnableOutput([])
    setEnableError(null)

    const appendLine = (line: string) => {
      setEnableOutput(prev => [...prev, line])
    }

    try {
      const response = await fetch("/api/enable-service-stream", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ service: "containers" }),
      })

      if (!response.ok) {
        throw new Error(`Server returned ${response.status}`)
      }

      if (!response.body) {
        throw new Error("No response body")
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ""
      let installationSucceeded = false

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split("\n")
        buffer = lines.pop() ?? ""

        for (const line of lines) {
          if (line.startsWith("data: ")) {
            const data = line.slice(6).trim()
            if (data) {
              appendLine(data)
            }
          } else if (line.startsWith("event: done")) {
            installationSucceeded = true
          } else if (line.startsWith("event: error")) {
            // error data line follows; will be appended via the data: handler above
          }
        }
      }

      if (installationSucceeded) {
        setServiceEnabled(true)
        fetchContainers()
      }
    } catch (err) {
      console.error("Failed to enable service:", err)
      setEnableError(err instanceof Error ? err.message : "Failed to enable service")
    } finally {
      setEnablingService(false)
    }
  }

  // Fetch containers
  const fetchContainers = async () => {
    setLoading(true)
    try {
      const res = await client.get<ContainerItem[]>("/get-containers")
      setContainers(res.data)
    } catch (err) {
      console.error("Failed to fetch containers:", err)
    } finally {
      setLoading(false)
    }
  }

  // Fetch available images from the Container Registry to populate the image dropdown
  const fetchAvailableImages = async () => {
    setLoadingImages(true)
    try {
      const res = await client.get<AvailableImage[]>("/get-images")
      setAvailableImages(res.data)
    } catch (err) {
      console.error("Failed to fetch available images:", err)
    } finally {
      setLoadingImages(false)
    }
  }

  // Fetch blob storage buckets marked as container volume mounts
  const fetchMountBuckets = async () => {
    setLoadingMountBuckets(true)
    try {
      const res = await client.get<{ name: string }[]>("/list-container-mount-buckets")
      setMountBuckets(res.data || [])
    } catch (err) {
      console.error("Failed to fetch mount buckets:", err)
    } finally {
      setLoadingMountBuckets(false)
    }
  }

  useEffect(() => {
    checkServiceStatus()
  }, [])

  useEffect(() => {
    if (serviceEnabled) {
      fetchContainers()
    }
  }, [serviceEnabled])

  // Manage container actions
  const handleAction = async (id: string, action: "start" | "stop") => {
    setActionLoadingId(id)
    try {
      await client.post(`/containers/${id}/${action}`)
      await fetchContainers()
      return true
    } catch (err) {
      console.error(`Failed to ${action} container:`, err)
      return false
    } finally {
      setActionLoadingId(null)
    }
  }

  const openStopDialog = (container: ContainerItem) => {
    setStopError("")
    setContainerToStop(container)
    setIsStopDialogOpen(true)
  }

  const closeStopDialog = () => {
    setStopError("")
    setContainerToStop(null)
    setIsStopDialogOpen(false)
  }

  const handleStopContainer = async () => {
    if (!containerToStop) return

    const stopped = await handleAction(containerToStop.Id, "stop")
    if (!stopped) {
      setStopError("Failed to stop container. Please try again or check the logs.")
      return
    }

    closeStopDialog()
  }

  const openDeleteDialog = (container: ContainerItem) => {
    setContainerToDelete(container)
    setIsDeleteDialogOpen(true)
  }

  const handleDeleteContainer = async () => {
    if (!containerToDelete) return

    try {
      console.log("before delete")
      await client.post("/delete-container", { containerId: containerToDelete.Id })
      console.log("after delete")
      await fetchContainers()
      setIsDeleteDialogOpen(false)
      setContainerToDelete(null)
    } catch (err) {
      console.error("Failed to delete container:", err)
    }
  }

  // Reset the pull-and-run dialog form fields to defaults
  const resetPullRunForm = () => {
    setRunImage("")
    setRunCustomImage("")
    setRunContainerName("")
    resetContainerNameWarning()
    setRunPorts([{ hostPort: "", containerPort: "" }])
    setRunEnvVars([{ key: "", value: "" }])
    setRunVolumes([{ hostPath: "", containerPath: "", Z: false, U: false }])
    setRunRestartPolicy("no")
    setRunAutoRemove(false)
    setRunCommand("")
    setRunFullCustom(false)
    setRunCustomCommand("")
    setPullRunOutput([])
    setPullRunError(null)
  }

  // Submit handler for pulling and running a container
  const handlePullAndRun = async () => {
    // In fully-custom mode the user provides everything in one command string.
    if (runFullCustom) {
      if (!runCustomCommand.trim()) {
        toast.error("Please enter a custom container command")
        return
      }
    } else {
      // Resolve the actual image: use custom input when CUSTOM_IMAGE_VALUE is selected
      const resolvedImage = runImage === CUSTOM_IMAGE_VALUE ? runCustomImage : runImage
      if (!resolvedImage) {
        toast.error("Please select or enter an image name to pull and run")
        return
      }
    }

    setIsPullingAndRunning(true)
    setPullRunOutput([])
    setPullRunError(null)

    const appendLine = (line: string) => {
      setPullRunOutput(prev => [...prev, line])
    }

    try {
      let requestBody: Record<string, unknown>

      if (runFullCustom) {
        // Send the raw command string; the backend parses it.
        requestBody = { fullCustomCommand: runCustomCommand.trim() }
      } else {
        // Build the request payload with non-empty port, env, and volume entries
        const resolvedImage = runImage === CUSTOM_IMAGE_VALUE ? runCustomImage : runImage
        const ports = runPorts
          .filter(p => p.containerPort)
          .map(p => p.hostPort ? `${p.hostPort}:${p.containerPort}` : p.containerPort)

        const envVars = runEnvVars
          .filter(e => e.key)
          .map(e => (e.value ? `${e.key}=${e.value}` : e.key))

        const volumes = runVolumes
          .filter(v => v.hostPath && v.containerPath)
          .map(v => `${v.hostPath}:${v.containerPath}`)

        requestBody = {
          image: resolvedImage,
          name: runContainerName || undefined,
          ports,
          env: envVars,
          volumes,
          restartPolicy: runRestartPolicy,
          autoRemove: runAutoRemove,
          command: runCommand || undefined,
        }
      }

      const response = await fetch("/api/pull-and-run-stream", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(requestBody),
      })

      if (!response.ok) {
        const text = await response.text()
        throw new Error(text || `Server returned ${response.status}`)
      }

      if (!response.body) {
        throw new Error("No response body")
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ""
      let succeeded = false
      let errorMsg = ""

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split("\n")
        buffer = lines.pop() ?? ""

        for (const line of lines) {
          if (line.startsWith("data: ")) {
            const data = line.slice(6).trim()
            if (data) appendLine(data)
          } else if (line.startsWith("event: done")) {
            succeeded = true
          } else if (line.startsWith("event: error")) {
            errorMsg = "Pull and run failed"
          }
        }
      }

      if (succeeded) {
        await fetchContainers()
        toast.success("Container pulled and started successfully!")
        setTimeout(() => {
          resetPullRunForm()
          setIsPullRunDialogOpen(false)
          router.replace("/compute/containers")
        }, 1500)
      } else {
        setPullRunError(errorMsg || "Failed to pull and run container. Please check the image name and try again.")
      }
    } catch (err) {
      console.error("Failed to pull and run container:", err)
      const msg = err instanceof Error ? err.message : "Failed to pull and run container. Please check the logs."
      setPullRunError(msg)
    } finally {
      setIsPullingAndRunning(false)
    }
  }

  // Helpers for dynamic port mapping rows
  const addPortMapping = () => setRunPorts(prev => [...prev, { hostPort: "", containerPort: "" }])
  const removePortMapping = (index: number) =>
    setRunPorts(prev => prev.filter((_, i) => i !== index))
  const updatePortMapping = (index: number, field: keyof PortMapping, value: string) =>
    setRunPorts(prev => prev.map((p, i) => (i === index ? { ...p, [field]: value } : p)))

  // Helpers for dynamic environment variable rows
  const addEnvVar = () => setRunEnvVars(prev => [...prev, { key: "", value: "" }])
  const removeEnvVar = (index: number) =>
    setRunEnvVars(prev => prev.filter((_, i) => i !== index))
  const updateEnvVar = (index: number, field: keyof EnvVar, value: string) =>
    setRunEnvVars(prev => prev.map((e, i) => (i === index ? { ...e, [field]: value } : e)))

  // Helpers for dynamic volume mount rows
  const addVolumeMount = () => setRunVolumes(prev => [...prev, { hostPath: "", containerPath: "" }])
  const removeVolumeMount = (index: number) =>
    setRunVolumes(prev => prev.filter((_, i) => i !== index))
  const updateVolumeMount = (index: number, field: keyof VolumeMount, value: string | boolean) =>
    setRunVolumes(prev => prev.map((v, i) => (i === index ? { ...v, [field]: value } : v)))

  // Filter containers based on search
  const filteredContainers = containers.filter(container => 
    container.Names?.[0]?.toLowerCase().includes(searchTerm.toLowerCase()) ||
    container.Image?.toLowerCase().includes(searchTerm.toLowerCase()) ||
    container.Id?.toLowerCase().includes(searchTerm.toLowerCase())
  )

  // Calculate statistics
  const totalContainers = containers.length
  const runningContainers = containers.filter(c => c.State === "running").length
  const stoppedContainers = containers.filter(c => c.State !== "running").length

  // Show loading state while checking service status
  if (serviceEnabled === null) {
    return (
      <DashboardShell>
        <div className="flex items-center justify-center h-64">
          <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      </DashboardShell>
    )
  }

  // Show enable prompt if service is not enabled
  if (!serviceEnabled) {
    return (
      <DashboardShell>
        <DashboardHeader heading="Containers" text="Manage your containers" />
        <div className="flex flex-col items-center gap-6 pt-8">
          <Card className="max-w-md w-full">
            <CardHeader className="text-center">
              <div className="mx-auto p-3 rounded-full bg-blue-50 w-fit mb-4">
                <Container className="h-8 w-8 text-blue-600" />
              </div>
              <CardTitle>Enable Containers Service</CardTitle>
              <CardDescription>
                The Containers service is not yet enabled. Enable it to start pulling and managing containers.
              </CardDescription>
            </CardHeader>
            <CardContent className="flex justify-center">
              <Button onClick={handleEnableService} disabled={enablingService} size="lg">
                {enablingService ? (
                  <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <Power className="mr-2 h-4 w-4" />
                )}
                {enablingService ? "Enabling..." : "Enable Containers"}
              </Button>
            </CardContent>
          </Card>

          {(enablingService || enableOutput.length > 0 || enableError) && (
            <Card className="w-full max-w-2xl">
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">Installation Output</CardTitle>
              </CardHeader>
              <CardContent>
                <div
                  ref={outputBoxRef}
                  className="bg-black text-green-400 font-mono text-xs p-4 rounded-lg h-64 overflow-y-auto whitespace-pre-wrap"
                >
                  {enableOutput.map((line, i) => (
                    <div key={i}>{line}</div>
                  ))}
                  {enablingService && (
                    <span className="animate-pulse">▌</span>
                  )}
                  {enableError && (
                    <div className="text-red-400 mt-2">[ERROR] {enableError}</div>
                  )}
                </div>
              </CardContent>
            </Card>
          )}
        </div>
      </DashboardShell>
    )
  }

  return (
    <DashboardShell>
      <Suspense fallback={null}>
        <SearchParamsReader onCreateRequested={(image) => {
          if (image) {
            setRunImage(image)
          }
          fetchAvailableImages()
          setIsPullRunDialogOpen(true)
        }} />
      </Suspense>
      <DashboardHeader heading="Containers" text="Manage your containers">
        <div className="flex items-center space-x-2">
          <Button variant="outline" onClick={fetchContainers} disabled={loading}>
            <RefreshCw className={`mr-2 h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
        </div>
      </DashboardHeader>

      {/* Statistics Cards */}
      <div className="grid gap-6 md:grid-cols-3">
        <Card className="border-l-4 border-l-blue-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Containers</CardTitle>
            <Package className="h-4 w-4 text-blue-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalContainers}</div>
            <p className="text-xs text-muted-foreground">All containers</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-green-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Running</CardTitle>
            <Activity className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{runningContainers}</div>
            <p className="text-xs text-muted-foreground">Active containers</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-gray-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Stopped</CardTitle>
            <Square className="h-4 w-4 text-gray-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stoppedContainers}</div>
            <p className="text-xs text-muted-foreground">Inactive containers</p>
          </CardContent>
        </Card>
      </div>

      {/* Main Content: Container List + Quick Actions Sidebar */}
      <div className="grid gap-6 md:grid-cols-3">
        {/* Container List */}
        <div className="md:col-span-2">
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle>Containers</CardTitle>
                  <CardDescription>View and manage your containers</CardDescription>
                </div>
              </div>
              <div className="relative mt-4">
                <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  type="text"
                  placeholder="Search containers by name, image, or ID..."
                  className="pl-8"
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                />
              </div>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                {filteredContainers.map((c) => (
                  <div
                    key={c.Id}
                    className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors cursor-pointer"
                    onClick={() => router.push(`/compute/containers/${encodeURIComponent(c.Id)}`)}
                  >
                    <div className="flex items-center space-x-4 flex-1 min-w-0">
                      <div className={`shrink-0 p-2 rounded-lg ${c.State === "running" ? "bg-green-50" : "bg-gray-50"}`}>
                        <Container className={`h-5 w-5 ${c.State === "running" ? "text-green-600" : "text-gray-600"}`} />
                      </div>
                      <div className="space-y-1 flex-1 min-w-0">
                        <div className="flex items-center space-x-2">
                          <h4 className="font-medium text-foreground truncate">
                            {c.Names?.[0]?.replace(/^\//, "") || "Unnamed"}
                          </h4>
                          <Badge variant={c.State === "running" ? "default" : "secondary"}>
                            {c.State}
                          </Badge>
                        </div>
                        <div className="hidden lg:flex items-center space-x-4 text-xs text-muted-foreground">
                          <span className="flex items-center">
                            <ImageIcon className="h-3 w-3 mr-1" />
                            {stripRegistryPrefix(c.Image)}
                          </span>
                          <span>•</span>
                          <span className="flex items-center">
                            ID: {c.Id.slice(7, 19)}
                          </span>

                        </div>
                      </div>
                    </div>
                    <div className="flex shrink-0 items-center space-x-2 ml-4">
                      {c.State !== "running" && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={(e) => { e.stopPropagation(); handleAction(c.Id, "start") }}
                          disabled={actionLoadingId === c.Id}
                        >
                          {actionLoadingId === c.Id ? (
                            <RefreshCw className="h-4 w-4 mr-1 animate-spin" />
                          ) : (
                            <Play className="h-4 w-4 mr-1" />
                          )}
                          Start
                        </Button>
                      )}
                      {c.State === "running" && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={(e) => { e.stopPropagation(); openStopDialog(c) }}
                          disabled={actionLoadingId === c.Id}
                        >
                          <Square className="h-4 w-4 mr-1" />
                          Stop
                        </Button>
                      )}
                      <Button
                        variant="destructive"
                        size="sm"
                        onClick={(e) => { e.stopPropagation(); openDeleteDialog(c) }}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>
                ))}
                {filteredContainers.length === 0 && !loading && (
                  <div className="text-center py-12">
                    <Container className="mx-auto h-12 w-12 text-muted-foreground" />
                    <h3 className="mt-4 text-lg font-semibold">No containers found</h3>
                    <p className="mt-2 text-sm text-muted-foreground">
                      {searchTerm ? "Try adjusting your search terms" : "No containers are currently available"}
                    </p>
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Quick Actions Sidebar */}
        <div>
          <Card>
            <CardHeader>
              <CardTitle>Quick Actions</CardTitle>
              <CardDescription>Common container operations</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {/* Pull and Run Container dialog */}
              <Dialog
                open={isPullRunDialogOpen}
                onOpenChange={(open) => {
                  setIsPullRunDialogOpen(open)
                  if (open) {
                    fetchAvailableImages()
                    fetchMountBuckets()
                  } else {
                    resetPullRunForm()
                    router.replace("/compute/containers")
                  }
                }}
              >
                <DialogTrigger asChild>
                  <Button variant="ghost" className="w-full justify-start h-auto p-4 bg-blue-50 hover:bg-blue-100 dark:bg-blue-950/40 dark:hover:bg-blue-900/50">
                    <div className="flex items-center space-x-3 min-w-0">
                      <div className="shrink-0 p-2 rounded-lg bg-white dark:bg-white/10 text-blue-600 dark:text-blue-400">
                        <Download className="h-4 w-4" />
                      </div>
                      <div className="text-left min-w-0">
                        <div className="font-semibold text-sm text-gray-900 dark:text-white whitespace-normal break-words">Pull and Run Container</div>
                        <div className="text-xs text-muted-foreground whitespace-normal break-words">Pull an image from your Container Registry</div>
                      </div>
                    </div>
                  </Button>
                </DialogTrigger>

                <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
                  <DialogHeader>
                    <DialogTitle>Pull and Run Container</DialogTitle>
                    <DialogDescription>
                      Specify an image to pull from your registry or another, then configure common runtime options before starting the container.
                    </DialogDescription>
                  </DialogHeader>

                  <div className="grid gap-4 py-4">
                    {!runFullCustom ? (
                      /* ── Individual fields ── */
                      <>
                    {/* Image — dropdown of available images with a custom-image fallback */}
                    <div className="grid gap-2">
                      <Label htmlFor="runImage">Image *</Label>
                      <Select value={runImage} onValueChange={setRunImage}>
                        <SelectTrigger id="runImage" disabled={loadingImages}>
                          <SelectValue
                            placeholder={
                              loadingImages ? "Loading images…" : "Select an image"
                            }
                          />
                        </SelectTrigger>
                        <SelectContent>
                          {/* If runImage is set to a value not in availableImages and not a sentinel,
                              surface it as the first option so the Select can display it correctly */}
                          {runImage &&
                            runImage !== CUSTOM_IMAGE_VALUE &&
                            runImage !== NO_IMAGES_VALUE &&
                            !availableImages.some(
                              (img) => (img.RepoTags?.[0] || img.Image || img.Id) === runImage
                            ) && (
                              <SelectItem key="__preselected__" value={runImage}>
                                {stripRegistryPrefix(runImage)}
                              </SelectItem>
                            )}
                          {availableImages.map((img) => {
                            // RepoTags is the preferred display value (e.g. "nginx:latest").
                            // Fall back to Image (short name) then Id if no tags are present.
                            const tag = img.RepoTags?.[0] || img.Image || img.Id
                            return (
                              <SelectItem key={img.Id} value={tag}>
                                {stripRegistryPrefix(tag)}
                              </SelectItem>
                            )
                          })}
                          {availableImages.length === 0 && !loadingImages && (
                            <SelectItem value={NO_IMAGES_VALUE} disabled>
                              No images found in registry
                            </SelectItem>
                          )}
                          <SelectItem value={CUSTOM_IMAGE_VALUE}>Enter custom image…</SelectItem>
                        </SelectContent>
                      </Select>
                      {runImage === CUSTOM_IMAGE_VALUE && (
                        <Input
                          placeholder="nginx:latest"
                          value={runCustomImage}
                          onChange={(e) => setRunCustomImage(e.target.value)}
                        />
                      )}
                      <p className="text-xs text-muted-foreground">
                        Choose from your OpenCloud Container Registry
                      </p>
                    </div>

                    {/* Container Name */}
                    <div className="grid gap-2">
                      <Label htmlFor="runContainerName">Container Name</Label>
                      <Input
                        id="runContainerName"
                        placeholder="my-container"
                        value={runContainerName}
                        onChange={(e) => handleContainerNameChange(e.target.value)}
                        onBeforeInput={handleContainerNameBeforeInput}
                        onPaste={handleContainerNamePaste}
                        maxLength={FUNCTION_NAME_MAX_LENGTH}
                      />
                      <p className="text-xs text-muted-foreground">
                        Optional name for the container (--name). Cannot contain spaces and must be 50 characters or fewer.
                      </p>
                    </div>

                    {/* Port Mappings */}
                    <div className="grid gap-2">
                      <div className="flex items-center justify-between">
                        <Label>Port Mappings</Label>
                        <Button type="button" variant="outline" size="sm" onClick={addPortMapping}>
                          <Plus className="h-3 w-3 mr-1" />
                          Add Port
                        </Button>
                      </div>
                      {runPorts.map((port, index) => (
                        <div key={index} className="flex items-center space-x-2">
                          <Input
                            placeholder="Host port (optional, e.g. 8080)"
                            value={port.hostPort}
                            onChange={(e) => updatePortMapping(index, "hostPort", e.target.value)}
                          />
                          <span className="text-muted-foreground">:</span>
                          <Input
                            placeholder="Container port (e.g. 80)"
                            value={port.containerPort}
                            onChange={(e) => updatePortMapping(index, "containerPort", e.target.value)}
                          />
                          {runPorts.length > 1 && (
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              onClick={() => removePortMapping(index)}
                              className="shrink-0"
                            >
                              <X className="h-4 w-4" />
                            </Button>
                          )}
                        </div>
                      ))}
                      <p className="text-xs text-muted-foreground">
                        Map host ports to container ports (-p hostPort:containerPort). Leave the host port blank to let the system assign a random available port automatically.
                      </p>
                    </div>

                    {/* Environment Variables */}
                    <div className="grid gap-2">
                      <div className="flex items-center justify-between">
                        <Label>Environment Variables</Label>
                        <Button type="button" variant="outline" size="sm" onClick={addEnvVar}>
                          <Plus className="h-3 w-3 mr-1" />
                          Add Variable
                        </Button>
                      </div>
                      {runEnvVars.map((env, index) => (
                        <div key={index} className="flex items-center space-x-2">
                          <Input
                            placeholder="KEY"
                            value={env.key}
                            onChange={(e) => updateEnvVar(index, "key", e.target.value)}
                          />
                          <span className="text-muted-foreground">=</span>
                          <Input
                            placeholder="value"
                            value={env.value}
                            onChange={(e) => updateEnvVar(index, "value", e.target.value)}
                          />
                          {runEnvVars.length > 1 && (
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              onClick={() => removeEnvVar(index)}
                              className="shrink-0"
                            >
                              <X className="h-4 w-4" />
                            </Button>
                          )}
                        </div>
                      ))}
                      <p className="text-xs text-muted-foreground">
                        Set environment variables inside the container (-e KEY=value)
                      </p>
                    </div>

                    {/* Volume Mounts */}
                    <div className="grid gap-2">
                      <div className="flex items-center justify-between">
                        <Label>Volume Mounts</Label>
                        <Button type="button" variant="outline" size="sm" onClick={addVolumeMount}>
                          <Plus className="h-3 w-3 mr-1" />
                          Add Volume
                        </Button>
                      </div>
                      {runVolumes.map((vol, index) => (
                        <div key={index} className="flex items-center space-x-2">
                          <Select
                            value={vol.hostPath}
                            onValueChange={(value) => updateVolumeMount(index, "hostPath", value)}
                          >
                            <SelectTrigger className="w-[200px]">
                              <SelectValue placeholder="Select bucket" />
                            </SelectTrigger>
                            <SelectContent>
                              {mountBuckets.length === 0 ? (
                                <SelectItem value="__no_buckets__" disabled>
                                  No mount buckets available
                                </SelectItem>
                              ) : (
                                mountBuckets.map((bucket) => (
                                  <SelectItem key={bucket.name} value={bucket.volumeName || `${BLOB_STORAGE_MOUNT_PREFIX}/${bucket.name}`}>
                                    {bucket.name}
                                  </SelectItem>
                                ))
                              )}
                            </SelectContent>
                          </Select>
                          <span className="text-muted-foreground">:</span>
                          <Input
                            placeholder="Container path (e.g. /app/data)"
                            value={vol.containerPath}
                            onChange={(e) => updateVolumeMount(index, "containerPath", e.target.value)}
                          />
                          {runVolumes.length > 1 && (
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              onClick={() => removeVolumeMount(index)}
                              className="shrink-0"
                            >
                              <X className="h-4 w-4" />
                            </Button>
                          )}
                        </div>
                      ))}
                      <p className="text-xs text-muted-foreground">
                        Select a Blob Storage bucket marked as a container mount to use as a volume. Create mount-enabled buckets in Blob Storage.
                      </p>
                    </div>

                    {/* Restart Policy */}
                    <div className="grid gap-2">
                      <Label htmlFor="runRestartPolicy">Restart Policy</Label>
                      <Select value={runRestartPolicy} onValueChange={setRunRestartPolicy}>
                        <SelectTrigger id="runRestartPolicy">
                          <SelectValue placeholder="Select restart policy" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="no">No (never restart)</SelectItem>
                          <SelectItem value="always">Always</SelectItem>
                          <SelectItem value="on-failure">On failure</SelectItem>
                          <SelectItem value="unless-stopped">Unless stopped</SelectItem>
                        </SelectContent>
                      </Select>
                      <p className="text-xs text-muted-foreground">
                        Container restart behavior (--restart)
                      </p>
                    </div>

                    {/* Command Override */}
                    <div className="grid gap-2">
                      <Label htmlFor="runCommand">Command Override</Label>
                      <Input
                        id="runCommand"
                        placeholder='e.g. /bin/bash or "npm start"'
                        value={runCommand}
                        onChange={(e) => setRunCommand(e.target.value)}
                      />
                      <p className="text-xs text-muted-foreground">
                        Optional command to override the image default CMD
                      </p>
                    </div>

                    {/* Auto-Remove */}
                    <div className="flex items-center space-x-2">
                      <input
                        type="checkbox"
                        id="runAutoRemove"
                        checked={runAutoRemove}
                        onChange={(e) => setRunAutoRemove(e.target.checked)}
                        className="h-4 w-4 rounded border-gray-300"
                      />
                      <Label htmlFor="runAutoRemove" className="text-sm font-normal cursor-pointer">
                        Automatically remove container when it stops (--rm)
                      </Label>
                    </div>
                      </>
                    ) : (
                      /* ── Fully custom command input ── */
                      <div className="grid gap-2">
                        <Label htmlFor="runCustomCommand">Custom Command *</Label>
                        <Textarea
                          id="runCustomCommand"
                          placeholder={`-p 8080:80 -e NGINX_HOST=example.com -v /my/content:/usr/share/nginx/html:ro nginx:latest`}
                          value={runCustomCommand}
                          onChange={(e) => setRunCustomCommand(e.target.value)}
                          disabled={isPullingAndRunning}
                          rows={4}
                          className="font-mono text-sm"
                        />
                        <p className="text-xs text-muted-foreground">
                          Enter everything you would type after <code className="font-mono">podman run</code>. Supported flags:
                          {" "}<code className="font-mono">-p</code>, <code className="font-mono">-e</code>,{" "}
                          <code className="font-mono">-v</code>, <code className="font-mono">--name</code>,{" "}
                          <code className="font-mono">--restart</code>, <code className="font-mono">--rm</code>.
                          The image must be the first non-flag argument.
                        </p>
                      </div>
                    )}

                    {/* Toggle: Fully Custom Command — low-profile, at the bottom */}
                    <div className="flex items-center gap-2 pt-1">
                      <Switch
                        id="runFullCustom"
                        checked={runFullCustom}
                        onCheckedChange={setRunFullCustom}
                        disabled={isPullingAndRunning}
                      />
                      <Label htmlFor="runFullCustom" className="text-xs text-muted-foreground font-normal cursor-pointer">
                        Use custom <code className="font-mono">podman run</code> command
                      </Label>
                    </div>

                    {/* Streaming progress output */}
                    {(isPullingAndRunning || pullRunOutput.length > 0 || pullRunError) && (
                      <div className="space-y-1">
                        <Label>Pull Progress</Label>
                        <div className="max-h-32 overflow-y-auto overflow-x-hidden rounded-md border bg-black p-3 font-mono text-xs text-green-400">
                          {pullRunOutput.map((line, i) => (
                            <div key={i} className="break-all whitespace-pre-wrap">{line}</div>
                          ))}
                          {pullRunError && (
                            <div className="text-red-400 break-all whitespace-pre-wrap">{pullRunError}</div>
                          )}
                          {isPullingAndRunning && (
                            <div className="animate-pulse">▌</div>
                          )}
                          <div ref={pullRunOutputEndRef} />
                        </div>
                      </div>
                    )}
                  </div>

                  <DialogFooter>
                    <Button
                      variant="outline"
                      onClick={() => setIsPullRunDialogOpen(false)}
                      disabled={isPullingAndRunning}
                    >
                      Cancel
                    </Button>
                    <Button
                      onClick={handlePullAndRun}
                      disabled={
                        isPullingAndRunning || (
                          runFullCustom
                            ? !runCustomCommand.trim()
                            : (
                                !runImage ||
                                runImage === NO_IMAGES_VALUE ||
                                (runImage === CUSTOM_IMAGE_VALUE && !runCustomImage) ||
                                !isContainerNameValid
                              )
                        )
                      }
                    >
                      {isPullingAndRunning ? (
                        <>
                          <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                          Pulling and Running...
                        </>
                      ) : (
                        <>
                          <Play className="mr-2 h-4 w-4" />
                          Pull and Run
                        </>
                      )}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </CardContent>
          </Card>
        </div>
      </div>

      <Dialog
        open={isStopDialogOpen}
        onOpenChange={(open) => {
          if (!open) {
            closeStopDialog()
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Stop Container</DialogTitle>
            <DialogDescription>
              Are you sure you want to stop{" "}
              <span className="font-medium">
                {containerToStop?.Names?.[0]?.replace(/^\//, "") || "this container"}
              </span>
              ? You can start it again later.
            </DialogDescription>
          </DialogHeader>
          {stopError && (
            <p className="text-sm text-destructive">
              {stopError}
            </p>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={closeStopDialog} disabled={actionLoadingId === containerToStop?.Id}>
              Cancel
            </Button>
            <Button onClick={handleStopContainer} disabled={actionLoadingId === containerToStop?.Id}>
              {actionLoadingId === containerToStop?.Id ? (
                <>
                  <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                  Stopping...
                </>
              ) : (
                "Stop"
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Container</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete{" "}
              <span className="font-medium">
                {containerToDelete?.Names?.[0]?.replace(/^\//, "") || "this container"}
              </span>
              ? This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsDeleteDialogOpen(false)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleDeleteContainer}>
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </DashboardShell>
  )
}
