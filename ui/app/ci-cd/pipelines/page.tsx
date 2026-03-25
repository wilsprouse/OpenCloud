'use client'

import { useEffect, useState } from "react"
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
import { Textarea } from "@/components/ui/textarea"
import { toast } from "sonner"
import client from "@/app/utility/post"
import { PIPELINE_NAME_MAX_LENGTH, isValidPipelineName } from "@/lib/pipeline-name"
import { usePipelineNameWarning } from "@/lib/use-pipeline-name-warning"
import { 
  RefreshCw, 
  Search,
  Play,
  Pause,
  Trash2,
  GitBranch,
  Clock,
  CheckCircle2,
  XCircle,
  Edit,
  Plus,
  Activity,
  AlertCircle,
  Power
} from "lucide-react"

type Pipeline = {
  id: string
  name: string
  description: string
  status: "idle" | "running" | "success" | "failed"
  lastRun?: string
  duration?: string
  branch?: string
  code: string
  createdAt: string
}

const SAMPLE_PIPELINE_SCRIPT = `#!/bin/bash
# Example CI/CD Pipeline Script
set -e  # Exit on error

# Color output functions
warning() {
  echo -e "\\033[1;33m[WARNING]\\033[0m $1"
}

error() {
  echo -e "\\033[1;31m[ERROR]\\033[0m $1"
  exit 1
}

success() {
  echo -e "\\033[1;32m[SUCCESS]\\033[0m $1"
}

# Source stage
echo "=== Source Stage ==="
success "Checking out source code..."
git clone https://github.com/user/repo.git || error "Failed to clone repository"

# Build stage
echo "=== Build Stage ==="
success "Installing dependencies..."
npm install || error "Failed to install dependencies"
success "Building application..."
npm run build || error "Build failed"

# Test stage
echo "=== Test Stage ==="
success "Running tests..."
npm test || error "Tests failed"

# Deploy stage
echo "=== Deploy Stage ==="
warning "Starting deployment..."
# Add your deployment commands here
success "Deployment completed successfully!"`

