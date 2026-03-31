'use client'

import { Suspense, useEffect, useRef, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
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
import client from "@/app/utility/post"
import { BUCKET_NAME_MAX_LENGTH, isValidBucketName } from "@/lib/bucket-name"
import { useBucketNameWarning } from "@/lib/use-bucket-name-warning"
import { 
  RefreshCw, 
  Search,
  FolderPlus,
  File,
  HardDrive,
  Package,
  Folder,
  ChevronRight,
  Database,
  Power
} from "lucide-react"

type Bucket = {
  name: string
  objectCount: number
  totalSize: number
  lastModified: string
}

function SearchParamsReader({ onCreateRequested }: { onCreateRequested: () => void }) {
  const searchParams = useSearchParams()
  const handled = useRef(false)
  useEffect(() => {
    if (!handled.current && searchParams.get("create") === "true") {
      handled.current = true
      onCreateRequested()
    }
  }, [searchParams, onCreateRequested])
  return null
}

export default function BlobStorage() {
  const router = useRouter()
  const [buckets, setBuckets] = useState<Bucket[]>([])
  const [loading, setLoading] = useState(false)
  const [searchTerm, setSearchTerm] = useState("")
  const [isBucketDialogOpen, setIsBucketDialogOpen] = useState(false)

  // Service enabled state
  const [serviceEnabled, setServiceEnabled] = useState<boolean | null>(null)
  const [enablingService, setEnablingService] = useState(false)
  
  // Bucket form state
  const [bucketName, setBucketName] = useState<string>("")
  const isBucketNameValid = isValidBucketName(bucketName)
  const {
    handleBeforeInput: handleBucketNameBeforeInput,
    handleChange: handleBucketNameChange,
    handlePaste: handleBucketNamePaste,
    resetWarning: resetBucketNameWarning,
  } = useBucketNameWarning(setBucketName)

  // Check if service is enabled
  const checkServiceStatus = async () => {
    try {
      const res = await client.get<{ service: string; enabled: boolean }>("/get-service-status?service=blob_storage")
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
      await client.post("/enable-service", { service: "blob_storage" })
      setServiceEnabled(true)
      fetchBuckets()
    } catch (err) {
      console.error("Failed to enable service:", err)
    } finally {
      setEnablingService(false)
    }
  }

  // Fetch buckets
  const fetchBuckets = async () => {
    setLoading(true)
    try {
      const res = await client.get<Bucket[]>("/list-blob-buckets")
      setBuckets(res.data || [])
    } catch (err) {
      console.error("Failed to fetch buckets:", err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    checkServiceStatus()
  }, [])

  useEffect(() => {
    if (serviceEnabled) {
      fetchBuckets()
    }
  }, [serviceEnabled])

  const handleCreateBucket = async (name: string) => {
    try {
      console.log(`Creating bucket: ${name}`)
      const res = await client.post("/create-bucket", { name })

      if (res.status === 200 || res.status === 201) {
        setIsBucketDialogOpen(false)
        setBucketName("")
        fetchBuckets()
      }
    } catch (err) {
      console.error("Failed to create bucket:", err)
    }
  }

  const handleBucketClick = (bucketName: string) => {
    router.push(`/storage/blob/${encodeURIComponent(bucketName)}`)
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

  // Filter buckets based on search
  const filteredBuckets = buckets.filter(bucket => 
    bucket.name.toLowerCase().includes(searchTerm.toLowerCase())
  )

  // Calculate statistics
  const totalBuckets = buckets.length
  const totalObjects = buckets.reduce((sum, bucket) => sum + bucket.objectCount, 0)
  const totalSize = buckets.reduce((sum, bucket) => sum + bucket.totalSize, 0)

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
        <DashboardHeader heading="Blob Storage" text="Manage your buckets and objects" />
        <div className="flex items-center justify-center min-h-[400px]">
          <Card className="max-w-md w-full">
            <CardHeader className="text-center">
              <div className="mx-auto p-3 rounded-full bg-blue-50 w-fit mb-4">
                <Database className="h-8 w-8 text-blue-600" />
              </div>
              <CardTitle>Enable Blob Storage Service</CardTitle>
              <CardDescription>
                The Blob Storage service is not yet enabled. Enable it to start creating buckets and managing objects.
              </CardDescription>
            </CardHeader>
            <CardContent className="flex justify-center">
              <Button onClick={handleEnableService} disabled={enablingService} size="lg">
                <Power className="mr-2 h-4 w-4" />
                {enablingService ? "Enabling..." : "Enable Blob Storage"}
              </Button>
            </CardContent>
          </Card>
        </div>
      </DashboardShell>
    )
  }

  return (
    <DashboardShell>
      <Suspense fallback={null}>
        <SearchParamsReader onCreateRequested={() => setIsBucketDialogOpen(true)} />
      </Suspense>
      <DashboardHeader heading="Blob Storage" text="Manage your buckets and objects">
        <div className="flex items-center space-x-2">
          <Button variant="outline" onClick={fetchBuckets} disabled={loading}>
            <RefreshCw className={`mr-2 h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
          <Dialog open={isBucketDialogOpen} onOpenChange={(open) => {
              setIsBucketDialogOpen(open)
              if (!open) resetBucketNameWarning()
            }}>
            <DialogTrigger asChild>
              <Button>
                <FolderPlus className="mr-2 h-4 w-4" />
                Create Bucket
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create New Bucket</DialogTitle>
                <DialogDescription>
                  Buckets help organize your objects. Enter a name for your new bucket.
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="bucket-name">Bucket Name</Label>
                  <Input
                    id="bucket-name"
                    placeholder="my-bucket"
                    value={bucketName}
                    onChange={(e) => handleBucketNameChange(e.target.value)}
                    onBeforeInput={handleBucketNameBeforeInput}
                    onPaste={handleBucketNamePaste}
                    maxLength={BUCKET_NAME_MAX_LENGTH}
                  />
                  <p className="text-xs text-muted-foreground">
                    Bucket names cannot contain spaces and must be 50 characters or fewer.
                  </p>
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => { setIsBucketDialogOpen(false); resetBucketNameWarning() }}>
                  Cancel
                </Button>
                <Button onClick={() => handleCreateBucket(bucketName)} disabled={!isBucketNameValid}>
                  Create
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
            <CardTitle className="text-sm font-medium">Total Buckets</CardTitle>
            <Package className="h-4 w-4 text-blue-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalBuckets}</div>
            <p className="text-xs text-muted-foreground">Storage buckets</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-green-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Objects</CardTitle>
            <File className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalObjects}</div>
            <p className="text-xs text-muted-foreground">Files stored</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-purple-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Storage Used</CardTitle>
            <HardDrive className="h-4 w-4 text-purple-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{formatSize(totalSize)}</div>
            <p className="text-xs text-muted-foreground">Total storage size</p>
          </CardContent>
        </Card>
      </div>

      {/* Main Bucket List */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Buckets</CardTitle>
              <CardDescription>Browse and manage your storage buckets</CardDescription>
            </div>
          </div>
          <div className="relative mt-4">
            <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
            <input
              type="text"
              placeholder="Search buckets by name..."
              className="w-full pl-8 pr-4 py-2 border rounded-md bg-background"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
            />
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {filteredBuckets.map((bucket) => (
              <div
                key={bucket.name}
                className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors cursor-pointer"
                onClick={() => handleBucketClick(bucket.name)}
              >
                <div className="flex items-center space-x-4 flex-1">
                  <div className="p-2 rounded-lg bg-blue-50">
                    <Folder className="h-5 w-5 text-blue-600" />
                  </div>
                  <div className="space-y-1 flex-1 min-w-0">
                    <div className="flex items-center space-x-2">
                      <h4 className="font-medium truncate">{bucket.name}</h4>
                    </div>
                    <div className="flex items-center space-x-4 text-xs text-muted-foreground">
                      <span className="flex items-center">
                        <File className="h-3 w-3 mr-1" />
                        {bucket.objectCount} {bucket.objectCount === 1 ? 'object' : 'objects'}
                      </span>
                      <span>•</span>
                      <span className="flex items-center">
                        <HardDrive className="h-3 w-3 mr-1" />
                        {formatSize(bucket.totalSize)}
                      </span>
                      <span>•</span>
                      <span>Modified: {formatDate(bucket.lastModified)}</span>
                    </div>
                  </div>
                </div>
                <div className="flex items-center space-x-2 ml-4">
                  <ChevronRight className="h-5 w-5 text-muted-foreground" />
                </div>
              </div>
            ))}
            {filteredBuckets.length === 0 && !loading && (
              <div className="text-center py-12">
                <Package className="mx-auto h-12 w-12 text-muted-foreground" />
                <h3 className="mt-4 text-lg font-semibold">No buckets found</h3>
                <p className="mt-2 text-sm text-muted-foreground">
                  {searchTerm ? "Try adjusting your search terms" : "Create your first bucket to get started"}
                </p>
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </DashboardShell>
  )
}
