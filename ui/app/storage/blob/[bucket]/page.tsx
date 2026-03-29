'use client'

import { use, useEffect, useRef, useState } from "react"
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
import { BUCKET_NAME_MAX_LENGTH, isValidBucketName } from "@/lib/bucket-name"
import { useBucketNameWarning } from "@/lib/use-bucket-name-warning"
import { Progress } from "@/components/ui/progress"
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
  bucket: string
}

export default function BucketDetail({ params }: { params: Promise<{ bucket: string }> }) {
  const resolvedParams = use(params)
  const bucketName = decodeURIComponent(resolvedParams.bucket)
  const router = useRouter()
  const [blobs, setBlobs] = useState<Blob[]>([])
  const [loading, setLoading] = useState(false)
  const [searchTerm, setSearchTerm] = useState("")
  const [isUploadDialogOpen, setIsUploadDialogOpen] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [blobToDelete, setBlobToDelete] = useState<{ bucket: string; name: string } | null>(null)

  // Rename dialog state
  const [isRenameDialogOpen, setIsRenameDialogOpen] = useState(false)
  const [newBucketName, setNewBucketName] = useState<string>("")
  const [isRenaming, setIsRenaming] = useState(false)
  const [renameError, setRenameError] = useState<string | null>(null)
  const isNewBucketNameValid = isValidBucketName(newBucketName)

  const {
    handleBeforeInput: handleNewNameBeforeInput,
    handleChange: handleNewNameChange,
    handlePaste: handleNewNamePaste,
    resetWarning: resetNewNameWarning,
  } = useBucketNameWarning(setNewBucketName)

  // Upload form state
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [isUploading, setIsUploading] = useState(false)
  const [uploadProgress, setUploadProgress] = useState(0)
  const uploadAbortRef = useRef<AbortController | null>(null)

  // Fetch blobs for this bucket
  const fetchBlobs = async () => {
    setLoading(true)
    try {
      const res = await client.get<Blob[]>(`/get-blobs?bucket=${encodeURIComponent(bucketName)}`)
      setBlobs(res.data || [])
    } catch (err) {
      console.error("Failed to fetch blobs:", err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchBlobs()
  }, [bucketName])

  const handleDownload = async (bucket: string, name: string) => {
    try {
      console.log(`Downloading blob: ${name} from bucket: ${bucket}`);

      // Send POST request with JSON body
      const res = await client.post("/download-object", { bucket, name }, {
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
  const openDeleteDialog = (bucket: string, name: string) => {
    setBlobToDelete({ bucket, name })
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
        data: { bucket: blobToDelete.bucket, name: blobToDelete.name },
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

  const handleUpload = async () => {
    try {
      if (!selectedFile) {
        console.warn("No file selected")
        return
      }

      console.log(`Uploading file: ${selectedFile.name} to bucket: ${bucketName}`)

      // Create FormData for multipart/form-data upload.
      // bucket must be appended before file so the streaming backend
      // receives the bucket name before it begins writing the file to disk.
      const formData = new FormData()
      formData.append("bucket", bucketName)
      formData.append("file", selectedFile)

      const controller = new AbortController()
      uploadAbortRef.current = controller

      setIsUploading(true)
      setUploadProgress(0)

      // POST to backend endpoint
      const res = await client.post("/upload-object", formData, {
        headers: { "Content-Type": "multipart/form-data" },
        signal: controller.signal,
        onUploadProgress: (progressEvent) => {
          if (progressEvent.total) {
            const pct = Math.round((progressEvent.loaded * 100) / progressEvent.total)
            setUploadProgress(pct)
          }
        },
      })

      if (res.status === 200 || res.status === 201) {
        console.log("Upload successful:", res.data)

        // Reset form & close dialog
        setIsUploadDialogOpen(false)
        setSelectedFile(null)

        // Refresh blob list
        fetchBlobs()
      } else {
        console.error("Upload failed:", res.status, res.statusText)
      }
    } catch (err) {
      console.error("Failed to upload blob:", err)
    } finally {
      uploadAbortRef.current = null
      setIsUploading(false)
      setUploadProgress(0)
    }
  }

  const openRenameDialog = () => {
    setNewBucketName(bucketName)
    resetNewNameWarning()
    setRenameError(null)
    setIsRenameDialogOpen(true)
  }

  const closeRenameDialog = () => {
    setNewBucketName("")
    resetNewNameWarning()
    setRenameError(null)
    setIsRenameDialogOpen(false)
  }

  const handleRename = async () => {
    if (!isNewBucketNameValid || newBucketName === bucketName) return

    setIsRenaming(true)
    setRenameError(null)
    try {
      const res = await client.put("/rename-bucket", {
        currentName: bucketName,
        newName: newBucketName,
      })

      if (res.status === 200) {
        closeRenameDialog()
        router.push(`/storage/blob/${encodeURIComponent(newBucketName)}`)
      } else {
        setRenameError("Failed to rename bucket. Please try again.")
      }
    } catch (err: unknown) {
      const message =
        err instanceof Error ? err.message : "Failed to rename bucket. Please try again."
      setRenameError(message)
      console.error("Failed to rename bucket:", err)
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
            <span>{bucketName}</span>
            <Button
              variant="ghost"
              size="icon"
              onClick={openRenameDialog}
              title="Rename bucket"
              className="text-muted-foreground hover:text-foreground"
            >
              <Pencil className="h-4 w-4" />
            </Button>
          </div>
        } 
        text="Manage objects in this bucket"
      >
        <div className="flex items-center space-x-2">
          <Button variant="outline" onClick={fetchBlobs} disabled={loading}>
            <RefreshCw className={`mr-2 h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
          <Dialog open={isUploadDialogOpen} onOpenChange={(open) => {
              if (!open) {
                // Cancel any in-flight upload when the dialog is dismissed;
                // the finally block in handleUpload resets all upload state.
                uploadAbortRef.current?.abort()
              }
              setIsUploadDialogOpen(open)
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
                  Select a file to upload to <strong>{bucketName}</strong>.
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="file">File</Label>
                  <Input
                    id="file"
                    type="file"
                    onChange={(e) => setSelectedFile(e.target.files?.[0] || null)}
                    disabled={isUploading}
                  />
                </div>
                {isUploading && (
                  <div className="space-y-2">
                    <div className="flex justify-between text-sm text-muted-foreground">
                      <span>Uploading…</span>
                      <span>{uploadProgress}%</span>
                    </div>
                    <Progress value={uploadProgress} className="h-2" />
                  </div>
                )}
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setIsUploadDialogOpen(false)} disabled={isUploading}>
                  Cancel
                </Button>
                <Button onClick={handleUpload} disabled={!selectedFile || isUploading}>
                  {isUploading ? "Uploading…" : "Upload"}
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
            <p className="text-xs text-muted-foreground">Files in this bucket</p>
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
              <CardDescription>View and manage objects in {bucketName}</CardDescription>
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
                    onClick={() => handleDownload(blob.bucket, blob.name)}
                    title="Download"
                  >
                    <Download className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => openDeleteDialog(blob.bucket, blob.name)}
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

      {/* Rename Bucket Dialog */}
      <Dialog open={isRenameDialogOpen} onOpenChange={(open) => { if (!open) closeRenameDialog() }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Rename Bucket</DialogTitle>
            <DialogDescription>
              Enter a new name for <strong>{bucketName}</strong>.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="new-bucket-name">New Bucket Name</Label>
              <Input
                id="new-bucket-name"
                placeholder="new-bucket-name"
                value={newBucketName}
                onChange={(e) => handleNewNameChange(e.target.value)}
                onBeforeInput={handleNewNameBeforeInput}
                onPaste={handleNewNamePaste}
                maxLength={BUCKET_NAME_MAX_LENGTH}
              />
              <p className="text-xs text-muted-foreground">
                Bucket names cannot contain spaces and must be 50 characters or fewer.
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
              disabled={!isNewBucketNameValid || newBucketName === bucketName || isRenaming}
            >
              {isRenaming ? "Renaming..." : "Rename"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </DashboardShell>
  )
}
