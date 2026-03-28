'use client'

import { use, useEffect, useState } from "react"
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
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import client from "@/app/utility/post"
import { Progress } from "@/components/ui/progress"
import { CONTAINER_NAME_MAX_LENGTH, isValidContainerName } from "@/lib/container-name"
import { useContainerNameWarning } from "@/lib/use-container-name-warning"
import { 
  Upload, 
  RefreshCw, 
  Download, 
  Search,
  Trash2,
  File,
  HardDrive,
  FileText,
  ArrowLeft,
  Package,
  Pencil,
} from "lucide-react"

type Blob = {
  id: string
  name: string
  size: number
  contentType: string
  lastModified: string
  container: string
}

export default function ContainerDetail({ params }: { params: Promise<{ container: string }> }) {
  const resolvedParams = use(params)
  const containerName = decodeURIComponent(resolvedParams.container)
  const router = useRouter()
  const [blobs, setBlobs] = useState<Blob[]>([])
  const [loading, setLoading] = useState(false)
  const [searchTerm, setSearchTerm] = useState("")
  const [isUploadDialogOpen, setIsUploadDialogOpen] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [blobToDelete, setBlobToDelete] = useState<{ container: string; name: string } | null>(null)

  // Rename dialog state
  const [isRenameDialogOpen, setIsRenameDialogOpen] = useState(false)
  const [newContainerName, setNewContainerName] = useState<string>("")
  const [isRenaming, setIsRenaming] = useState(false)
  const [renameError, setRenameError] = useState<string | null>(null)
  const isNewContainerNameValid = isValidContainerName(newContainerName)

  const {
    handleBeforeInput: handleNewNameBeforeInput,
    handleChange: handleNewNameChange,
    handlePaste: handleNewNamePaste,
    resetWarning: resetNewNameWarning,
  } = useContainerNameWarning(setNewContainerName)

  // Upload form state
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [isUploading, setIsUploading] = useState(false)
  const [uploadProgress, setUploadProgress] = useState(0)
  const [uploadError, setUploadError] = useState<string | null>(null)

  // Fetch blobs for this container
  const fetchBlobs = async () => {
    setLoading(true)
    try {
      const res = await client.get<Blob[]>(`/get-blobs?container=${encodeURIComponent(containerName)}`)
      setBlobs(res.data || [])
    } catch (err) {
      console.error("Failed to fetch blobs:", err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchBlobs()
  }, [containerName])

  const handleDownload = async (container: string, name: string) => {
    try {
      console.log(`Downloading blob: ${name} from container: ${container}`);

      // Send POST request with JSON body
      const res = await client.post("/download-object", { container, name }, {
        responseType: "blob", // important for file download
      });

      // Create a download link
      const url = window.URL.createObjectURL(new Blob([res.data]));
      const link = document.createElement("a");
      link.href = url;
      link.setAttribute("download", name); // use the blob's original filename
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url); // cleanup
    } catch (err) {
      console.error("Failed to download blob:", err);
    }
  };

  // Open the delete confirmation dialog for the selected blob
  const openDeleteDialog = (container: string, name: string) => {
    setBlobToDelete({ container, name })
    setIsDeleteDialogOpen(true)
  }

  const closeDeleteDialog = () => {
    setBlobToDelete(null)
    setIsDeleteDialogOpen(false)
  }

  const handleDelete = async () => {
    if (!blobToDelete) return

    try {
      const res = await client.delete("/delete-object", {
        data: { container: blobToDelete.container, name: blobToDelete.name },
      })

      if (res.status === 200) {
        closeDeleteDialog()
        fetchBlobs() // Refresh blob list after deletion
      } else {
        console.error("Failed to delete blob:", res.statusText)
      }
    } catch (err) {
      console.error("Failed to delete blob:", err)
    }
  }

  const closeUploadDialog = () => {
    setIsUploadDialogOpen(false)
    setUploadError(null)
    setSelectedFile(null)
  }

  const handleUpload = async () => {
    if (!selectedFile) {
      console.warn("No file selected")
      return
    }

    console.log(`Uploading file: ${selectedFile.name} to container: ${containerName}`)

    // Create FormData for multipart/form-data upload
    const formData = new FormData()
    formData.append("file", selectedFile)
    formData.append("container", containerName)

    setIsUploading(true)
    setUploadProgress(0)
    setUploadError(null)

    try {
      // POST to backend endpoint with upload progress tracking.
      // Do NOT set Content-Type manually — the browser must set it so the
      // required multipart boundary is included automatically.
      const res = await client.post("/upload-object", formData, {
        onUploadProgress: (progressEvent) => {
          if (progressEvent.total) {
            const percent = Math.round((progressEvent.loaded * 100) / progressEvent.total)
            setUploadProgress(percent)
          }
        },
      })

      if (res.status === 200 || res.status === 201) {
        console.log("Upload successful:", res.data)

        // Refresh blob list before closing so file appears immediately
        await fetchBlobs()

        // Reset form & close dialog
        closeUploadDialog()
      } else {
        setUploadError(`Upload failed (${res.status}). Please try again.`)
        console.error("Upload failed:", res.status, res.statusText)
      }
    } catch (err) {
      setUploadError("Upload failed. Please check your connection and try again.")
      console.error("Failed to upload blob:", err)
    } finally {
      setIsUploading(false)
      setUploadProgress(0)
    }
  }

  const openRenameDialog = () => {
    setNewContainerName(containerName)
    resetNewNameWarning()
    setRenameError(null)
    setIsRenameDialogOpen(true)
  }

  const closeRenameDialog = () => {
    setNewContainerName("")
    resetNewNameWarning()
    setRenameError(null)
    setIsRenameDialogOpen(false)
  }

  const handleRename = async () => {
    if (!isNewContainerNameValid || newContainerName === containerName) return

    setIsRenaming(true)
    setRenameError(null)
    try {
      const res = await client.put("/rename-container", {
        currentName: containerName,
        newName: newContainerName,
      })

      if (res.status === 200) {
        closeRenameDialog()
        router.push(`/storage/blob/${encodeURIComponent(newContainerName)}`)
      } else {
        setRenameError("Failed to rename container. Please try again.")
      }
    } catch (err: unknown) {
      const message =
        err instanceof Error ? err.message : "Failed to rename container. Please try again."
      setRenameError(message)
      console.error("Failed to rename container:", err)
    } finally {
      setIsRenaming(false)
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
    blob.contentType.toLowerCase().includes(searchTerm.toLowerCase())
  )

  // Calculate statistics
  const totalBlobs = blobs.length
  const totalSize = blobs.reduce((sum, blob) => sum + blob.size, 0)

  return (
    <DashboardShell>
      <DashboardHeader 
        heading={
          <div className="flex items-center space-x-2">
            <Button 
              variant="ghost" 
              size="icon"
              onClick={() => router.push('/storage/blob')}
              className="mr-2"
            >
              <ArrowLeft className="h-4 w-4" />
            </Button>
            <span>{containerName}</span>
            <Button
              variant="ghost"
              size="icon"
              onClick={openRenameDialog}
              title="Rename container"
              className="text-muted-foreground hover:text-foreground"
            >
              <Pencil className="h-4 w-4" />
            </Button>
          </div>
        } 
        text="Manage objects in this container"
      >
        <div className="flex items-center space-x-2">
          <Button variant="outline" onClick={fetchBlobs} disabled={loading}>
            <RefreshCw className={`mr-2 h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
          <Dialog open={isUploadDialogOpen} onOpenChange={(open) => {
            if (!isUploading) {
              if (!open) closeUploadDialog()
              else setIsUploadDialogOpen(true)
            }
          }}>
            <DialogTrigger asChild>
              <Button>
                <Upload className="mr-2 h-4 w-4" />
                Upload Object
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Upload Object</DialogTitle>
                <DialogDescription>
                  Select a file to upload to <strong>{containerName}</strong>.
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="file">File</Label>
                  <Input
                    id="file"
                    type="file"
                    disabled={isUploading}
                    onChange={(e) => {
                      setSelectedFile(e.target.files?.[0] || null)
                      setUploadError(null)
                    }}
                  />
                </div>
                {uploadError && (
                  <p className="text-sm text-destructive">{uploadError}</p>
                )}
                {isUploading && (
                  <div className="space-y-2">
                    <div className="flex justify-between text-sm text-muted-foreground">
                      <span>Uploading...</span>
                      <span>{uploadProgress}%</span>
                    </div>
                    <Progress value={uploadProgress} className="h-2" />
                  </div>
                )}
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={closeUploadDialog} disabled={isUploading}>
                  Cancel
                </Button>
                <Button onClick={handleUpload} disabled={!selectedFile || isUploading}>
                  {isUploading ? (
                    <>
                      <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                      Uploading...
                    </>
                  ) : (
                    "Upload"
                  )}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </DashboardHeader>

      {/* Statistics Cards */}
      <div className="grid gap-6 md:grid-cols-2">
        <Card className="border-l-4 border-l-blue-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Objects</CardTitle>
            <File className="h-4 w-4 text-blue-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalBlobs}</div>
            <p className="text-xs text-muted-foreground">Files in this container</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-purple-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Storage Used</CardTitle>
            <HardDrive className="h-4 w-4 text-purple-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{formatSize(totalSize)}</div>
            <p className="text-xs text-muted-foreground">Total object size</p>
          </CardContent>
        </Card>
      </div>

      {/* Main Blob List */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Objects</CardTitle>
              <CardDescription>View and manage objects in {containerName}</CardDescription>
            </div>
          </div>
          <div className="relative mt-4">
            <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
            <input
              type="text"
              placeholder="Search objects by name or content type..."
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
                    </div>
                    <p className="text-sm text-muted-foreground truncate">{blob.contentType || 'unknown'}</p>
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
                    onClick={() => handleDownload(blob.container, blob.name)}
                    title="Download"
                  >
                    <Download className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => openDeleteDialog(blob.container, blob.name)}
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
                <h3 className="mt-4 text-lg font-semibold">No objects found</h3>
                <p className="mt-2 text-sm text-muted-foreground">
                  {searchTerm ? "Try adjusting your search terms" : "Upload your first object to get started"}
                </p>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Delete Blob Confirmation Dialog */}
      <Dialog open={isDeleteDialogOpen} onOpenChange={(open) => { if (!open) closeDeleteDialog() }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Object</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete{" "}
              <strong>{blobToDelete?.name}</strong>? This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={closeDeleteDialog}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleDelete}>
              Delete Object
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Rename Container Dialog */}
      <Dialog open={isRenameDialogOpen} onOpenChange={(open) => { if (!open) closeRenameDialog() }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Rename Container</DialogTitle>
            <DialogDescription>
              Enter a new name for <strong>{containerName}</strong>.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="new-container-name">New Container Name</Label>
              <Input
                id="new-container-name"
                placeholder="new-container-name"
                value={newContainerName}
                onChange={(e) => handleNewNameChange(e.target.value)}
                onBeforeInput={handleNewNameBeforeInput}
                onPaste={handleNewNamePaste}
                maxLength={CONTAINER_NAME_MAX_LENGTH}
              />
              <p className="text-xs text-muted-foreground">
                Container names cannot contain spaces and must be 50 characters or fewer.
              </p>
              {renameError && (
                <p className="text-xs text-destructive">{renameError}</p>
              )}
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={closeRenameDialog} disabled={isRenaming}>
              Cancel
            </Button>
            <Button
              onClick={handleRename}
              disabled={!isNewContainerNameValid || newContainerName === containerName || isRenaming}
            >
              {isRenaming ? "Renaming..." : "Rename"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </DashboardShell>
  )
}