export default function Pipelines() {
  const router = useRouter()
  const [pipelines, setPipelines] = useState<Pipeline[]>([])
  const [loading, setLoading] = useState(false)
  const [searchTerm, setSearchTerm] = useState("")
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [selectedPipeline, setSelectedPipeline] = useState<Pipeline | null>(null)
  const [pipelineToDelete, setPipelineToDelete] = useState<string | null>(null)
  
  // Service enabled state
  const [serviceEnabled, setServiceEnabled] = useState<boolean | null>(null)
  const [enablingService, setEnablingService] = useState(false)
  
  // Create/Upload form state
  const [pipelineName, setPipelineName] = useState("")
  const [pipelineDescription, setPipelineDescription] = useState("")
  const [pipelineCode, setPipelineCode] = useState("")
  const isPipelineNameValid = isValidPipelineName(pipelineName)
  const {
    handleBeforeInput: handlePipelineNameBeforeInput,
    handleChange: handlePipelineNameChange,
    handlePaste: handlePipelineNamePaste,
    resetWarning: resetPipelineNameWarning,
  } = usePipelineNameWarning(setPipelineName)

  // Check if service is enabled
  const checkServiceStatus = async () => {
    try {
      const res = await client.get<{ service: string; enabled: boolean }>("/get-service-status?service=pipelines")
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
      await client.post("/enable-service", { service: "pipelines" })
      setServiceEnabled(true)
      fetchPipelines()
    } catch (err) {
      console.error("Failed to enable service:", err)
    } finally {
      setEnablingService(false)
    }
  }

  // Fetch pipelines
  const fetchPipelines = async () => {
    setLoading(true)
    try {
      // Sync pipelines from disk to service ledger first
      await client.post("/sync-pipelines")
      
      // Then fetch the pipelines
      const res = await client.get<Pipeline[]>("/get-pipelines")
      setPipelines(res.data || [])
    } catch (err) {
      console.error("Failed to fetch pipelines:", err)
      // Set mock data for demonstration if API fails
      setPipelines([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    checkServiceStatus()
  }, [])

  useEffect(() => {
    if (serviceEnabled) {
      fetchPipelines()
    }
  }, [serviceEnabled])

  // Handle pipeline actions
  const handleRunPipeline = async (id: string) => {
    try {
      await client.post(`/run-pipeline/${id}`)
      fetchPipelines()
    } catch (err) {
      console.error("Failed to run pipeline:", err)
    }
  }

  const handleStopPipeline = async (id: string) => {
    try {
      await client.post(`/stop-pipeline/${id}`)
      fetchPipelines()
    } catch (err) {
      console.error("Failed to stop pipeline:", err)
    }
  }

  const openDeleteDialog = (id: string) => {
    setPipelineToDelete(id)
    setIsDeleteDialogOpen(true)
  }

  const handleDeletePipeline = async () => {
    if (!pipelineToDelete) return
    
    try {
      await client.delete(`/delete-pipeline/${pipelineToDelete}`)
      toast.success("Pipeline deleted successfully")
      fetchPipelines()
      setIsDeleteDialogOpen(false)
      setPipelineToDelete(null)
    } catch (err) {
      console.error("Failed to delete pipeline:", err)
      toast.error("Failed to delete pipeline")
    }
  }

  const handleCreatePipeline = async () => {
    if (!pipelineName || !pipelineCode) {
      toast.error("Please provide both a pipeline name and code")
      return
    }

    try {
      await client.post("/create-pipeline", {
        name: pipelineName,
        description: pipelineDescription,
        code: pipelineCode,
      })
      
      // Reset form and close dialog
      setPipelineName("")
      setPipelineDescription("")
      setPipelineCode("")
      setIsCreateDialogOpen(false)
      
      // Refresh the pipeline list
      await fetchPipelines()
      
      toast.success("Pipeline created successfully")
    } catch (err) {
      console.error("Failed to create pipeline:", err)
      toast.error("Failed to create pipeline")
    }
  }

  const handleEditPipeline = async () => {
    if (!selectedPipeline || !pipelineCode) {
      toast.error("Please provide pipeline code")
      return
    }

    try {
      await client.put(`/update-pipeline/${selectedPipeline.id}`, {
        name: pipelineName,
        description: pipelineDescription,
        code: pipelineCode,
      })
      
      // Reset form and close dialog
      setPipelineName("")
      setPipelineDescription("")
      setPipelineCode("")
      setSelectedPipeline(null)
      setIsEditDialogOpen(false)
      
      // Refresh the pipeline list
      await fetchPipelines()
      
      toast.success("Pipeline updated successfully")
    } catch (err) {
      console.error("Failed to update pipeline:", err)
      toast.error("Failed to update pipeline")
    }
  }

  const openEditDialog = (pipeline: Pipeline) => {
    setSelectedPipeline(pipeline)
    setPipelineName(pipeline.name)
    setPipelineDescription(pipeline.description)
    setPipelineCode(pipeline.code)
    setIsEditDialogOpen(true)
  }

  // Calculate statistics
  const totalPipelines = pipelines.length
  const runningPipelines = pipelines.filter(p => p.status === "running").length
  const successfulPipelines = pipelines.filter(p => p.status === "success").length
  const failedPipelines = pipelines.filter(p => p.status === "failed").length

  // Filter pipelines based on search
  const filteredPipelines = pipelines.filter(p => 
    p.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    p.description?.toLowerCase().includes(searchTerm.toLowerCase()) ||
    p.id.toLowerCase().includes(searchTerm.toLowerCase())
  )

  // Get status icon and color
  const getStatusInfo = (status: string) => {
    switch (status) {
      case "running":
        return { icon: Activity, color: "text-blue-600", bg: "bg-blue-50", badge: "bg-blue-100 text-blue-800" }
      case "success":
        return { icon: CheckCircle2, color: "text-green-600", bg: "bg-green-50", badge: "bg-green-100 text-green-800" }
      case "failed":
        return { icon: XCircle, color: "text-red-600", bg: "bg-red-50", badge: "bg-red-100 text-red-800" }
      default:
        return { icon: Clock, color: "text-gray-600", bg: "bg-gray-50", badge: "bg-gray-100 text-gray-800" }
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
        <DashboardHeader heading="Pipelines" text="CI/CD automation" />
        <div className="flex items-center justify-center min-h-[400px]">
          <Card className="max-w-md w-full">
            <CardHeader className="text-center">
              <div className="mx-auto p-3 rounded-full bg-blue-50 w-fit mb-4">
                <GitBranch className="h-8 w-8 text-blue-600" />
              </div>
              <CardTitle>Enable Pipelines Service</CardTitle>
              <CardDescription>
                The Pipelines service is not yet enabled. Enable it to start creating and managing CI/CD pipelines.
              </CardDescription>
            </CardHeader>
            <CardContent className="flex justify-center">
              <Button onClick={handleEnableService} disabled={enablingService} size="lg">
                <Power className="mr-2 h-4 w-4" />
                {enablingService ? "Enabling..." : "Enable Pipelines"}
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
        heading="CI/CD Pipelines" 
        text="Manage and run your continuous integration and deployment pipelines"
      >
        <div className="flex items-center space-x-2">
          <Button variant="outline" onClick={fetchPipelines} disabled={loading}>
            <RefreshCw className={`mr-2 h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            {loading ? "Refreshing..." : "Refresh"}
          </Button>
          <Dialog open={isCreateDialogOpen} onOpenChange={(open) => {
              setIsCreateDialogOpen(open)
              if (!open) resetPipelineNameWarning()
            }}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="mr-2 h-4 w-4" />
                New Pipeline
              </Button>
            </DialogTrigger>
            <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
              <DialogHeader>
                <DialogTitle>Create New Pipeline</DialogTitle>
                <DialogDescription>
                  Define your CI/CD pipeline with bash scripts.
                </DialogDescription>
              </DialogHeader>
              
              <div className="grid gap-4 py-4">
                {/* Pipeline Name */}
                <div className="grid gap-2">
                  <Label htmlFor="pipelineName">Pipeline Name *</Label>
                  <Input
                    id="pipelineName"
                    placeholder="my-build-pipeline"
                    value={pipelineName}
                    onChange={(e) => handlePipelineNameChange(e.target.value)}
                    onBeforeInput={handlePipelineNameBeforeInput}
                    onPaste={handlePipelineNamePaste}
                    maxLength={PIPELINE_NAME_MAX_LENGTH}
                  />
                  <p className="text-xs text-muted-foreground">
                    Pipeline names cannot contain spaces and must be 50 characters or fewer.
                  </p>
                </div>

                {/* Pipeline Description */}
                <div className="grid gap-2">
                  <Label htmlFor="pipelineDescription">Description</Label>
                  <Input
                    id="pipelineDescription"
                    placeholder="Build and test application"
                    value={pipelineDescription}
                    onChange={(e) => setPipelineDescription(e.target.value)}
                  />
                </div>

                {/* Pipeline Code */}
                <div className="grid gap-2">
                  <Label htmlFor="pipelineCode">Pipeline Configuration *</Label>
                  <Textarea
                    id="pipelineCode"
                    placeholder={SAMPLE_PIPELINE_SCRIPT}
                    className="min-h-[300px] font-mono text-sm"
                    value={pipelineCode}
                    onChange={(e) => setPipelineCode(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Tab" && !pipelineCode) {
                        e.preventDefault()
                        setPipelineCode(SAMPLE_PIPELINE_SCRIPT)
                      }
                    }}
                  />
                  <p className="text-xs text-muted-foreground">
                    Press <kbd className="px-1 py-0.5 rounded border border-muted-foreground/30 bg-muted font-mono text-xs">Tab</kbd> to populate with a sample pipeline script, or type your own bash script directly.
                  </p>
                </div>
              </div>

              <DialogFooter>
                <Button
                  variant="outline"
                  onClick={() => { setIsCreateDialogOpen(false); resetPipelineNameWarning() }}
                >
                  Cancel
                </Button>
                <Button
                  onClick={handleCreatePipeline}
                  disabled={!isPipelineNameValid || !pipelineCode}
                >
                  <Plus className="mr-2 h-4 w-4" />
                  Create Pipeline
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </DashboardHeader>

      {/* Statistics Cards */}
      <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
        <Card className="border-l-4 border-l-blue-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Pipelines</CardTitle>
            <GitBranch className="h-4 w-4 text-blue-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalPipelines}</div>
            <p className="text-xs text-muted-foreground">Configured pipelines</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-orange-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Running</CardTitle>
            <Activity className="h-4 w-4 text-orange-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{runningPipelines}</div>
            <p className="text-xs text-muted-foreground">Active pipelines</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-green-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Successful</CardTitle>
            <CheckCircle2 className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{successfulPipelines}</div>
            <p className="text-xs text-muted-foreground">Completed successfully</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-red-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Failed</CardTitle>
            <AlertCircle className="h-4 w-4 text-red-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{failedPipelines}</div>
            <p className="text-xs text-muted-foreground">Failed executions</p>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Pipelines</CardTitle>
              <CardDescription>Manage your CI/CD pipelines</CardDescription>
            </div>
          </div>
          <div className="relative mt-4">
            <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
            <input
              type="text"
              placeholder="Search pipelines by name or description..."
              className="w-full pl-8 pr-4 py-2 border rounded-md bg-background"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
            />
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {filteredPipelines.map((pipeline) => {
              const statusInfo = getStatusInfo(pipeline.status)
              const StatusIcon = statusInfo.icon
              const isRunning = pipeline.status === "running"
              
              return (
                <div
                  key={pipeline.id}
                  className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors cursor-pointer"
                  onClick={() => router.push(`/ci-cd/pipelines/${encodeURIComponent(pipeline.id)}`)}
                >
                  <div className="flex items-center space-x-4 flex-1">
                    <div className={`p-2 rounded-lg ${statusInfo.bg}`}>
                      <StatusIcon className={`h-5 w-5 ${statusInfo.color} ${isRunning ? 'animate-pulse' : ''}`} />
                    </div>
                    <div className="space-y-1 flex-1 min-w-0">
                      <div className="flex items-center space-x-2">
                        <h4 className="font-medium truncate">{pipeline.name}</h4>
                        <Badge 
                          variant="outline" 
                          className={statusInfo.badge}
                        >
                          {pipeline.status.charAt(0).toUpperCase() + pipeline.status.slice(1)}
                        </Badge>
                      </div>
                      {pipeline.description && (
                        <p className="text-sm text-muted-foreground truncate">{pipeline.description}</p>
                      )}
                      <div className="flex items-center space-x-4 text-xs text-muted-foreground">
                        {pipeline.lastRun && (
                          <span className="flex items-center">
                            <Clock className="h-3 w-3 mr-1" />
                            Last run: {new Date(pipeline.lastRun).toLocaleString()}
                          </span>
                        )}
                        {pipeline.duration && (
                          <>
                            <span>•</span>
                            <span>{pipeline.duration}</span>
                          </>
                        )}
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center space-x-2 ml-4" onClick={(e) => e.stopPropagation()}>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => openEditDialog(pipeline)}
                      title="Edit Pipeline"
                    >
                      <Edit className="h-4 w-4 mr-1" />
                      Edit
                    </Button>
                    {!isRunning ? (
                      <Button
                        variant="default"
                        size="sm"
                        onClick={() => handleRunPipeline(pipeline.id)}
                        title="Run Pipeline"
                      >
                        <Play className="h-4 w-4 mr-1" />
                        Run
                      </Button>
                    ) : (
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => handleStopPipeline(pipeline.id)}
                        title="Stop Pipeline"
                      >
                        <Pause className="h-4 w-4 mr-1" />
                        Stop
                      </Button>
                    )}
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={() => openDeleteDialog(pipeline.id)}
                      title="Delete Pipeline"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              )
            })}
            {filteredPipelines.length === 0 && !loading && (
              <div className="text-center py-12">
                <GitBranch className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
                <h3 className="text-lg font-medium mb-2">No pipelines found</h3>
                <p className="text-sm text-muted-foreground mb-4">
                  {searchTerm ? "Try adjusting your search terms" : "Create your first pipeline to get started"}
                </p>
                {!searchTerm && (
                  <Button onClick={() => setIsCreateDialogOpen(true)}>
                    <Plus className="mr-2 h-4 w-4" />
                    Create Pipeline
                  </Button>
                )}
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Edit Pipeline Dialog */}
      <Dialog open={isEditDialogOpen} onOpenChange={(open) => {
          setIsEditDialogOpen(open)
          if (!open) resetPipelineNameWarning()
        }}>
        <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Edit Pipeline</DialogTitle>
            <DialogDescription>
              Update your pipeline configuration and settings.
            </DialogDescription>
          </DialogHeader>
          
          <div className="grid gap-4 py-4">
            {/* Pipeline Name */}
            <div className="grid gap-2">
              <Label htmlFor="editPipelineName">Pipeline Name *</Label>
              <Input
                id="editPipelineName"
                placeholder="my-build-pipeline"
                value={pipelineName}
                onChange={(e) => handlePipelineNameChange(e.target.value)}
                onBeforeInput={handlePipelineNameBeforeInput}
                onPaste={handlePipelineNamePaste}
                maxLength={PIPELINE_NAME_MAX_LENGTH}
              />
              <p className="text-xs text-muted-foreground">
                Pipeline names cannot contain spaces and must be 50 characters or fewer.
              </p>
            </div>

            {/* Pipeline Description */}
            <div className="grid gap-2">
              <Label htmlFor="editPipelineDescription">Description</Label>
              <Input
                id="editPipelineDescription"
                placeholder="Build and test application"
                value={pipelineDescription}
                onChange={(e) => setPipelineDescription(e.target.value)}
              />
            </div>

            {/* Pipeline Code */}
            <div className="grid gap-2">
              <Label htmlFor="editPipelineCode">Pipeline Configuration *</Label>
              <Textarea
                id="editPipelineCode"
                className="min-h-[300px] font-mono text-sm"
                value={pipelineCode}
                onChange={(e) => setPipelineCode(e.target.value)}
              />
              <p className="text-xs text-muted-foreground">
                Edit your pipeline configuration as a bash script
              </p>
            </div>
          </div>

          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setIsEditDialogOpen(false)
                setSelectedPipeline(null)
                setPipelineName("")
                setPipelineDescription("")
                setPipelineCode("")
                setPipelineBranch("main")
              }}
            >
              Cancel
            </Button>
            <Button
              onClick={handleEditPipeline}
              disabled={!isPipelineNameValid || !pipelineCode}
            >
              <Edit className="mr-2 h-4 w-4" />
              Update Pipeline
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Pipeline</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete this pipeline? This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setIsDeleteDialogOpen(false)
                setPipelineToDelete(null)
              }}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleDeletePipeline}
            >
              <Trash2 className="mr-2 h-4 w-4" />
              Delete Pipeline
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </DashboardShell>
  )
}
