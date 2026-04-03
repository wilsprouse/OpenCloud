'use client'

import { Suspense, useEffect, useRef, useState } from "react"
import { useRouter } from "next/navigation"
import { useSearchParams } from "next/navigation"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
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
import { toast } from "sonner"
import client from "@/app/utility/post"
import { FUNCTION_NAME_MAX_LENGTH, isValidFunctionName } from "@/lib/function-name"
import { useFunctionNameWarning } from "@/lib/use-function-name-warning"
import { 
  Container, 
  RefreshCw, 
  Download, 
  Upload, 
  Search,
  Square,
  Trash2,
  Play,
  Package,
  HardDrive,
  Layers,
  Copy,
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

function SearchParamsReader({ onCreateRequested }: { onCreateRequested: () => void }) {
  const searchParams = useSearchParams()
  useEffect(() => {
    if (searchParams.get("create") === "true") {
      onCreateRequested()
    }
  }, [searchParams, onCreateRequested])
  return null
}

export default function ContainerRegistry() {
  const router = useRouter()
  const [images, setImages] = useState<Image[]>([])
  const [loading, setLoading] = useState(false)
  const [searchTerm, setSearchTerm] = useState("")
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [isBuilding, setIsBuilding] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [imageToDelete, setImageToDelete] = useState<Image | null>(null)
  const [pendingCreate, setPendingCreate] = useState(false)
  
  // Service enabled state
  const [serviceEnabled, setServiceEnabled] = useState<boolean | null>(null)
  const [enablingService, setEnablingService] = useState(false)
  const [enableOutput, setEnableOutput] = useState<string[]>([])
  const [enableError, setEnableError] = useState<string | null>(null)
  const outputBoxRef = useRef<HTMLDivElement>(null)
  
  // Dockerfile form state
  const [dockerfileContent, setDockerfileContent] = useState("")
  const [imageName, setImageName] = useState("")
  const isImageNameValid = imageName.length > 0 && isValidFunctionName(imageName)
  const {
    handleBeforeInput: handleImageNameBeforeInput,
    handleChange: handleImageNameChange,
    handlePaste: handleImageNamePaste,
    resetWarning: resetImageNameWarning,
  } = useFunctionNameWarning(setImageName)
  const [imageTag, setImageTag] = useState("latest")
  const [nocache, setNocache] = useState(false)

  // Pull image dialog state
  const [isPullDialogOpen, setIsPullDialogOpen] = useState(false)
  const [pullImageName, setPullImageName] = useState("")
  const [pullRegistry, setPullRegistry] = useState<"docker.io" | "quay.io">("docker.io")
  const [isPulling, setIsPulling] = useState(false)
  const [pullOutput, setPullOutput] = useState<string[]>([])
  const [pullError, setPullError] = useState<string | null>(null)
  const pullOutputEndRef = useRef<HTMLDivElement>(null)

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

  // Auto-scroll the output box whenever a new line is appended
  useEffect(() => {
    if (outputBoxRef.current) {
      outputBoxRef.current.scrollTop = outputBoxRef.current.scrollHeight
    }
  }, [enableOutput])

  // Auto-scroll pull output box when new lines arrive
  useEffect(() => {
    if (pullOutputEndRef.current) {
      pullOutputEndRef.current.scrollIntoView({ behavior: "smooth" })
    }
  }, [pullOutput])

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
        body: JSON.stringify({ service: "container_registry" }),
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
        fetchImages()
      }
    } catch (err) {
      console.error("Failed to enable service:", err)
      setEnableError(err instanceof Error ? err.message : "Failed to enable service")
    } finally {
      setEnablingService(false)
    }
  }

  // Fetch containers
  const fetchImages = async () => {
    setLoading(true)
    try {
      const res = await client.get<Image[]>("/get-images")
      setImages(Array.isArray(res.data) ? res.data : [])
    } catch (err) {
      console.error("Failed to fetch containers:", err)
      setImages([])
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

  useEffect(() => {
    if (serviceEnabled && pendingCreate) {
      setIsDialogOpen(true)
    }
  }, [serviceEnabled, pendingCreate])

  // Open the delete confirmation dialog for the selected image
  const openDeleteDialog = (image: Image) => {
    setImageToDelete(image)
    setIsDeleteDialogOpen(true)
  }

  // Confirm deletion: POST to /delete-image with the image name
  const handleDeleteImage = async () => {
    if (!imageToDelete) return
    try {
      await client.post("/delete-image", { imageName: imageToDelete.Image })
      fetchImages()
      setIsDeleteDialogOpen(false)
      setImageToDelete(null)
    } catch (err: unknown) {
      const axiosErr = err as { response?: { status?: number; data?: unknown } }
      if (axiosErr?.response?.status === 409) {
        const rawData = axiosErr.response.data
        const message = typeof rawData === "string" && rawData.trim()
          ? rawData.trim()
          : "This image is in use by a container. Remove the container before deleting the image."
        toast.error(message)
      } else {
        toast.error("Failed to delete image. Please try again.")
        console.error("Failed to delete image:", err)
      }
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
      toast.error("Please provide both a Dockerfile and an image name")
      return
    }

    if (!isImageNameValid) {
      toast.error("Image name cannot contain spaces and must be 50 characters or fewer")
      return
    }

    setIsBuilding(true)
    try {
      // Send the dockerfile and build parameters to the backend
      await client.post("/build-image", {
        dockerfile: dockerfileContent,
        imageName: `${imageName}:${imageTag}`,
        context: ".",
        nocache: nocache,
        platform: "linux/amd64",
      })
      
      // Reset form and close dialog
      setDockerfileContent("")
      setImageName("")
      resetImageNameWarning()
      setImageTag("latest")
      setNocache(false)
      setIsDialogOpen(false)
      
      // Refresh the image list
      await fetchImages()
      
      toast.success("Image built successfully!")
    } catch (err) {
      console.error("Failed to build image:", err)
      toast.error("Failed to build image. Please check the logs.")
    } finally {
      setIsBuilding(false)
    }
  }

  // Closes the pull image dialog and resets all associated state.
  const handleClosePullDialog = () => {
    setIsPullDialogOpen(false)
    setPullOutput([])
    setPullError(null)
  }

  // Pull an image from a public registry, streaming real-time progress updates.
  const handlePullImage = async () => {
    const trimmed = pullImageName.trim()
    if (!trimmed) {
      toast.error("Please provide an image name")
      return
    }

    setIsPulling(true)
    setPullOutput([])
    setPullError(null)

    const appendLine = (line: string) => {
      setPullOutput(prev => [...prev, line])
    }

    try {
      const response = await fetch("/api/pull-image-stream", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ imageName: trimmed, registry: pullRegistry }),
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
            // Error message will arrive on the next data: line
            errorMsg = "Pull failed"
          }
        }
      }

      if (succeeded) {
        await fetchImages()
        toast.success(`Image "${trimmed}" pulled successfully!`)
        // Keep dialog open briefly so the user can see the completed output,
        // then close it after a short delay.
        setTimeout(() => {
          setIsPullDialogOpen(false)
          setPullImageName("")
          setPullRegistry("docker.io")
          setPullOutput([])
        }, 1500)
      } else {
        // The error message may have been appended to pullOutput already.
        setPullError(errorMsg || "Failed to pull image. Please check the image name and try again.")
      }
    } catch (err) {
      console.error("Failed to pull image:", err)
      const msg = err instanceof Error ? err.message : "Failed to pull image. Please check the image name and try again."
      setPullError(msg)
    } finally {
      setIsPulling(false)
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
        <div className="flex flex-col items-center gap-6 pt-8">
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
                {enablingService ? (
                  <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <Power className="mr-2 h-4 w-4" />
                )}
                {enablingService ? "Enabling..." : "Enable Container Registry"}
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
        <SearchParamsReader onCreateRequested={() => setPendingCreate(true)} />
      </Suspense>
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
                {filteredImages.map((c) => (
                    <div
                      key={`${c.Id}-${c.Image}`}
                      className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
                    >
                      <div className="flex items-center space-x-4 flex-1">
                        <div className="p-2 rounded-lg bg-muted">
                          <Container className="h-5 w-5 text-muted-foreground" />
                        </div>
                        <div className="space-y-1 flex-1 min-w-0">
                          <h4 className="font-medium text-foreground truncate">
                            {c.RepoTags?.[0]?.replace(/^\//, "") || "Unnamed"}
                          </h4>
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
                          onClick={() => {
                            const imageName = c.RepoTags?.[0]?.replace(/^\//, "") || c.Image
                            router.push(`/compute/containers?create=true&image=${encodeURIComponent(imageName)}`)
                          }}
                          title="Deploy container"
                        >
                          <Play className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="destructive"
                          size="sm"
                          onClick={() => openDeleteDialog(c)}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </div>
                  ))}
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
              <Dialog open={isDialogOpen} onOpenChange={(open) => {
                setIsDialogOpen(open)
                if (!open) resetImageNameWarning()
              }}>
                <DialogTrigger asChild>
                  <Button variant="ghost" className="w-full justify-start h-auto p-4 bg-blue-50 hover:bg-blue-100 dark:bg-blue-950/40 dark:hover:bg-blue-900/50">
                    <div className="flex items-center space-x-3">
                      <div className="p-2 rounded-lg bg-white dark:bg-white/10 text-blue-600 dark:text-blue-400 shrink-0">
                        <Upload className="h-4 w-4" />
                      </div>
                      <div className="text-left min-w-0">
                        <div className="font-semibold text-sm text-gray-900 dark:text-white">Build from Dockerfile</div>
                        <div className="text-xs text-muted-foreground whitespace-normal">Upload and build container image</div>
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
                        onChange={(e) => handleImageNameChange(e.target.value)}
                        onBeforeInput={handleImageNameBeforeInput}
                        onPaste={handleImageNamePaste}
                        maxLength={FUNCTION_NAME_MAX_LENGTH}
                      />
                      <p className="text-xs text-muted-foreground">
                        Name for your container image (e.g., my-app, nginx-custom). Cannot contain spaces and must be 50 characters or fewer.
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
                      disabled={isBuilding || !dockerfileContent || !imageName || !isImageNameValid}
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

              <Dialog open={isPullDialogOpen} onOpenChange={(open) => { if (!open) handleClosePullDialog(); else setIsPullDialogOpen(true) }}>
                <DialogTrigger asChild>
                  <Button variant="ghost" className="w-full justify-start h-auto p-4 bg-green-50 hover:bg-green-100 dark:bg-green-950/40 dark:hover:bg-green-900/50">
                    <div className="flex items-center space-x-3">
                      <div className="p-2 rounded-lg bg-white dark:bg-white/10 text-green-600 dark:text-green-400 shrink-0">
                        <Download className="h-4 w-4" />
                      </div>
                      <div className="text-left min-w-0">
                        <div className="font-semibold text-sm text-gray-900 dark:text-white">Pull Image</div>
                        <div className="text-xs text-muted-foreground whitespace-normal">Download from registry</div>
                      </div>
                    </div>
                  </Button>
                </DialogTrigger>
                <DialogContent className="max-h-[90vh] overflow-y-auto">
                  <DialogHeader>
                    <DialogTitle>Pull Container Image</DialogTitle>
                    <DialogDescription>
                      Download a container image from a public registry.
                    </DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4 py-2">
                    <div className="space-y-2">
                      <Label htmlFor="pull-image-name">Image Name</Label>
                      <Input
                        id="pull-image-name"
                        placeholder="e.g. nginx:latest or library/alpine:3.18"
                        value={pullImageName}
                        onChange={(e) => setPullImageName(e.target.value)}
                        onKeyDown={(e) => { if (e.key === "Enter") handlePullImage() }}
                        disabled={isPulling}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label>Source Registry</Label>
                      <div className="flex flex-col gap-2">
                        <label className="flex items-center gap-2 cursor-pointer">
                          <input
                            type="radio"
                            name="pullRegistry"
                            checked={pullRegistry === "docker.io"}
                            onChange={() => setPullRegistry("docker.io")}
                            className="h-4 w-4 border-gray-300"
                            disabled={isPulling}
                          />
                          <span className="text-sm">Docker Hub (docker.io)</span>
                        </label>
                        <label className="flex items-center gap-2 cursor-pointer">
                          <input
                            type="radio"
                            name="pullRegistry"
                            checked={pullRegistry === "quay.io"}
                            onChange={() => setPullRegistry("quay.io")}
                            className="h-4 w-4 border-gray-300"
                            disabled={isPulling}
                          />
                          <span className="text-sm">Quay.io (quay.io)</span>
                        </label>
                      </div>
                    </div>

                    {/* Streaming progress output */}
                    {(isPulling || pullOutput.length > 0 || pullError) && (
                      <div className="space-y-1">
                        <Label>Pull Progress</Label>
                        <div className="max-h-32 overflow-y-auto overflow-x-hidden rounded-md border bg-black p-3 font-mono text-xs text-green-400">
                          {pullOutput.map((line, i) => (
                            <div key={i} className="break-all whitespace-pre-wrap">{line}</div>
                          ))}
                          {pullError && (
                            <div className="text-red-400 break-all whitespace-pre-wrap">{pullError}</div>
                          )}
                          {isPulling && (
                            <div className="animate-pulse">▌</div>
                          )}
                          <div ref={pullOutputEndRef} />
                        </div>
                      </div>
                    )}
                  </div>
                  <DialogFooter>
                    <Button variant="outline" onClick={handleClosePullDialog} disabled={isPulling}>
                      Cancel
                    </Button>
                    <Button onClick={handlePullImage} disabled={isPulling || !pullImageName.trim()}>
                      {isPulling ? (
                        <>
                          <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                          Pulling...
                        </>
                      ) : (
                        <>
                          <Download className="mr-2 h-4 w-4" />
                          Pull Image
                        </>
                      )}
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
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

      {/* Delete Image Confirmation Dialog */}
      <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Container Image</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete{" "}
              <span className="font-semibold">
                {imageToDelete?.RepoTags?.[0]?.replace(/^\//, "") || imageToDelete?.Image || "this image"}
              </span>
              ? This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setIsDeleteDialogOpen(false)}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleDeleteImage}
            >
              <Trash2 className="mr-2 h-4 w-4" />
              Delete Image
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </DashboardShell>
  )
}
