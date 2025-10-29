'use client'

import { useEffect, useState } from "react"
import axios from "axios"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { DashboardHeader } from "@/components/dashboard-header"
import { DashboardShell } from "@/components/dashboard-shell"
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
  Layers
} from "lucide-react"

type Image = {
  Id: string
  Names: string[]
  Image: string
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
    fetchImages()
  }, [])

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
    c.Image?.toLowerCase().includes(searchTerm.toLowerCase()) ||
    c.Id?.toLowerCase().includes(searchTerm.toLowerCase())
  )

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text)
    } catch (err) {
      console.error("Failed to copy to clipboard:", err)
    }
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
                              {c.Names?.[0]?.replace(/^\//, "") || "Unnamed"}
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
                              ID: {c.Id.slice(0, 12)}
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
              <Button variant="ghost" className="w-full justify-start h-auto p-4 bg-blue-50 hover:bg-blue-100">
                <div className="flex items-center space-x-3">
                  <div className="p-2 rounded-lg bg-white text-blue-600">
                    <Upload className="h-4 w-4" />
                  </div>
                  <div className="text-left">
                    <div className="font-medium text-sm">Push Image</div>
                    <div className="text-xs text-muted-foreground">Upload container image</div>
                  </div>
                </div>
              </Button>

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
