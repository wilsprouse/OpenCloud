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
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { toast } from "sonner"
import client from "@/app/utility/post"
import { 
  ArrowLeft,
  RefreshCw, 
  Play,
  Pause,
  Trash2,
  GitBranch,
  Clock,
  CheckCircle2,
  XCircle,
  Edit,
  Activity,
  AlertCircle,
  Save,
  Terminal,
  FileCode,
  Settings,
  Calendar
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

type PipelineLog = {
  timestamp: string
  output: string
  error?: string
  status: "success" | "error"
}

export default function PipelineDetail({ params }: { params: Promise<{ id: string }> }) {
  const resolvedParams = use(params)
  const pipelineId = decodeURIComponent(resolvedParams.id)
  const router = useRouter()
  
  const [pipeline, setPipeline] = useState<Pipeline | null>(null)
  const [loading, setLoading] = useState(false)
  const [running, setRunning] = useState(false)
  const [saving, setSaving] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [isEditMode, setIsEditMode] = useState(false)
  const [logs, setLogs] = useState<PipelineLog[]>([])
  const [loadingLogs, setLoadingLogs] = useState(false)
  const [activeTab, setActiveTab] = useState("overview")
  
  // Editable form state
  const [pipelineName, setPipelineName] = useState("")
  const [pipelineDescription, setPipelineDescription] = useState("")
  const [pipelineCode, setPipelineCode] = useState("")
  const [pipelineBranch, setPipelineBranch] = useState("main")

  // Fetch pipeline details
  const fetchPipelineDetails = async () => {
    setLoading(true)
    try {
      const res = await client.get<Pipeline>(`/get-pipeline/${encodeURIComponent(pipelineId)}`)
      const data = res.data
      setPipeline(data)
      
      // Populate form fields
      setPipelineName(data.name)
      setPipelineDescription(data.description || "")
      setPipelineCode(data.code || "")
      setPipelineBranch(data.branch || "main")
    } catch (err) {
      console.error("Failed to fetch pipeline details:", err)
    } finally {
      setLoading(false)
    }
  }

  // Fetch pipeline logs
  const fetchPipelineLogs = async () => {
    setLoadingLogs(true)
    try {
      const res = await client.get<PipelineLog[]>(`/get-pipeline-logs/${encodeURIComponent(pipelineId)}`)
      setLogs(res.data || [])
    } catch (err) {
      console.error("Failed to fetch pipeline logs:", err)
      setLogs([])
    } finally {
      setLoadingLogs(false)
    }
  }

  useEffect(() => {
    fetchPipelineDetails()
    fetchPipelineLogs()
  }, [pipelineId])

  const handleRunPipeline = async () => {
    setRunning(true)
    try {
      await client.post(`/run-pipeline/${pipelineId}`)
      toast.success("Pipeline started successfully")
      fetchPipelineDetails()
      fetchPipelineLogs()
      setActiveTab("logs")
    } catch (err) {
      console.error("Failed to run pipeline:", err)
      toast.error("Failed to run pipeline")
    } finally {
      setRunning(false)
    }
  }

  const handleStopPipeline = async () => {
    try {
      await client.post(`/stop-pipeline/${pipelineId}`)
      toast.success("Pipeline stopped")
      fetchPipelineDetails()
    } catch (err) {
      console.error("Failed to stop pipeline:", err)
      toast.error("Failed to stop pipeline")
    }
  }

  const handleSavePipeline = async () => {
    if (!pipelineCode) {
      toast.error("Please provide pipeline code")
      return
    }

    setSaving(true)
    try {
      await client.put(`/update-pipeline/${pipelineId}`, {
        name: pipelineName,
        description: pipelineDescription,
        code: pipelineCode,
        branch: pipelineBranch,
      })
      
      toast.success("Pipeline updated successfully")
      setIsEditMode(false)
      fetchPipelineDetails()
    } catch (err) {
      console.error("Failed to update pipeline:", err)
      toast.error("Failed to update pipeline")
    } finally {
      setSaving(false)
    }
  }

  const handleDeletePipeline = async () => {
    try {
      await client.delete(`/delete-pipeline/${pipelineId}`)
      toast.success("Pipeline deleted successfully")
      router.push('/ci-cd/pipelines')
    } catch (err) {
      console.error("Failed to delete pipeline:", err)
      toast.error("Failed to delete pipeline")
    }
  }

  const cancelEdit = () => {
    if (pipeline) {
      setPipelineName(pipeline.name)
      setPipelineDescription(pipeline.description || "")
      setPipelineCode(pipeline.code || "")
      setPipelineBranch(pipeline.branch || "main")
    }
    setIsEditMode(false)
  }

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

  if (loading && !pipeline) {
    return (
      <DashboardShell>
        <div className="flex items-center justify-center h-64">
          <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      </DashboardShell>
    )
  }

  if (!pipeline && !loading) {
    return (
      <DashboardShell>
        <div className="text-center py-12">
          <GitBranch className="mx-auto h-12 w-12 text-muted-foreground" />
          <h3 className="mt-4 text-lg font-semibold">Pipeline not found</h3>
          <p className="mt-2 text-sm text-muted-foreground">
            The pipeline you're looking for doesn't exist.
          </p>
          <Button onClick={() => router.push('/ci-cd/pipelines')} className="mt-4">
            Back to Pipelines
          </Button>
        </div>
      </DashboardShell>
    )
  }

  const statusInfo = pipeline ? getStatusInfo(pipeline.status) : getStatusInfo("idle")
  const StatusIcon = statusInfo.icon
  const isRunning = pipeline?.status === "running"

  return (
    <DashboardShell>
      <DashboardHeader 
        heading={
          <div className="flex items-center space-x-2">
            <Button 
              variant="ghost" 
              size="icon"
              onClick={() => router.push('/ci-cd/pipelines')}
              className="mr-2"
            >
              <ArrowLeft className="h-4 w-4" />
            </Button>
            <span>{pipelineName || pipelineId}</span>
            {pipeline && (
              <Badge className={statusInfo.badge}>
                {pipeline.status.charAt(0).toUpperCase() + pipeline.status.slice(1)}
              </Badge>
            )}
          </div>
        } 
        text="View and manage pipeline details"
      >
        <div className="flex items-center space-x-2">
          <Button variant="outline" onClick={fetchPipelineDetails} disabled={loading}>
            <RefreshCw className={`mr-2 h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
          {!isEditMode ? (
            <>
              {!isRunning ? (
                <Button onClick={handleRunPipeline} disabled={running}>
                  {running ? (
                    <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                  ) : (
                    <Play className="mr-2 h-4 w-4" />
                  )}
                  {running ? "Starting..." : "Run"}
                </Button>
              ) : (
                <Button variant="secondary" onClick={handleStopPipeline}>
                  <Pause className="mr-2 h-4 w-4" />
                  Stop
                </Button>
              )}
              <Button variant="outline" onClick={() => setIsEditMode(true)}>
                <Edit className="mr-2 h-4 w-4" />
                Edit
              </Button>
              <Button variant="destructive" onClick={() => setIsDeleteDialogOpen(true)}>
                <Trash2 className="mr-2 h-4 w-4" />
                Delete
              </Button>
            </>
          ) : (
            <>
              <Button variant="outline" onClick={cancelEdit}>
                Cancel
              </Button>
              <Button onClick={handleSavePipeline} disabled={saving}>
                <Save className="mr-2 h-4 w-4" />
                {saving ? "Saving..." : "Save"}
              </Button>
            </>
          )}
        </div>
      </DashboardHeader>

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Info & Configuration Section */}
        <Card className="lg:col-span-1">
          <CardHeader>
            <CardTitle className="flex items-center">
              <Settings className="h-5 w-5 mr-2" />
              Pipeline Info
            </CardTitle>
            <CardDescription>Pipeline configuration and metadata</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {isEditMode ? (
              <>
                <div className="space-y-2">
                  <Label htmlFor="pipeline-name">Pipeline Name</Label>
                  <Input
                    id="pipeline-name"
                    value={pipelineName}
                    onChange={(e) => setPipelineName(e.target.value)}
                    placeholder="my-pipeline"
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="pipeline-description">Description</Label>
                  <Input
                    id="pipeline-description"
                    value={pipelineDescription}
                    onChange={(e) => setPipelineDescription(e.target.value)}
                    placeholder="Pipeline description"
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="pipeline-branch">Branch</Label>
                  <Input
                    id="pipeline-branch"
                    value={pipelineBranch}
                    onChange={(e) => setPipelineBranch(e.target.value)}
                    placeholder="main"
                  />
                </div>
              </>
            ) : (
              <>
                <div className="space-y-1">
                  <div className="text-sm font-medium text-muted-foreground">Name</div>
                  <div className="text-base font-semibold">{pipeline?.name}</div>
                </div>

                {pipeline?.description && (
                  <div className="space-y-1">
                    <div className="text-sm font-medium text-muted-foreground">Description</div>
                    <div className="text-sm">{pipeline.description}</div>
                  </div>
                )}

                <div className="space-y-1">
                  <div className="text-sm font-medium text-muted-foreground">Status</div>
                  <div className="flex items-center space-x-2">
                    <div className={`p-2 rounded-lg ${statusInfo.bg}`}>
                      <StatusIcon className={`h-4 w-4 ${statusInfo.color} ${isRunning ? 'animate-pulse' : ''}`} />
                    </div>
                    <span className="text-sm capitalize">{pipeline?.status}</span>
                  </div>
                </div>

                <div className="space-y-1">
                  <div className="text-sm font-medium text-muted-foreground">Branch</div>
                  <div className="flex items-center space-x-1 text-sm">
                    <GitBranch className="h-4 w-4" />
                    <span>{pipeline?.branch || "main"}</span>
                  </div>
                </div>

                {pipeline?.lastRun && (
                  <div className="space-y-1">
                    <div className="text-sm font-medium text-muted-foreground">Last Run</div>
                    <div className="flex items-center space-x-1 text-sm">
                      <Clock className="h-4 w-4" />
                      <span>{new Date(pipeline.lastRun).toLocaleString()}</span>
                    </div>
                  </div>
                )}

                {pipeline?.duration && (
                  <div className="space-y-1">
                    <div className="text-sm font-medium text-muted-foreground">Duration</div>
                    <div className="text-sm">{pipeline.duration}</div>
                  </div>
                )}

                {pipeline?.createdAt && (
                  <div className="space-y-1">
                    <div className="text-sm font-medium text-muted-foreground">Created</div>
                    <div className="flex items-center space-x-1 text-sm">
                      <Calendar className="h-4 w-4" />
                      <span>{new Date(pipeline.createdAt).toLocaleString()}</span>
                    </div>
                  </div>
                )}

                <div className="space-y-1">
                  <div className="text-sm font-medium text-muted-foreground">Pipeline ID</div>
                  <div className="text-xs font-mono bg-muted p-2 rounded break-all">{pipelineId}</div>
                </div>
              </>
            )}
          </CardContent>
        </Card>

        {/* Tabs for Code and Logs */}
        <Card className="lg:col-span-2">
          <CardContent className="pt-6">
            <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
              <TabsList className="grid w-full grid-cols-3">
                <TabsTrigger value="overview" className="flex items-center">
                  <Activity className="h-4 w-4 mr-2" />
                  Overview
                </TabsTrigger>
                <TabsTrigger value="code" className="flex items-center">
                  <FileCode className="h-4 w-4 mr-2" />
                  Code
                </TabsTrigger>
                <TabsTrigger value="logs" className="flex items-center">
                  <Terminal className="h-4 w-4 mr-2" />
                  Logs
                </TabsTrigger>
              </TabsList>
              
              <TabsContent value="overview" className="space-y-4">
                <div className="space-y-4 pt-4">
                  <div>
                    <h3 className="text-lg font-semibold mb-2">Pipeline Overview</h3>
                    <p className="text-sm text-muted-foreground">
                      {pipeline?.description || "No description available"}
                    </p>
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <Card>
                      <CardHeader className="pb-3">
                        <CardTitle className="text-sm font-medium">Status</CardTitle>
                      </CardHeader>
                      <CardContent>
                        <div className="flex items-center space-x-2">
                          <StatusIcon className={`h-5 w-5 ${statusInfo.color}`} />
                          <span className="text-lg font-bold capitalize">{pipeline?.status}</span>
                        </div>
                      </CardContent>
                    </Card>

                    <Card>
                      <CardHeader className="pb-3">
                        <CardTitle className="text-sm font-medium">Branch</CardTitle>
                      </CardHeader>
                      <CardContent>
                        <div className="flex items-center space-x-2">
                          <GitBranch className="h-5 w-5" />
                          <span className="text-lg font-bold">{pipeline?.branch || "main"}</span>
                        </div>
                      </CardContent>
                    </Card>
                  </div>

                  {pipeline?.lastRun && (
                    <Card>
                      <CardHeader className="pb-3">
                        <CardTitle className="text-sm font-medium">Last Execution</CardTitle>
                      </CardHeader>
                      <CardContent className="space-y-2">
                        <div className="flex items-center justify-between text-sm">
                          <span className="text-muted-foreground">Last Run:</span>
                          <span className="font-medium">{new Date(pipeline.lastRun).toLocaleString()}</span>
                        </div>
                        {pipeline.duration && (
                          <div className="flex items-center justify-between text-sm">
                            <span className="text-muted-foreground">Duration:</span>
                            <span className="font-medium">{pipeline.duration}</span>
                          </div>
                        )}
                      </CardContent>
                    </Card>
                  )}

                  <div className="flex items-center space-x-2 pt-4">
                    {!isRunning ? (
                      <Button onClick={handleRunPipeline} disabled={running} className="flex-1">
                        {running ? (
                          <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                        ) : (
                          <Play className="mr-2 h-4 w-4" />
                        )}
                        {running ? "Starting Pipeline..." : "Run Pipeline"}
                      </Button>
                    ) : (
                      <Button variant="secondary" onClick={handleStopPipeline} className="flex-1">
                        <Pause className="mr-2 h-4 w-4" />
                        Stop Pipeline
                      </Button>
                    )}
                    <Button variant="outline" onClick={() => setActiveTab("logs")}>
                      <Terminal className="mr-2 h-4 w-4" />
                      View Logs
                    </Button>
                  </div>
                </div>
              </TabsContent>
              
              <TabsContent value="code" className="space-y-4">
                <div className="space-y-2 pt-4">
                  <Label htmlFor="code">Pipeline Code</Label>
                  {isEditMode ? (
                    <Textarea
                      id="code"
                      value={pipelineCode}
                      onChange={(e) => setPipelineCode(e.target.value)}
                      placeholder="Enter your pipeline code here..."
                      className="font-mono text-sm min-h-[500px] resize-y"
                    />
                  ) : (
                    <div className="border rounded-lg p-4 bg-muted">
                      <pre className="text-sm font-mono overflow-x-auto whitespace-pre-wrap">
                        {pipeline?.code || "No code available"}
                      </pre>
                    </div>
                  )}
                  {!isEditMode && (
                    <div className="flex justify-end">
                      <Button variant="outline" onClick={() => setIsEditMode(true)}>
                        <Edit className="mr-2 h-4 w-4" />
                        Edit Code
                      </Button>
                    </div>
                  )}
                </div>
              </TabsContent>
              
              <TabsContent value="logs" className="space-y-4">
                <div className="pt-4">
                  <div className="flex items-center justify-between mb-4">
                    <div>
                      <h3 className="text-lg font-semibold flex items-center">
                        <Terminal className="h-5 w-5 mr-2" />
                        Execution Logs
                      </h3>
                      <p className="text-sm text-muted-foreground">View pipeline execution output</p>
                    </div>
                    <Button variant="outline" size="sm" onClick={fetchPipelineLogs} disabled={loadingLogs}>
                      <RefreshCw className={`h-4 w-4 mr-2 ${loadingLogs ? 'animate-spin' : ''}`} />
                      Refresh
                    </Button>
                  </div>
                  <div className="space-y-3">
                    {logs.length === 0 ? (
                      <div className="text-center py-12 border rounded-lg bg-muted/30">
                        <Terminal className="h-12 w-12 mx-auto mb-2 text-muted-foreground opacity-50" />
                        <p className="text-sm text-muted-foreground">No execution logs yet</p>
                        <p className="text-xs mt-1 text-muted-foreground">Run the pipeline to see output logs here</p>
                      </div>
                    ) : (
                      logs.map((log, index) => (
                        <div
                          key={index}
                          className={`border rounded-lg p-4 ${
                            log.status === "error" ? "border-red-200 bg-red-50" : "border-gray-200 bg-white"
                          }`}
                        >
                          <div className="flex items-center justify-between mb-2">
                            <Badge className={log.status === "error" ? "bg-red-100 text-red-800" : "bg-green-100 text-green-800"}>
                              {log.status === "error" ? "Error" : "Success"}
                            </Badge>
                            <span className="text-xs text-muted-foreground flex items-center">
                              <Clock className="h-3 w-3 mr-1" />
                              {new Date(log.timestamp).toLocaleString()}
                            </span>
                          </div>
                          {log.output && (
                            <div className="mt-2">
                              <p className="text-xs font-semibold text-muted-foreground mb-1">Output:</p>
                              <pre className="text-xs font-mono bg-white p-3 rounded border overflow-x-auto whitespace-pre-wrap">
                                {log.output}
                              </pre>
                            </div>
                          )}
                          {log.error && (
                            <div className="mt-2">
                              <p className="text-xs font-semibold mb-1 text-red-600">Error:</p>
                              <pre className="text-xs font-mono bg-white p-3 rounded border border-red-300 overflow-x-auto whitespace-pre-wrap text-red-600">
                                {log.error}
                              </pre>
                            </div>
                          )}
                        </div>
                      ))
                    )}
                  </div>
                </div>
              </TabsContent>
            </Tabs>
          </CardContent>
        </Card>
      </div>

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
              onClick={() => setIsDeleteDialogOpen(false)}
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
