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
  Settings
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
        timeout: parseInt(timeout)
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
      await client.post(`/invoke-function/${encodeURIComponent(functionId)}`)
      console.log("Function invoked successfully")
      fetchFunctionDetails() // Refresh to update invocation count
    } catch (err) {
      console.error("Failed to invoke function:", err)
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

      {/* Statistics Cards */}
      {functionData && (
        <div className="grid gap-6 md:grid-cols-3">
          <Card className="border-l-4 border-l-blue-500">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Total Invocations</CardTitle>
              <Activity className="h-4 w-4 text-blue-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{functionData.invocations.toLocaleString()}</div>
              <p className="text-xs text-muted-foreground">Function calls</p>
            </CardContent>
          </Card>

          <Card className="border-l-4 border-l-purple-500">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Memory Allocated</CardTitle>
              <Settings className="h-4 w-4 text-purple-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{memorySize} MB</div>
              <p className="text-xs text-muted-foreground">Memory size</p>
            </CardContent>
          </Card>

          <Card className="border-l-4 border-l-green-500">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Last Modified</CardTitle>
              <Calendar className="h-4 w-4 text-green-500" />
            </CardHeader>
            <CardContent>
              <div className="text-sm font-bold">{formatDate(functionData.lastModified)}</div>
              <p className="text-xs text-muted-foreground">Last update time</p>
            </CardContent>
          </Card>
        </div>
      )}

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
                  <SelectItem value="nodejs20.x">Node.js 20.x</SelectItem>
                  <SelectItem value="nodejs18.x">Node.js 18.x</SelectItem>
                  <SelectItem value="python3.12">Python 3.12</SelectItem>
                  <SelectItem value="python3.11">Python 3.11</SelectItem>
                  <SelectItem value="python3.10">Python 3.10</SelectItem>
                  <SelectItem value="python">Python</SelectItem>
                  <SelectItem value="java21">Java 21</SelectItem>
                  <SelectItem value="java17">Java 17</SelectItem>
                  <SelectItem value="go1.x">Go 1.x</SelectItem>
                  <SelectItem value="dotnet8">.NET 8</SelectItem>
                  <SelectItem value="ruby3.3">Ruby 3.3</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="memory">Memory (MB)</Label>
              <Select value={memorySize} onValueChange={setMemorySize}>
                <SelectTrigger id="memory">
                  <SelectValue placeholder="Select memory" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="128">128 MB</SelectItem>
                  <SelectItem value="256">256 MB</SelectItem>
                  <SelectItem value="512">512 MB</SelectItem>
                  <SelectItem value="1024">1024 MB</SelectItem>
                  <SelectItem value="2048">2048 MB</SelectItem>
                  <SelectItem value="4096">4096 MB</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="timeout">Timeout (seconds)</Label>
              <Select value={timeout} onValueChange={setTimeout}>
                <SelectTrigger id="timeout">
                  <SelectValue placeholder="Select timeout" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="3">3 seconds</SelectItem>
                  <SelectItem value="5">5 seconds</SelectItem>
                  <SelectItem value="10">10 seconds</SelectItem>
                  <SelectItem value="30">30 seconds</SelectItem>
                  <SelectItem value="60">60 seconds</SelectItem>
                  <SelectItem value="300">300 seconds</SelectItem>
                  <SelectItem value="900">900 seconds</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="pt-4">
              <div className="flex items-center space-x-2 text-sm text-muted-foreground">
                <Code className="h-4 w-4" />
                <span>Runtime: {runtime}</span>
              </div>
              <div className="flex items-center space-x-2 text-sm text-muted-foreground mt-2">
                <Clock className="h-4 w-4" />
                <span>Timeout: {timeout}s</span>
              </div>
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
    </DashboardShell>
  )
}
