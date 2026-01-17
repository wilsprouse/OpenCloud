'use client'

import { use, useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { DashboardHeader } from "@/components/dashboard-header"
import { DashboardShell } from "@/components/dashboard-shell"
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
import { Switch } from "@/components/ui/switch"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import client from "@/app/utility/post"
import { 
  ArrowLeft,
  Save,
  Play,
  RefreshCw,
  Zap,
  Code,
  Clock,
  Activity,
  Calendar,
  Settings,
  FileText,
  Terminal
} from "lucide-react"

type FunctionLog = {
  timestamp: string
  output: string
  error?: string
  status: "success" | "error"
}

type FunctionDetail = {
  id: string
  name: string
  runtime: string
  status: "active" | "inactive" | "error"
  lastModified: string
  invocations: number
  memorySize: number
  timeout: number
  code: string
  trigger?: {
    type: string
    schedule: string
    enabled: boolean
  }
}

export default function FunctionDetail({ params }: { params: Promise<{ id: string }> }) {
  const resolvedParams = use(params)
  const functionId = decodeURIComponent(resolvedParams.id)
  const router = useRouter()
  const [functionData, setFunctionData] = useState<FunctionDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [logs, setLogs] = useState<FunctionLog[]>([])
  const [loadingLogs, setLoadingLogs] = useState(false)
  
  // Editable form state
  const [name, setName] = useState("")
  const [runtime, setRuntime] = useState("python")
  const [code, setCode] = useState("")
  const [memorySize, setMemorySize] = useState("128")
  const [timeout, setTimeout] = useState("3")
  
  // Trigger state
  const [triggerEnabled, setTriggerEnabled] = useState(false)
  const [triggerSchedule, setTriggerSchedule] = useState("0 0 * * *")

  // Fetch function details
  const fetchFunctionDetails = async () => {
    setLoading(true)
    try {
      const res = await client.get<FunctionDetail>(`/get-function/${encodeURIComponent(functionId)}`)
      const data = res.data
      setFunctionData(data)
      
      // Populate form fields
      setName(data.name)
      setRuntime(data.runtime)
      setCode(data.code || "")
      setMemorySize(data.memorySize?.toString() || "128")
      setTimeout(data.timeout?.toString() || "3")
      
      // Populate trigger fields
      if (data.trigger) {
        setTriggerEnabled(data.trigger.enabled)
        setTriggerSchedule(data.trigger.schedule || "0 0 * * *")
      } else {
        setTriggerEnabled(false)
        setTriggerSchedule("0 0 * * *")
      }
    } catch (err) {
      console.error("Failed to fetch function details:", err)
    } finally {
      setLoading(false)
    }
  }

  // Fetch function logs
  const fetchFunctionLogs = async () => {
    setLoadingLogs(true)
    try {
      const res = await client.get<FunctionLog[]>(`/get-function-logs/${encodeURIComponent(functionId)}`)
      setLogs(res.data || [])
    } catch (err) {
      console.error("Failed to fetch function logs:", err)
      setLogs([])
    } finally {
      setLoadingLogs(false)
    }
  }

  useEffect(() => {
    fetchFunctionDetails()
    fetchFunctionLogs()
  }, [functionId])

  const handleSaveAndDeploy = async () => {
    setSaving(true)
    try {
      console.log(`Updating function: ${name}`)
      const res = await client.put(`/update-function/${encodeURIComponent(functionId)}`, {
        name,
        runtime,
        code,
        memorySize: parseInt(memorySize),
        timeout: parseInt(timeout),
        trigger: triggerEnabled ? {
          type: "cron",
          schedule: triggerSchedule,
          enabled: true
        } : null
      })

      if (res.status === 200 || res.status === 201) {
        console.log("Function updated successfully")
        fetchFunctionDetails() // Refresh to get latest data
      }
    } catch (err) {
      console.error("Failed to update function:", err)
    } finally {
      setSaving(false)
    }
  }

  const handleInvoke = async () => {
    try {
      await client.post(`/invoke-function?name=${encodeURIComponent(functionId)}`)
      console.log("Function invoked successfully")
      fetchFunctionDetails() // Refresh to update invocation count
      fetchFunctionLogs() // Refresh logs to show new output
    } catch (err) {
      console.error("Failed to invoke function:", err)
      // Still fetch logs in case there was an error logged
      fetchFunctionLogs()
    }
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

  // Get status badge color
  const getStatusColor = (status: string) => {
    switch (status) {
      case "active":
        return "bg-green-100 text-green-800"
      case "inactive":
        return "bg-gray-100 text-gray-800"
      case "error":
        return "bg-red-100 text-red-800"
      default:
        return "bg-gray-100 text-gray-800"
    }
  }

  // No need to reverse logs anymore since backend returns only the last execution
  const displayLogs = logs

  if (loading && !functionData) {
    return (
      <DashboardShell>
        <div className="flex items-center justify-center h-64">
          <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      </DashboardShell>
    )
  }

  if (!functionData && !loading) {
    return (
      <DashboardShell>
        <div className="text-center py-12">
          <Zap className="mx-auto h-12 w-12 text-muted-foreground" />
          <h3 className="mt-4 text-lg font-semibold">Function not found</h3>
          <p className="mt-2 text-sm text-muted-foreground">
            The function you're looking for doesn't exist.
          </p>
          <Button onClick={() => router.push('/compute/functions')} className="mt-4">
            Back to Functions
          </Button>
        </div>
      </DashboardShell>
    )
  }

  return (
    <DashboardShell>
      <DashboardHeader 
        heading={
          <div className="flex items-center space-x-2">
            <Button 
              variant="ghost" 
              size="icon"
              onClick={() => router.push('/compute/functions')}
              className="mr-2"
            >
              <ArrowLeft className="h-4 w-4" />
            </Button>
            <span>{name || functionId}</span>
            {functionData && (
              <Badge className={getStatusColor(functionData.status)}>
                {functionData.status}
              </Badge>
            )}
          </div>
        } 
        text="Edit function configuration and code"
      >
        <div className="flex items-center space-x-2">
          <Button variant="outline" onClick={handleInvoke}>
            <Play className="mr-2 h-4 w-4" />
            Invoke
          </Button>
          <Button onClick={handleSaveAndDeploy} disabled={saving}>
            <Save className="mr-2 h-4 w-4" />
            {saving ? "Deploying..." : "Save & Deploy"}
          </Button>
        </div>
      </DashboardHeader>

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Configuration Section */}
        <Card className="lg:col-span-1">
          <CardHeader>
            <CardTitle className="flex items-center">
              <Settings className="h-5 w-5 mr-2" />
              Configuration
            </CardTitle>
            <CardDescription>Function settings and parameters</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="function-name">Function Name</Label>
              <Input
                id="function-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="my-function"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="runtime">Runtime</Label>
              <Select value={runtime} onValueChange={setRuntime}>
                <SelectTrigger id="runtime">
                  <SelectValue placeholder="Select runtime" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="nodejs">Node.js</SelectItem>
                  <SelectItem value="python">Python</SelectItem>
                  <SelectItem value="java">Java</SelectItem>
                  <SelectItem value="go">Go</SelectItem>
                  <SelectItem value="dotnet">.NET</SelectItem>
                  <SelectItem value="ruby">Ruby</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="pt-4 border-t space-y-4">
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label htmlFor="trigger-enabled" className="text-base">
                    CRON Trigger
                  </Label>
                  <div className="text-sm text-muted-foreground">
                    Schedule automatic function execution
                  </div>
                </div>
                <Switch
                  id="trigger-enabled"
                  checked={triggerEnabled}
                  onCheckedChange={setTriggerEnabled}
                />
              </div>
              
              {triggerEnabled && (
                <div className="space-y-2">
                  <Label htmlFor="trigger-schedule">CRON Schedule</Label>
                  <Input
                    id="trigger-schedule"
                    value={triggerSchedule}
                    onChange={(e) => setTriggerSchedule(e.target.value)}
                    placeholder="0 0 * * *"
                  />
                  <p className="text-xs text-muted-foreground">
                    Example: "0 0 * * *" runs daily at midnight
                  </p>
                </div>
              )}
            </div>

            <div className="pt-4">
              <div className="flex items-center space-x-2 text-sm text-muted-foreground">
                <Code className="h-4 w-4" />
                <span>Runtime: {runtime}</span>
              </div>
              {/*<div className="flex items-center space-x-2 text-sm text-muted-foreground mt-2">
                <Clock className="h-4 w-4" />
                <span>Timeout: {timeout}s</span>
              </div>*/}
            </div>
          </CardContent>
        </Card>

        {/* Tabs for Code Editor and Logs */}
        <Card className="lg:col-span-2">
          <CardContent className="pt-6">
            <Tabs defaultValue="code" className="w-full">
              <TabsList className="grid w-full grid-cols-2">
                <TabsTrigger value="code" className="flex items-center">
                  <Code className="h-4 w-4 mr-2" />
                  Code Editor
                </TabsTrigger>
                <TabsTrigger value="logs" className="flex items-center">
                  <Terminal className="h-4 w-4 mr-2" />
                  Logs
                </TabsTrigger>
              </TabsList>
              
              <TabsContent value="code" className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="code">Function Code</Label>
                  <Textarea
                    id="code"
                    value={code}
                    onChange={(e) => setCode(e.target.value)}
                    placeholder="Enter your function code here..."
                    className="font-mono text-sm min-h-[500px] resize-y"
                  />
                </div>
              </TabsContent>
              
              <TabsContent value="logs" className="space-y-4">
                <div className="flex items-center justify-between mb-4">
                  <div>
                    <h3 className="text-lg font-semibold flex items-center">
                      <Terminal className="h-5 w-5 mr-2" />
                      Execution Logs
                    </h3>
                    <p className="text-sm text-muted-foreground">View the last function invocation output</p>
                  </div>
                  <Button variant="outline" size="sm" onClick={fetchFunctionLogs} disabled={loadingLogs}>
                    <RefreshCw className={`h-4 w-4 mr-2 ${loadingLogs ? 'animate-spin' : ''}`} />
                    Refresh
                  </Button>
                </div>
                <div className="space-y-3">
                  {logs.length === 0 ? (
                    <div className="text-center py-8 text-muted-foreground">
                      <FileText className="h-8 w-8 mx-auto mb-2 opacity-50" />
                      <p className="text-sm">No execution logs yet</p>
                      <p className="text-xs mt-1">Invoke the function to see output logs here</p>
                    </div>
                  ) : (
                    displayLogs.map((log, index) => (
                      <div
                        key={index}
                        className={`border rounded-lg p-4 ${
                          log.status === "error" ? "border-red-200 bg-red-50" : "border-black"
                        }`}
                      >
                        {log.status === "error" && (
                          <div className="mb-2">
                            <Badge className="bg-red-100 text-red-800">
                              Error
                            </Badge>
                          </div>
                        )}
                        {log.output && (
                          <div className="mt-2">
                            <div className="flex items-center justify-between mb-1">
                              <p className="text-xs font-semibold text-muted-foreground">Output:</p>
                              <span className="text-xs text-muted-foreground flex items-center">
                                <Clock className="h-3 w-3 mr-1" />
                                {new Date(log.timestamp).toLocaleString()}
                              </span>
                            </div>
                            <pre className="text-xs font-mono bg-white p-2 rounded border overflow-x-auto whitespace-pre-wrap">
                              {log.output}
                            </pre>
                          </div>
                        )}
                        {log.error && (
                          <div className="mt-2">
                            <p className="text-xs font-semibold mb-1 text-red-600">Error:</p>
                            <pre className="text-xs font-mono bg-white p-2 rounded border border-red-300 overflow-x-auto whitespace-pre-wrap text-red-600">
                              {log.error}
                            </pre>
                          </div>
                        )}
                      </div>
                    ))
                  )}
                </div>
              </TabsContent>
            </Tabs>
          </CardContent>
        </Card>
      </div>
    </DashboardShell>
  )
}
