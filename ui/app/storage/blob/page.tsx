'use client'

import { useEffect, useState } from "react"
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
import client from "@/app/utility/post"
import { 
  Upload, 
  RefreshCw, 
  Download, 
  Search,
  Trash2,
  FolderPlus,
  File,
  HardDrive,
  Package,
  FileText
} from "lucide-react"

type Blob = {
  id: string
  name: string
  size: number
  contentType: string
  lastModified: string
  container: string
}

export default function BlobStorage() {
  const [blobs, setBlobs] = useState<Blob[]>([])
  const [loading, setLoading] = useState(false)
  const [searchTerm, setSearchTerm] = useState("")
  const [isUploadDialogOpen, setIsUploadDialogOpen] = useState(false)
  const [isContainerDialogOpen, setIsContainerDialogOpen] = useState(false)
  
  // Upload form state
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [uploadContainer, setUploadContainer] = useState("")
  
  // Container form state
  const [containerName, setContainerName] = useState("")

  // Fetch blobs
  const fetchBlobs = async () => {
    setLoading(true)
    try {
      const res = await client.get<Blob[]>("/get-blobs")
      setBlobs(res.data)
    } catch (err) {
      console.error("Failed to fetch blobs:", err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchBlobs()
  }, [])

  // Handle blob actions
  const handleDownload = async (id: string, name: string) => {
    try {
      console.log(`Downloading blob: ${name}`)
      // Backend implementation will handle actual download
    } catch (err) {
      console.error("Failed to download blob:", err)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      console.log(`Deleting blob: ${id}`)
      // Backend implementation will handle actual delete
      fetchBlobs() // refresh list
    } catch (err) {
      console.error("Failed to delete blob:", err)
    }
  }

  const handleUpload = async () => {
    try {
      if (!selectedFile) {
        console.log("No file selected")
        return
      }
      console.log(`Uploading file: ${selectedFile.name} to container: ${uploadContainer}`)
      // Backend implementation will handle file upload
      setIsUploadDialogOpen(false)
      setSelectedFile(null)
      setUploadContainer("")
      fetchBlobs()
    } catch (err) {
      console.error("Failed to upload blob:", err)
    }
  }

  const handleCreateContainer = async () => {
    try {
      console.log(`Creating container: ${containerName}`)
      // Backend implementation will handle container creation
      setIsContainerDialogOpen(false)
      setContainerName("")
    } catch (err) {
      console.error("Failed to create container:", err)
    }
  }

  // Format file size
  const formatSize = (bytes: number): string => {
    if (bytes === 0) return "0 B"
    const k = 1024
    const sizes = ["B", "KB", "MB", "GB", "TB"]
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i]
  }

  // Format date
  const formatDate = (dateString: string): string => {
    try {
      const date = new Date(dateString)
      return date.toLocaleString()
    } catch {
      return dateString
    }
  }

  // Filter blobs based on search
  const filteredBlobs = blobs.filter(blob => 
    blob.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    blob.container.toLowerCase().includes(searchTerm.toLowerCase()) ||
    blob.contentType.toLowerCase().includes(searchTerm.toLowerCase())
  )

  // Calculate statistics
  const totalBlobs = blobs.length
  const totalSize = blobs.reduce((sum, blob) => sum + blob.size, 0)
  const containers = [...new Set(blobs.map(blob => blob.container))]
  const uniqueContainers = containers.length

  return (
    <DashboardShell>
      <DashboardHeader heading="Blob Storage" text="Manage your files and objects in cloud storage">
        <div className="flex items-center space-x-2">
          <Button variant="outline" onClick={fetchBlobs} disabled={loading}>
            <RefreshCw className={`mr-2 h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
          <Dialog open={isContainerDialogOpen} onOpenChange={setIsContainerDialogOpen}>
            <DialogTrigger asChild>
              <Button variant="outline">
                <FolderPlus className="mr-2 h-4 w-4" />
                Create Container
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create New Container</DialogTitle>
                <DialogDescription>
                  Containers help organize your blobs. Enter a name for your new container.
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="container-name">Container Name</Label>
                  <Input
                    id="container-name"
                    placeholder="my-container"
                    value={containerName}
                    onChange={(e) => setContainerName(e.target.value)}
                  />
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setIsContainerDialogOpen(false)}>
                  Cancel
                </Button>
                <Button onClick={handleCreateContainer} disabled={!containerName}>
                  Create
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
          <Dialog open={isUploadDialogOpen} onOpenChange={setIsUploadDialogOpen}>
            <DialogTrigger asChild>
              <Button>
                <Upload className="mr-2 h-4 w-4" />
                Upload Blob
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Upload Blob</DialogTitle>
                <DialogDescription>
                  Select a file and choose a container to upload your blob.
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="file">File</Label>
                  <Input
                    id="file"
                    type="file"
                    onChange={(e) => setSelectedFile(e.target.files?.[0] || null)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="container">Container</Label>
                  <Input
                    id="container"
                    placeholder="Container name"
                    value={uploadContainer}
                    onChange={(e) => setUploadContainer(e.target.value)}
                  />
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setIsUploadDialogOpen(false)}>
                  Cancel
                </Button>
                <Button onClick={handleUpload} disabled={!selectedFile || !uploadContainer}>
                  Upload
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </DashboardHeader>

      {/* Statistics Cards */}
      <div className="grid gap-6 md:grid-cols-3">
        <Card className="border-l-4 border-l-blue-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Blobs</CardTitle>
            <File className="h-4 w-4 text-blue-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalBlobs}</div>
            <p className="text-xs text-muted-foreground">Files stored</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-green-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Containers</CardTitle>
            <Package className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{uniqueContainers}</div>
            <p className="text-xs text-muted-foreground">Unique containers</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-purple-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Storage Used</CardTitle>
            <HardDrive className="h-4 w-4 text-purple-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{formatSize(totalSize)}</div>
            <p className="text-xs text-muted-foreground">Total blob size</p>
          </CardContent>
        </Card>
      </div>

      {/* Main Blob List */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Stored Blobs</CardTitle>
              <CardDescription>View and manage your blob storage files</CardDescription>
            </div>
          </div>
          <div className="relative mt-4">
            <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
            <input
              type="text"
              placeholder="Search blobs by name, container, or content type..."
              className="w-full pl-8 pr-4 py-2 border rounded-md bg-background"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
            />
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {filteredBlobs.map((blob) => (
              <div
                key={blob.id}
                className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
              >
                <div className="flex items-center space-x-4 flex-1">
                  <div className="p-2 rounded-lg bg-blue-50">
                    <FileText className="h-5 w-5 text-blue-600" />
                  </div>
                  <div className="space-y-1 flex-1 min-w-0">
                    <div className="flex items-center space-x-2">
                      <h4 className="font-medium truncate">{blob.name}</h4>
                      <Badge variant="outline">
                        {blob.container}
                      </Badge>
                    </div>
                    <p className="text-sm text-muted-foreground truncate">{blob.contentType}</p>
                    <div className="flex items-center space-x-4 text-xs text-muted-foreground">
                      <span className="flex items-center">
                        <HardDrive className="h-3 w-3 mr-1" />
                        {formatSize(blob.size)}
                      </span>
                      <span>•</span>
                      <span>Modified: {formatDate(blob.lastModified)}</span>
                    </div>
                  </div>
                </div>
                <div className="flex items-center space-x-2 ml-4">
                  <Button 
                    variant="ghost" 
                    size="icon"
                    onClick={() => handleDownload(blob.id, blob.name)}
                    title="Download"
                  >
                    <Download className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => handleDelete(blob.id)}
                    title="Delete"
                  >
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              </div>
            ))}
            {filteredBlobs.length === 0 && !loading && (
              <div className="text-center py-12">
                <Package className="mx-auto h-12 w-12 text-muted-foreground" />
                <h3 className="mt-4 text-lg font-semibold">No blobs found</h3>
                <p className="mt-2 text-sm text-muted-foreground">
                  {searchTerm ? "Try adjusting your search terms" : "Upload your first blob to get started"}
                </p>
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </DashboardShell>
  )
}
