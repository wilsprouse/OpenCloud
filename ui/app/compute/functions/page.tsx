'use client'

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
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
import { Textarea } from "@/components/ui/textarea"
import { 
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Badge } from "@/components/ui/badge"
import client from "@/app/utility/post"
import { 
  RefreshCw, 
  Search,
  Plus,
  Zap,
  Clock,
  Activity,
  Play,
  Edit,
  Trash2,
  Code,
  Calendar
} from "lucide-react"

type FunctionItem = {
  id: string
  name: string
  runtime: string
  status: "active" | "inactive" | "error"
  lastModified: string
  invocations: number
  memorySize: number
  timeout: number
  trigger?: {
    type: string
    schedule: string
    enabled: boolean
  }
}

export default function FunctionsPage() {
  const router = useRouter()
  const [functions, setFunctions] = useState<FunctionItem[]>([])
  const [loading, setLoading] = useState(false)
  const [searchTerm, setSearchTerm] = useState("")
  const [isFunctionDialogOpen, setIsFunctionDialogOpen] = useState(false)
  
  // Function form state
  const [functionName, setFunctionName] = useState<string>("")
  const [functionRuntime, setFunctionRuntime] = useState<string>("python")
  const [functionCode, setFunctionCode] = useState<string>("")

  // Fetch functions
  const fetchFunctions = async () => {
    setLoading(true)
    try {
      const res = await client.get<FunctionItem[]>("/list-functions")
      setFunctions(res.data || [])
    } catch (err) {
      console.error("Failed to fetch functions:", err)
      // Set empty array on error
      setFunctions([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchFunctions()
  }, [])

  const handleCreateFunction = async () => {
    // TODO: Implement this in the backend
    try {
      console.log(`Creating function: ${functionName}`)
      const res = await client.post("/create-function", { 
        name: functionName,
        runtime: functionRuntime,
        code: functionCode,
      })

      if (res.status === 200 || res.status === 201) {
        setIsFunctionDialogOpen(false)
        setFunctionName("")
        setFunctionRuntime("nodejs20.x")
        setFunctionCode("")
        fetchFunctions()
      }
    } catch (err) {
      console.error("Failed to create function:", err)
    }
  }

  const handleInvokeFunction = async (id: string) => {
    try {
      await client.post(`/invoke-function?name=${id}`)
      fetchFunctions()
    } catch (err) {
      console.error("Failed to invoke function:", err)
    }
  }

  const handleDeleteFunction = async (id: string) => {
    // TODO: Implement this in the backend
    try {
      await client.delete(`/delete-function?name=${id}`)
      fetchFunctions()
    } catch (err) {
      console.error("Failed to delete function:", err)
    }
  }

  const handleFunctionClick = (id: string) => {
    router.push(`/compute/functions/${encodeURIComponent(id)}`)
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

  // Filter functions based on search
  const filteredFunctions = functions.filter(fn => 
    fn.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    fn.runtime.toLowerCase().includes(searchTerm.toLowerCase())
  )

  // Calculate statistics
  const totalFunctions = functions.length
  const activeFunctions = functions.filter(fn => fn.status === "active").length
  const totalInvocations = functions.reduce((sum, fn) => sum + fn.invocations, 0)

  return (
    <DashboardShell>
      <DashboardHeader heading="Functions" text="Compute scripts">
        <div className="flex items-center space-x-2">
          <Button variant="outline" onClick={fetchFunctions} disabled={loading}>
            <RefreshCw className={`mr-2 h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
          <Dialog open={isFunctionDialogOpen} onOpenChange={setIsFunctionDialogOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="mr-2 h-4 w-4" />
                Create Function
              </Button>
            </DialogTrigger>
            <DialogContent className="max-w-2xl">
              <DialogHeader>
                <DialogTitle>Create New Function</DialogTitle>
                <DialogDescription>
                  Create a new serverless function. Choose your runtime and configure the function settings.
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="function-name">Function Name</Label>
                  <Input
                    id="function-name"
                    placeholder="my-function"
                    value={functionName}
                    onChange={(e) => setFunctionName(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="function-runtime">Language</Label>
                  <Select value={functionRuntime} onValueChange={setFunctionRuntime}>
                    <SelectTrigger id="function-runtime">
                      <SelectValue placeholder="Select Language" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="python">Python</SelectItem>
                      <SelectItem value="nodejs">Node.js (Coming Soon)</SelectItem>
                      <SelectItem value="java">Java 21 (Coming Soon)</SelectItem>
                      <SelectItem value="go">Go 1.x (Coming Soon)</SelectItem>
                      <SelectItem value="dotnet">.NET 8 (Coming Soon)</SelectItem>
                      <SelectItem value="ruby">Ruby 3.3 (Coming Soon)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="function-code">Function Code</Label>
                  <Textarea
                    id="function-code"
                    placeholder="Enter your function code here..."
                    value={functionCode}
                    onChange={(e) => setFunctionCode(e.target.value)}
                    className="font-mono text-sm min-h-[200px]"
                  />
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setIsFunctionDialogOpen(false)}>
                  Cancel
                </Button>
                <Button onClick={handleCreateFunction} disabled={!functionName || !functionCode}>
                  Create Function
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </DashboardHeader>

      {/* Statistics Cards */}
      <div className="grid gap-6 md:grid-cols-3">
        <Card className="border-l-4 border-l-green-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Functions</CardTitle>
            <Zap className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalFunctions}</div>
            <p className="text-xs text-muted-foreground">Deployed functions</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-blue-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Functions</CardTitle>
            <Activity className="h-4 w-4 text-blue-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{activeFunctions}</div>
            <p className="text-xs text-muted-foreground">Currently active</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-purple-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Invocations</CardTitle>
            <Clock className="h-4 w-4 text-purple-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalInvocations.toLocaleString()}</div>
            <p className="text-xs text-muted-foreground">Function calls</p>
          </CardContent>
        </Card>
      </div>

      {/* Main Functions List */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Functions</CardTitle>
              <CardDescription>Manage your functions</CardDescription>
            </div>
          </div>
          <div className="relative mt-4">
            <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
            <input
              type="text"
              placeholder="Search functions by name or runtime..."
              className="w-full pl-8 pr-4 py-2 border rounded-md bg-background"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
            />
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {filteredFunctions.map((fn) => (
              <div
                key={fn.id}
                className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
              >
                <div 
                  className="flex items-center space-x-4 flex-1 cursor-pointer"
                  onClick={() => handleFunctionClick(fn.id)}
                >
                  <div className="p-2 rounded-lg bg-green-50">
                    <Zap className="h-5 w-5 text-green-600" />
                  </div>
                  <div className="space-y-1 flex-1 min-w-0">
                    <div className="flex items-center space-x-2">
                      <h4 className="font-medium truncate">
                        {fn.name.replace(/\.[^/.]+$/, "")}
                      </h4>
                    </div>
                    <div className="flex items-center space-x-4 text-xs text-muted-foreground">
                      <span className="flex items-center">
                        <Code className="h-3 w-3 mr-1" />
                        {fn.runtime}
                      </span>
                      <span>•</span>
                      <span className="flex items-center">
                        <Activity className="h-3 w-3 mr-1" />
                        {fn.invocations.toLocaleString()} invocations
                      </span>
                      {fn.trigger && fn.trigger.enabled && (
                        <>
                          <span>•</span>
                          <span className="flex items-center text-blue-600">
                            <Clock className="h-3 w-3 mr-1" />
                            CRON: {fn.trigger.schedule}
                          </span>
                        </>
                      )}
                      <span>•</span>
                      <span className="flex items-center">
                        <Calendar className="h-3 w-3 mr-1" />
                        Modified: {formatDate(fn.lastModified)}
                      </span>
                    </div>
                  </div>
                </div>
                <div className="flex items-center space-x-2 ml-4">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={(e) => {
                      e.stopPropagation()
                      handleInvokeFunction(fn.id)
                    }}
                  >
                    <Play className="h-4 w-4 mr-1" />
                    Invoke
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={(e) => {
                      e.stopPropagation()
                      handleFunctionClick(fn.id)
                    }}
                  >
                    <Edit className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="destructive"
                    size="sm"
                    onClick={(e) => {
                      e.stopPropagation()
                      handleDeleteFunction(fn.id)
                    }}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            ))}
            {filteredFunctions.length === 0 && !loading && (
              <div className="text-center py-12">
                <Zap className="mx-auto h-12 w-12 text-muted-foreground" />
                <h3 className="mt-4 text-lg font-semibold">No functions found</h3>
                <p className="mt-2 text-sm text-muted-foreground">
                  {searchTerm ? "Try adjusting your search terms" : "Create your first function to get started"}
                </p>
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </DashboardShell>
  )
}
