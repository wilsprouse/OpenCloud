'use client'

import { useEffect, useState } from "react"
import axios from "axios"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { DashboardHeader } from "@/components/dashboard-header"
import { DashboardShell } from "@/components/dashboard-shell"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import client from "@/app/utility/post"
import { 
  RefreshCw, 
  Search,
  Container,
  Play,
  Square,
  Trash2,
  Activity,
  Package,
  Image as ImageIcon
} from "lucide-react"

type Container = {
  Id: string
  Names: string[]
  Image: string
  State: string
  Status: string
}

export default function ContainersPage() {
  const [containers, setContainers] = useState<Container[]>([])
  const [loading, setLoading] = useState(false)
  const [searchTerm, setSearchTerm] = useState("")

  // Fetch containers
  const fetchContainers = async () => {
    setLoading(true)
    try {
      const res = await client.get<Container[]>("/get-containers")
      setContainers(res.data)
    } catch (err) {
      console.error("Failed to fetch containers:", err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchContainers()
  }, [])

  // Manage container actions
  const handleAction = async (id: string, action: "start" | "stop" | "remove") => {
    try {
      if (action === "remove") {
        await axios.delete(`/api/containers/${id}`)
      } else {
        await axios.post(`/api/containers/${id}/${action}`)
      }
      fetchContainers() // refresh list
    } catch (err) {
      console.error(`Failed to ${action} container:`, err)
    }
  }

  // Filter containers based on search
  const filteredContainers = containers.filter(container => 
    container.Names?.[0]?.toLowerCase().includes(searchTerm.toLowerCase()) ||
    container.Image?.toLowerCase().includes(searchTerm.toLowerCase()) ||
    container.Id?.toLowerCase().includes(searchTerm.toLowerCase())
  )

  // Calculate statistics
  const totalContainers = containers.length
  const runningContainers = containers.filter(c => c.State === "running").length
  const stoppedContainers = containers.filter(c => c.State !== "running").length

  return (
    <DashboardShell>
      <DashboardHeader heading="Containers" text="Manage your Docker containers">
        <div className="flex items-center space-x-2">
          <Button variant="outline" onClick={fetchContainers} disabled={loading}>
            <RefreshCw className={`mr-2 h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
        </div>
      </DashboardHeader>

      {/* Statistics Cards */}
      <div className="grid gap-6 md:grid-cols-3">
        <Card className="border-l-4 border-l-blue-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Containers</CardTitle>
            <Package className="h-4 w-4 text-blue-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalContainers}</div>
            <p className="text-xs text-muted-foreground">All containers</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-green-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Running</CardTitle>
            <Activity className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{runningContainers}</div>
            <p className="text-xs text-muted-foreground">Active containers</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-gray-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Stopped</CardTitle>
            <Square className="h-4 w-4 text-gray-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stoppedContainers}</div>
            <p className="text-xs text-muted-foreground">Inactive containers</p>
          </CardContent>
        </Card>
      </div>

      {/* Main Container List */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Containers</CardTitle>
              <CardDescription>View and manage your Docker containers</CardDescription>
            </div>
          </div>
          <div className="relative mt-4">
            <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              type="text"
              placeholder="Search containers by name, image, or ID..."
              className="pl-8"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
            />
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {filteredContainers.map((c) => (
              <div
                key={c.Id}
                className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
              >
                <div className="flex items-center space-x-4 flex-1">
                  <div className={`p-2 rounded-lg ${c.State === "running" ? "bg-green-50" : "bg-gray-50"}`}>
                    <Container className={`h-5 w-5 ${c.State === "running" ? "text-green-600" : "text-gray-600"}`} />
                  </div>
                  <div className="space-y-1 flex-1 min-w-0">
                    <div className="flex items-center space-x-2">
                      <h4 className="font-medium truncate">
                        {c.Names?.[0]?.replace(/^\//, "") || "Unnamed"}
                      </h4>
                      <Badge variant={c.State === "running" ? "default" : "secondary"}>
                        {c.State}
                      </Badge>
                    </div>
                    <div className="flex items-center space-x-4 text-xs text-muted-foreground">
                      <span className="flex items-center">
                        <ImageIcon className="h-3 w-3 mr-1" />
                        {c.Image}
                      </span>
                      <span>•</span>
                      <span className="flex items-center">
                        ID: {c.Id.slice(7, 19)}
                      </span>
                      <span>•</span>
                      <span>{c.Status}</span>
                    </div>
                  </div>
                </div>
                <div className="flex items-center space-x-2 ml-4">
                  {c.State !== "running" && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleAction(c.Id, "start")}
                    >
                      <Play className="h-4 w-4 mr-1" />
                      Start
                    </Button>
                  )}
                  {c.State === "running" && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleAction(c.Id, "stop")}
                    >
                      <Square className="h-4 w-4 mr-1" />
                      Stop
                    </Button>
                  )}
                  <Button
                    variant="destructive"
                    size="sm"
                    onClick={() => handleAction(c.Id, "remove")}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            ))}
            {filteredContainers.length === 0 && !loading && (
              <div className="text-center py-12">
                <Container className="mx-auto h-12 w-12 text-muted-foreground" />
                <h3 className="mt-4 text-lg font-semibold">No containers found</h3>
                <p className="mt-2 text-sm text-muted-foreground">
                  {searchTerm ? "Try adjusting your search terms" : "No Docker containers are currently available"}
                </p>
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </DashboardShell>
  )
}
