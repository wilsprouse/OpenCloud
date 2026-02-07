'use client'

import { useEffect, useState } from "react"
import axios from "axios"
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
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import client from "@/app/utility/post"
import { 
  Container, 
  RefreshCw, 
  Download, 
  Upload, 
  Search,
  Play,
  Square,
  Trash2,
  Copy,
  Package,
  HardDrive,
  Layers,
  FileText,
  Power
} from "lucide-react"

type Image = {
  Id: string
  Names: string[]
  Image: string
  RepoTags: string[]
  State: string
  Size: string
  Status: string
}

// Constants
const REGISTRY_URL = "registry.opencloud.local"

export default function ContainerRegistry() {
  const [images, setImages] = useState<Image[]>([])
  const [loading, setLoading] = useState(false)
  const [searchTerm, setSearchTerm] = useState("")
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [isBuilding, setIsBuilding] = useState(false)
  
  // Service enabled state
  const [serviceEnabled, setServiceEnabled] = useState<boolean | null>(null)
  const [enablingService, setEnablingService] = useState(false)
  
  // Dockerfile form state
  const [dockerfileContent, setDockerfileContent] = useState("")
  const [imageName, setImageName] = useState("")
  const [imageTag, setImageTag] = useState("latest")
  const [buildContext, setBuildContext] = useState(".")
  const [nocache, setNocache] = useState(false)
  const [platform, setPlatform] = useState("linux/amd64")

  // Check if service is enabled
  const checkServiceStatus = async () => {
    try {
      const res = await client.get<{ service: string; enabled: boolean }>("/get-service-status?service=container_registry")
      setServiceEnabled(res.data.enabled)
    } catch (err) {
      console.error("Failed to check service status:", err)
      setServiceEnabled(false)
    }
  }

  // Enable the service
  const handleEnableService = async () => {
    setEnablingService(true)
    try {
      await client.post("/enable-service", { service: "container_registry" })
      setServiceEnabled(true)
      fetchImages()
    } catch (err) {
      console.error("Failed to enable service:", err)
    } finally {
      setEnablingService(false)
    }
  }

  // Fetch containers
  const fetchImages = async () => {
    setLoading(true)
    try {
      const res = await client.get<Image[]>("/get-containers")
      setImages(res.data)
    } catch (err) {
      console.error("Failed to fetch containers:", err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    checkServiceStatus()
  }, [])

  useEffect(() => {
    if (serviceEnabled) {
      fetchImages()
    }
  }, [serviceEnabled])

  // Manage container actions
  const handleAction = async (id: string, action: "start" | "stop" | "remove") => {
    try {
      if (action === "remove") {
        await axios.delete(`/api/containers/${id}`)
      } else {
        await axios.post(`/api/containers/${id}/${action}`)
      }
      fetchImages() // refresh list
    } catch (err) {
      console.error(`Failed to ${action} container:`, err)
    }
  }

  // Calculate statistics
  const totalImages = images.length
  const runningContainers = images.filter(c => c.State === "running").length
  const totalSize = images.reduce((acc, c) => acc + Number(c.Size), 0)
  const stoppedContainers = images.filter(c => c.State !== "running").length

  // Filter images based on search
  const filteredImages = images.filter(c => 
    c.Names?.[0]?.toLowerCase().includes(searchTerm.toLowerCase()) ||
    c.RepoTags?.[0]?.toLowerCase().includes(searchTerm.toLowerCase()) ||
    c.Id?.toLowerCase().includes(searchTerm.toLowerCase())
  )

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text)
    } catch (err) {
      console.error("Failed to copy to clipboard:", err)
    }
  }

  // Handle dockerfile upload and build
  const handleBuildImage = async () => {
    if (!dockerfileContent || !imageName) {
      alert("Please provide both a Dockerfile and an image name")
      return
    }

    setIsBuilding(true)
    try {
      // Send the dockerfile and build parameters to the backend
      await client.post("/build-image", {
        dockerfile: dockerfileContent,
        imageName: `${imageName}:${imageTag}`,
        context: buildContext,
        nocache: nocache,
        platform: platform,
      })
      
      // Reset form and close dialog
      setDockerfileContent("")
      setImageName("")
      setImageTag("latest")
      setBuildContext(".")
      setNocache(false)
      setPlatform("linux/amd64")
      setIsDialogOpen(false)
      
      // Refresh the image list
      await fetchImages()
      
      alert("Image built successfully!")
    } catch (err) {
      console.error("Failed to build image:", err)
      alert("Failed to build image. Please check the logs.")
    } finally {
      setIsBuilding(false)
    }
  }

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
        <DashboardHeader heading="Container Registry" text="Manage your container images and running containers" />
        <div className="flex items-center justify-center min-h-[400px]">
          <Card className="max-w-md w-full">
            <CardHeader className="text-center">
              <div className="mx-auto p-3 rounded-full bg-blue-50 w-fit mb-4">
                <Container className="h-8 w-8 text-blue-600" />
              </div>
              <CardTitle>Enable Container Registry Service</CardTitle>
              <CardDescription>
                The Container Registry service is not yet enabled. Enable it to start managing container images and deployments.
              </CardDescription>
            </CardHeader>
            <CardContent className="flex justify-center">
              <Button onClick={handleEnableService} disabled={enablingService} size="lg">
                <Power className="mr-2 h-4 w-4" />
                {enablingService ? "Enabling..." : "Enable Container Registry"}
              </Button>
            </CardContent>
          </Card>
        </div>
      </DashboardShell>
    )
  }

  return (
    <DashboardShell>
      <DashboardHeader 
        heading="Container Registry" 
        text="Manage your container images and running containers"
      >
        <Button onClick={fetchImages} disabled={loading}>
          <RefreshCw className={`mr-2 h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
          {loading ? "Refreshing..." : "Refresh"}
        </Button>
      </DashboardHeader>

      {/* Statistics Cards */}
      <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
        <Card className="border-l-4 border-l-blue-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Images</CardTitle>
            <Package className="h-4 w-4 text-blue-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalImages}</div>
            <p className="text-xs text-muted-foreground">Container images in registry</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-green-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Running</CardTitle>
            <Container className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{runningContainers}</div>
            <p className="text-xs text-muted-foreground">Active containers</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-orange-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Stopped</CardTitle>
            <Square className="h-4 w-4 text-orange-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stoppedContainers}</div>
            <p className="text-xs text-muted-foreground">Inactive containers</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-purple-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Storage Used</CardTitle>
            <HardDrive className="h-4 w-4 text-purple-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{(totalSize / 1_000_000_000).toFixed(2)} GB</div>
            <p className="text-xs text-muted-foreground">Total image size</p>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-6 md:grid-cols-3">
        {/* Main Container List */}
        <div className="md:col-span-2">
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle>Container Images</CardTitle>
                  <CardDescription>Manage and control your containers</CardDescription>
                </div>
              </div>
              <div className="relative mt-4">
                <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
                <input
                  type="text"
                  placeholder="Search containers by name, image, or ID..."
                  className="w-full pl-8 pr-4 py-2 border rounded-md bg-background"
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                />
              </div>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                {filteredImages.map((c) => {
                  const isRunning = c.State === "running"
                  return (
                    <div
                      key={c.Id}
                      className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
                    >
                      <div className="flex items-center space-x-4 flex-1">
                        <div className={`p-2 rounded-lg ${isRunning ? 'bg-green-50' : 'bg-muted'}`}>
                          <Container className={`h-5 w-5 ${isRunning ? 'text-green-600' : 'text-muted-foreground'}`} />
                        </div>
                        <div className="space-y-1 flex-1 min-w-0">
                          <div className="flex items-center space-x-2">
                            <h4 className="font-medium truncate">
                              {c.RepoTags?.[0]?.replace(/^\//, "") || "Unnamed"}
                            </h4>
                            <Badge 
                              variant="outline" 
                              className={isRunning ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-800"}
                            >
                              {isRunning ? "Running" : "Stopped"}
                            </Badge>
                          </div>
                          <p className="text-sm text-muted-foreground truncate">{c.Image}</p>
                          <div className="flex items-center space-x-4 text-xs text-muted-foreground">
                            <span className="flex items-center">
                              <Layers className="h-3 w-3 mr-1" />
                              ID: {c.Id.slice(7, 19)}
                            </span>
                            <span>•</span>
                            <span className="flex items-center">
                              <HardDrive className="h-3 w-3 mr-1" />
                              {(Number(c.Size) / 1_000_000).toFixed(2)} MB
                            </span>
                            <span>•</span>
                            <span>{c.Status}</span>
                          </div>
                        </div>
                      </div>
                      <div className="flex items-center space-x-2 ml-4">
                        <Button 
                          variant="ghost" 
                          size="icon"
                          onClick={() => copyToClipboard(c.Id)}
                          title="Copy ID"
                        >
                          <Copy className="h-4 w-4" />
                        </Button>
                        {!isRunning && (
                          <Button
                            variant="default"
                            size="sm"
                            onClick={() => handleAction(c.Id, "start")}
                          >
                            <Play className="h-4 w-4 mr-1" />
                            Start
                          </Button>
                        )}
                        {isRunning && (
                          <Button
                            variant="secondary"
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
                  )
                })}
                {filteredImages.length === 0 && !loading && (
                  <div className="text-center py-12">
                    <Container className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
                    <h3 className="text-lg font-medium mb-2">No containers found</h3>
                    <p className="text-sm text-muted-foreground">
                      {searchTerm ? "Try adjusting your search terms" : "Deploy your first container to get started"}
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
              <CardDescription>Common registry operations</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
                <DialogTrigger asChild>
                  <Button variant="ghost" className="w-full justify-start h-auto p-4 bg-blue-50 hover:bg-blue-100">
                    <div className="flex items-center space-x-3">
                      <div className="p-2 rounded-lg bg-white text-blue-600">
                        <Upload className="h-4 w-4" />
                      </div>
                      <div className="text-left">
                        <div className="font-medium text-sm">Build from Dockerfile</div>
                        <div className="text-xs text-muted-foreground">Upload and build container image</div>
                      </div>
                    </div>
                  </Button>
                </DialogTrigger>
                <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
                  <DialogHeader>
                    <DialogTitle>Build Docker Image</DialogTitle>
                    <DialogDescription>
                      Upload your Dockerfile and configure build options to create a new container image.
                    </DialogDescription>
                  </DialogHeader>
                  
                  <div className="grid gap-4 py-4">
                    {/* Image Name */}
                    <div className="grid gap-2">
                      <Label htmlFor="imageName">Image Name *</Label>
                      <Input
                        id="imageName"
                        placeholder="my-app"
                        value={imageName}
                        onChange={(e) => setImageName(e.target.value)}
                      />
                      <p className="text-xs text-muted-foreground">
                        Name for your container image (e.g., my-app, nginx-custom)
                      </p>
                    </div>

                    {/* Image Tag */}
                    <div className="grid gap-2">
                      <Label htmlFor="imageTag">Tag</Label>
                      <Input
                        id="imageTag"
                        placeholder="latest"
                        value={imageTag}
                        onChange={(e) => setImageTag(e.target.value)}
                      />
                      <p className="text-xs text-muted-foreground">
                        Version tag for the image (default: latest)
                      </p>
                    </div>

                    {/* Dockerfile Content */}
                    <div className="grid gap-2">
                      <Label htmlFor="dockerfile">Dockerfile Content *</Label>
                      <Textarea
                        id="dockerfile"
                        placeholder={`FROM node:18-alpine\nWORKDIR /app\nCOPY package*.json ./\nRUN npm install\nCOPY . .\nEXPOSE 3000\nCMD ["npm", "start"]`}
                        className="min-h-[200px] font-mono text-sm"
                        value={dockerfileContent}
                        onChange={(e) => setDockerfileContent(e.target.value)}
                      />
                      <p className="text-xs text-muted-foreground">
                        Paste your Dockerfile content here
                      </p>
                    </div>

                    {/* Build Context */}
                    <div className="grid gap-2">
                      <Label htmlFor="buildContext">Build Context Path</Label>
                      <Input
                        id="buildContext"
                        placeholder="."
                        value={buildContext}
                        onChange={(e) => setBuildContext(e.target.value)}
                      />
                      <p className="text-xs text-muted-foreground">
                        Path to build context (default: current directory)
                      </p>
                    </div>

                    {/* Platform Selection */}
                    <div className="grid gap-2">
                      <Label htmlFor="platform">Target Platform</Label>
                      <Select value={platform} onValueChange={setPlatform}>
                        <SelectTrigger id="platform">
                          <SelectValue placeholder="Select platform" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="linux/amd64">Linux AMD64</SelectItem>
                          <SelectItem value="linux/arm64">Linux ARM64</SelectItem>
                          <SelectItem value="linux/arm/v7">Linux ARM v7</SelectItem>
                          <SelectItem value="linux/386">Linux 386</SelectItem>
                        </SelectContent>
                      </Select>
                      <p className="text-xs text-muted-foreground">
                        Target architecture for the image
                      </p>
                    </div>

                    {/* No Cache Option */}
                    <div className="flex items-center space-x-2">
                      <input
                        type="checkbox"
                        id="nocache"
                        checked={nocache}
                        onChange={(e) => setNocache(e.target.checked)}
                        className="h-4 w-4 rounded border-gray-300"
                      />
                      <Label htmlFor="nocache" className="text-sm font-normal cursor-pointer">
                        Build without cache (--no-cache)
                      </Label>
                    </div>
                  </div>

                  <DialogFooter>
                    <Button
                      variant="outline"
                      onClick={() => setIsDialogOpen(false)}
                      disabled={isBuilding}
                    >
                      Cancel
                    </Button>
                    <Button
                      onClick={handleBuildImage}
                      disabled={isBuilding || !dockerfileContent || !imageName}
                    >
                      {isBuilding ? (
                        <>
                          <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                          Building...
                        </>
                      ) : (
                        <>
                          <Package className="mr-2 h-4 w-4" />
                          Build Image
                        </>
                      )}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>

              <Button variant="ghost" className="w-full justify-start h-auto p-4 bg-green-50 hover:bg-green-100">
                <div className="flex items-center space-x-3">
                  <div className="p-2 rounded-lg bg-white text-green-600">
                    <Download className="h-4 w-4" />
                  </div>
                  <div className="text-left">
                    <div className="font-medium text-sm">Pull Image</div>
                    <div className="text-xs text-muted-foreground">Download from registry</div>
                  </div>
                </div>
              </Button>

              <Button variant="ghost" className="w-full justify-start h-auto p-4 bg-purple-50 hover:bg-purple-100">
                <div className="flex items-center space-x-3">
                  <div className="p-2 rounded-lg bg-white text-purple-600">
                    <Container className="h-4 w-4" />
                  </div>
                  <div className="text-left">
                    <div className="font-medium text-sm">Deploy Container</div>
                    <div className="text-xs text-muted-foreground">Create new instance</div>
                  </div>
                </div>
              </Button>

              <Button variant="ghost" className="w-full justify-start h-auto p-4 bg-orange-50 hover:bg-orange-100">
                <div className="flex items-center space-x-3">
                  <div className="p-2 rounded-lg bg-white text-orange-600">
                    <Layers className="h-4 w-4" />
                  </div>
                  <div className="text-left">
                    <div className="font-medium text-sm">View Layers</div>
                    <div className="text-xs text-muted-foreground">Inspect image layers</div>
                  </div>
                </div>
              </Button>
            </CardContent>
          </Card>

          {/* Registry Info Card */}
          <Card className="mt-6">
            <CardHeader>
              <CardTitle>Registry Information</CardTitle>
              <CardDescription>Connection details</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div>
                <div className="text-xs font-medium text-muted-foreground mb-1">Registry URL</div>
                <div className="flex items-center justify-between p-2 bg-muted rounded text-sm font-mono">
                  <span className="truncate">{REGISTRY_URL}</span>
                  <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => copyToClipboard(REGISTRY_URL)}>
                    <Copy className="h-3 w-3" />
                  </Button>
                </div>
              </div>
              <div>
                <div className="text-xs font-medium text-muted-foreground mb-1">Pull Command</div>
                <div className="flex items-center justify-between p-2 bg-muted rounded text-sm font-mono">
                  <span className="truncate">docker pull {REGISTRY_URL}/image</span>
                  <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => copyToClipboard(`docker pull ${REGISTRY_URL}/image`)}>
                    <Copy className="h-3 w-3" />
                  </Button>
                </div>
              </div>
              <div>
                <div className="text-xs font-medium text-muted-foreground mb-1">Push Command</div>
                <div className="flex items-center justify-between p-2 bg-muted rounded text-sm font-mono">
                  <span className="truncate">docker push {REGISTRY_URL}/image</span>
                  <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => copyToClipboard(`docker push ${REGISTRY_URL}/image`)}>
                    <Copy className="h-3 w-3" />
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </DashboardShell>
  )
}
