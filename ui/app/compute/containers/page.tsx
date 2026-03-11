'use client'

import { useEffect, useState } from "react"
import axios from "axios"
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
import client from "@/app/utility/post"
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
} from "lucide-react"

type ContainerItem = {
  Id: string
  Names: string[]
  Image: string
  State: string
  Status: string
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

export default function ContainersPage() {
  const [containers, setContainers] = useState<ContainerItem[]>([])
  const [loading, setLoading] = useState(false)
  const [searchTerm, setSearchTerm] = useState("")

  // Pull and run dialog state
  const [isPullRunDialogOpen, setIsPullRunDialogOpen] = useState(false)
  const [isPullingAndRunning, setIsPullingAndRunning] = useState(false)
  const [runImage, setRunImage] = useState("")
  const [runContainerName, setRunContainerName] = useState("")
  const [runPorts, setRunPorts] = useState<PortMapping[]>([{ hostPort: "", containerPort: "" }])
  const [runEnvVars, setRunEnvVars] = useState<EnvVar[]>([{ key: "", value: "" }])
  const [runVolumes, setRunVolumes] = useState<VolumeMount[]>([{ hostPath: "", containerPath: "" }])
  const [runRestartPolicy, setRunRestartPolicy] = useState("no")
  const [runAutoRemove, setRunAutoRemove] = useState(false)
  const [runCommand, setRunCommand] = useState("")

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

  useEffect(() => {
    fetchContainers()
  }, [])

  // Manage container actions
  const handleAction = async (id: string, action: "start" | "stop" | "remove") => {
    try {
      if (action === "remove") {
        await axios.delete(`/api/containers/${id}`)
      } else {
        await axios.post(`/api/containers/${id}/${action}`)
      }
      fetchContainers() // refresh list
    } catch (err) {
      console.error(`Failed to ${action} container:`, err)
    }
  }

  // Reset the pull-and-run dialog form fields to defaults
  const resetPullRunForm = () => {
    setRunImage("")
    setRunContainerName("")
    setRunPorts([{ hostPort: "", containerPort: "" }])
    setRunEnvVars([{ key: "", value: "" }])
    setRunVolumes([{ hostPath: "", containerPath: "" }])
    setRunRestartPolicy("no")
    setRunAutoRemove(false)
    setRunCommand("")
  }

  // Submit handler for pulling and running a container
  const handlePullAndRun = async () => {
    if (!runImage) {
      alert("Please provide an image name to pull and run")
      return
    }

    setIsPullingAndRunning(true)
    try {
      // Build the request payload with non-empty port, env, and volume entries
      const ports = runPorts
        .filter(p => p.hostPort && p.containerPort)
        .map(p => `${p.hostPort}:${p.containerPort}`)

      const envVars = runEnvVars
        .filter(e => e.key)
        .map(e => (e.value ? `${e.key}=${e.value}` : e.key))

      const volumes = runVolumes
        .filter(v => v.hostPath && v.containerPath)
        .map(v => `${v.hostPath}:${v.containerPath}`)

      await client.post("/pull-and-run", {
        image: runImage,
        name: runContainerName || undefined,
        ports,
        env: envVars,
        volumes,
        restartPolicy: runRestartPolicy,
        autoRemove: runAutoRemove,
        command: runCommand || undefined,
      })

      resetPullRunForm()
      setIsPullRunDialogOpen(false)
      await fetchContainers()
      alert("Container pulled and started successfully!")
    } catch (err) {
      console.error("Failed to pull and run container:", err)
      alert("Failed to pull and run container. Please check the logs.")
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
  const updateVolumeMount = (index: number, field: keyof VolumeMount, value: string) =>
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

  return (
    <DashboardShell>
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
                    className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
                  >
                    <div className="flex items-center space-x-4 flex-1">
                      <div className={`p-2 rounded-lg ${c.State === "running" ? "bg-green-50" : "bg-gray-50"}`}>
                        <Container className={`h-5 w-5 ${c.State === "running" ? "text-green-600" : "text-gray-600"}`} />
                      </div>
                      <div className="space-y-1 flex-1 min-w-0">
                        <div className="flex items-center space-x-2">
                          <h4 className="font-medium truncate">
                            {c.Names?.[0]?.replace(/^\//, "") || "Unnamed"}
                          </h4>
                          <Badge variant={c.State === "running" ? "default" : "secondary"}>
                            {c.State}
                          </Badge>
                        </div>
                        <div className="flex items-center space-x-4 text-xs text-muted-foreground">
                          <span className="flex items-center">
                            <ImageIcon className="h-3 w-3 mr-1" />
                            {c.Image}
                          </span>
                          <span>•</span>
                          <span className="flex items-center">
                            ID: {c.Id.slice(7, 19)}
                          </span>
                          <span>•</span>
                          <span>{c.Status}</span>
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center space-x-2 ml-4">
                      {c.State !== "running" && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => handleAction(c.Id, "start")}
                        >
                          <Play className="h-4 w-4 mr-1" />
                          Start
                        </Button>
                      )}
                      {c.State === "running" && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => handleAction(c.Id, "stop")}
                        >
                          <Square className="h-4 w-4 mr-1" />
                          Stop
                        </Button>
                      )}
                      <Button
                        variant="destructive"
                        size="sm"
                        onClick={() => handleAction(c.Id, "remove")}
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
                  if (!open) resetPullRunForm()
                }}
              >
                <DialogTrigger asChild>
                  <Button variant="ghost" className="w-full justify-start h-auto p-4 bg-blue-50 hover:bg-blue-100">
                    <div className="flex items-center space-x-3">
                      <div className="p-2 rounded-lg bg-white text-blue-600">
                        <Download className="h-4 w-4" />
                      </div>
                      <div className="text-left">
                        <div className="font-medium text-sm">Pull and Run Container</div>
                        <div className="text-xs text-muted-foreground">Pull an image from your Container Registry</div>
                      </div>
                    </div>
                  </Button>
                </DialogTrigger>

                <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
                  <DialogHeader>
                    <DialogTitle>Pull and Run Container</DialogTitle>
                    <DialogDescription>
                      Specify an image to pull and configure common runtime options before starting the container.
                    </DialogDescription>
                  </DialogHeader>

                  <div className="grid gap-4 py-4">
                    {/* Image */}
                    <div className="grid gap-2">
                      <Label htmlFor="runImage">Image *</Label>
                      <Input
                        id="runImage"
                        placeholder="nginx:latest"
                        value={runImage}
                        onChange={(e) => setRunImage(e.target.value)}
                      />
                      <p className="text-xs text-muted-foreground">
                        Image name and optional tag to pull (e.g., nginx:latest, ubuntu:22.04)
                      </p>
                    </div>

                    {/* Container Name */}
                    <div className="grid gap-2">
                      <Label htmlFor="runContainerName">Container Name</Label>
                      <Input
                        id="runContainerName"
                        placeholder="my-container"
                        value={runContainerName}
                        onChange={(e) => setRunContainerName(e.target.value)}
                      />
                      <p className="text-xs text-muted-foreground">
                        Optional name for the container (--name)
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
                            placeholder="Host port (e.g. 8080)"
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
                        Map host ports to container ports (-p hostPort:containerPort)
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
                          <Input
                            placeholder="Host path (e.g. /data)"
                            value={vol.hostPath}
                            onChange={(e) => updateVolumeMount(index, "hostPath", e.target.value)}
                          />
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
                        Mount host directories into the container (-v hostPath:containerPath)
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
                      disabled={isPullingAndRunning || !runImage}
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
    </DashboardShell>
  )
}
