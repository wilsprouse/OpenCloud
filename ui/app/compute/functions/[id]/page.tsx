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
  Terminal
} from "lucide-react"

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
  
  // Editable form state
  const [name, setName] = useState("")
  const [runtime, setRuntime] = useState("python")
  const [code, setCode] = useState("")
  const [memorySize, setMemorySize] = useState("128")
  const [timeout, setTimeout] = useState("3")
  
  // Trigger state
  const [triggerEnabled, setTriggerEnabled] = useState(false)
  const [triggerSchedule, setTriggerSchedule] = useState("0 0 * * *")
  
  // Output state
  const [functionOutput, setFunctionOutput] = useState<string>("")
  const [invoking, setInvoking] = useState(false)

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

  useEffect(() => {
    fetchFunctionDetails()
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
    setInvoking(true)
    setFunctionOutput("") // Clear previous output
    try {
      const res = await client.post(`/invoke-function?name=${encodeURIComponent(functionId)}`)
      console.log("Function invoked successfully")
      
      // Extract and display the output
      if (res.data && res.data.output !== undefined) {
        setFunctionOutput(res.data.output)
      } else {
        setFunctionOutput("Function executed successfully with no output")
      }
      
      fetchFunctionDetails() // Refresh to update invocation count
    } catch (err: any) {
      console.error("Failed to invoke function:", err)
      // Display error message in output
      const errorMessage = err.response?.data || err.message || "Failed to invoke function"
      setFunctionOutput(`Error: ${typeof errorMessage === 'string' ? errorMessage : JSON.stringify(errorMessage)}`)
    } finally {
      setInvoking(false)
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
          <Button variant="outline" onClick={handleInvoke} disabled={invoking}>
            <Play className={`mr-2 h-4 w-4 ${invoking ? 'animate-pulse' : ''}`} />
            {invoking ? "Invoking..." : "Invoke"}
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

        {/* Code Editor Section */}
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle className="flex items-center">
              <Code className="h-5 w-5 mr-2" />
              Function Code
            </CardTitle>
            <CardDescription>Edit your function code below</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <Label htmlFor="code">Code</Label>
              <Textarea
                id="code"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                placeholder="Enter your function code here..."
                className="font-mono text-sm min-h-[500px] resize-y"
              />
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Output Section */}
      {functionOutput && (
        <Card className="mt-6">
          <CardHeader>
            <CardTitle className="flex items-center">
              <Terminal className="h-5 w-5 mr-2" />
              Function Output
            </CardTitle>
            <CardDescription>Latest execution result</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="bg-slate-950 text-slate-50 rounded-md p-4 overflow-x-auto">
              <pre className="font-mono text-sm whitespace-pre-wrap break-words">
                {functionOutput}
              </pre>
            </div>
          </CardContent>
        </Card>
      )}
    </DashboardShell>
  )
}
