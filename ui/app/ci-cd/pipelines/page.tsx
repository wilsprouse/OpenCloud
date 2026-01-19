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
import { Textarea } from "@/components/ui/textarea"
import { toast } from "sonner"
import client from "@/app/utility/post"
import { 
  RefreshCw, 
  Search,
  Play,
  Pause,
  Trash2,
  Copy,
  GitBranch,
  Clock,
  CheckCircle2,
  XCircle,
  Upload,
  Edit,
  Plus,
  FileCode,
  Activity,
  AlertCircle
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

export default function Pipelines() {
  const [pipelines, setPipelines] = useState<Pipeline[]>([])
  const [loading, setLoading] = useState(false)
  const [searchTerm, setSearchTerm] = useState("")
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [selectedPipeline, setSelectedPipeline] = useState<Pipeline | null>(null)
  const [pipelineToDelete, setPipelineToDelete] = useState<string | null>(null)
  
  // Create/Upload form state
  const [pipelineName, setPipelineName] = useState("")
  const [pipelineDescription, setPipelineDescription] = useState("")
  const [pipelineCode, setPipelineCode] = useState("")
  const [pipelineBranch, setPipelineBranch] = useState("main")

  // Fetch pipelines
  const fetchPipelines = async () => {
    setLoading(true)
    try {
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
    fetchPipelines()
  }, [])

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
        branch: pipelineBranch,
      })
      
      // Reset form and close dialog
      setPipelineName("")
      setPipelineDescription("")
      setPipelineCode("")
      setPipelineBranch("main")
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
        branch: pipelineBranch,
      })
      
      // Reset form and close dialog
      setPipelineName("")
      setPipelineDescription("")
      setPipelineCode("")
      setPipelineBranch("main")
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
    setPipelineBranch(pipeline.branch || "main")
    setIsEditDialogOpen(true)
  }

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text)
      toast.success("Copied to clipboard")
    } catch (err) {
      console.error("Failed to copy to clipboard:", err)
      toast.error("Failed to copy to clipboard")
    }
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
          <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
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
                    onChange={(e) => setPipelineName(e.target.value)}
                  />
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

                {/* Branch
                <div className="grid gap-2">
                  <Label htmlFor="pipelineBranch">Branch</Label>
                  <Input
                    id="pipelineBranch"
                    placeholder="main"
                    value={pipelineBranch}
                    onChange={(e) => setPipelineBranch(e.target.value)}
                  />
                </div*/}

                {/* Pipeline Code */}
                <div className="grid gap-2">
                  <Label htmlFor="pipelineCode">Pipeline Configuration *</Label>
                  <Textarea
                    id="pipelineCode"
                    placeholder={`#!/bin/bash
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
success "Deployment completed successfully!"`}
                    className="min-h-[300px] font-mono text-sm"
                    value={pipelineCode}
                    onChange={(e) => setPipelineCode(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    Enter your pipeline configuration as a bash script
                  </p>
                </div>
              </div>

              <DialogFooter>
                <Button
                  variant="outline"
                  onClick={() => setIsCreateDialogOpen(false)}
                >
                  Cancel
                </Button>
                <Button
                  onClick={handleCreatePipeline}
                  disabled={!pipelineName || !pipelineCode}
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

      <div className="grid gap-6 md:grid-cols-3">
        {/* Main Pipeline List */}
        <div className="md:col-span-2">
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
                      className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
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
                            <span className="flex items-center">
                              <GitBranch className="h-3 w-3 mr-1" />
                              {pipeline.branch || "main"}
                            </span>
                            {pipeline.lastRun && (
                              <>
                                <span>•</span>
                                <span className="flex items-center">
                                  <Clock className="h-3 w-3 mr-1" />
                                  Last run: {new Date(pipeline.lastRun).toLocaleString()}
                                </span>
                              </>
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
                      <div className="flex items-center space-x-2 ml-4">
                        <Button 
                          variant="ghost" 
                          size="icon"
                          onClick={() => copyToClipboard(pipeline.id)}
                          title="Copy ID"
                        >
                          <Copy className="h-4 w-4" />
                        </Button>
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
        </div>

        {/* Quick Actions Sidebar */}
        <div>
          <Card>
            <CardHeader>
              <CardTitle>Quick Actions</CardTitle>
              <CardDescription>Common pipeline operations</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <Button 
                variant="ghost" 
                className="w-full justify-start h-auto p-4 bg-blue-50 hover:bg-blue-100"
                onClick={() => setIsCreateDialogOpen(true)}
              >
                <div className="flex items-center space-x-3">
                  <div className="p-2 rounded-lg bg-white text-blue-600">
                    <Plus className="h-4 w-4" />
                  </div>
                  <div className="text-left">
                    <div className="font-medium text-sm">Create Pipeline</div>
                    <div className="text-xs text-muted-foreground">Add new CI/CD pipeline</div>
                  </div>
                </div>
              </Button>

              <Button variant="ghost" className="w-full justify-start h-auto p-4 bg-green-50 hover:bg-green-100">
                <div className="flex items-center space-x-3">
                  <div className="p-2 rounded-lg bg-white text-green-600">
                    <Upload className="h-4 w-4" />
                  </div>
                  <div className="text-left">
                    <div className="font-medium text-sm">Import Pipeline</div>
                    <div className="text-xs text-muted-foreground">Upload from file</div>
                  </div>
                </div>
              </Button>

              <Button variant="ghost" className="w-full justify-start h-auto p-4 bg-purple-50 hover:bg-purple-100">
                <div className="flex items-center space-x-3">
                  <div className="p-2 rounded-lg bg-white text-purple-600">
                    <FileCode className="h-4 w-4" />
                  </div>
                  <div className="text-left">
                    <div className="font-medium text-sm">View Templates</div>
                    <div className="text-xs text-muted-foreground">Browse pipeline templates</div>
                  </div>
                </div>
              </Button>

              <Button variant="ghost" className="w-full justify-start h-auto p-4 bg-orange-50 hover:bg-orange-100">
                <div className="flex items-center space-x-3">
                  <div className="p-2 rounded-lg bg-white text-orange-600">
                    <Activity className="h-4 w-4" />
                  </div>
                  <div className="text-left">
                    <div className="font-medium text-sm">View Logs</div>
                    <div className="text-xs text-muted-foreground">Check execution logs</div>
                  </div>
                </div>
              </Button>
            </CardContent>
          </Card>

          {/* Pipeline Info Card */}
          <Card className="mt-6">
            <CardHeader>
              <CardTitle>Pipeline Information</CardTitle>
              <CardDescription>CI/CD system details</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div>
                <div className="text-xs font-medium text-muted-foreground mb-1">Active Pipelines</div>
                <div className="text-2xl font-bold">{runningPipelines}</div>
              </div>
              <div>
                <div className="text-xs font-medium text-muted-foreground mb-1">Success Rate</div>
                <div className="text-2xl font-bold">
                  {totalPipelines > 0 
                    ? `${Math.round((successfulPipelines / totalPipelines) * 100)}%`
                    : "N/A"}
                </div>
              </div>
              <div>
                <div className="text-xs font-medium text-muted-foreground mb-1">Total Pipelines</div>
                <div className="text-2xl font-bold">{totalPipelines}</div>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Edit Pipeline Dialog */}
      <Dialog open={isEditDialogOpen} onOpenChange={setIsEditDialogOpen}>
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
                onChange={(e) => setPipelineName(e.target.value)}
              />
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

            {/* Branch */}
            <div className="grid gap-2">
              <Label htmlFor="editPipelineBranch">Branch</Label>
              <Input
                id="editPipelineBranch"
                placeholder="main"
                value={pipelineBranch}
                onChange={(e) => setPipelineBranch(e.target.value)}
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
              disabled={!pipelineCode}
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
