'use client'

import { useEffect, useState } from "react"
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
import { toast } from "sonner"
import client from "@/app/utility/post"
import {
  RefreshCw,
  Plus,
  Trash2,
  Edit,
  Power,
  Route,
  ArrowRight,
} from "lucide-react"

type GatewayRoute = {
  id: string
  pathPrefix: string
  targetURL: string
  description?: string
  createdAt: string
}

export default function Gateway() {
  const [routes, setRoutes] = useState<GatewayRoute[]>([])
  const [loading, setLoading] = useState(false)

  // Service enabled state
  const [serviceEnabled, setServiceEnabled] = useState<boolean | null>(null)
  const [enablingService, setEnablingService] = useState(false)

  // Create/Edit dialog state
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [selectedRoute, setSelectedRoute] = useState<GatewayRoute | null>(null)
  const [routeToDelete, setRouteToDelete] = useState<string | null>(null)

  // Form state
  const [pathPrefix, setPathPrefix] = useState("")
  const [targetURL, setTargetURL] = useState("")
  const [description, setDescription] = useState("")

  // Check if the gateway service is enabled
  const checkServiceStatus = async () => {
    try {
      const res = await client.get<{ service: string; enabled: boolean }>(
        "/get-service-status?service=gateway"
      )
      setServiceEnabled(res.data.enabled)
    } catch (err) {
      console.error("Failed to check service status:", err)
      setServiceEnabled(false)
    }
  }

  // Enable the gateway service
  const handleEnableService = async () => {
    setEnablingService(true)
    try {
      await client.post("/enable-service", { service: "gateway" })
      setServiceEnabled(true)
      fetchRoutes()
    } catch (err) {
      console.error("Failed to enable service:", err)
      toast.error("Failed to enable Gateway service")
    } finally {
      setEnablingService(false)
    }
  }

  // Fetch all gateway routes
  const fetchRoutes = async () => {
    setLoading(true)
    try {
      const res = await client.get<GatewayRoute[]>("/list-gateway-routes")
      setRoutes(res.data || [])
    } catch (err) {
      console.error("Failed to fetch gateway routes:", err)
      setRoutes([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    checkServiceStatus()
  }, [])

  useEffect(() => {
    if (serviceEnabled) {
      fetchRoutes()
    }
  }, [serviceEnabled])

  // Reset form fields
  const resetForm = () => {
    setPathPrefix("")
    setTargetURL("")
    setDescription("")
  }

  // Create a new route
  const handleCreateRoute = async () => {
    if (!pathPrefix || !targetURL) {
      toast.error("Path prefix and target URL are required")
      return
    }
    try {
      await client.post("/create-gateway-route", {
        pathPrefix,
        targetURL,
        description,
      })
      resetForm()
      setIsCreateDialogOpen(false)
      await fetchRoutes()
      toast.success("Gateway route created successfully")
    } catch (err) {
      console.error("Failed to create gateway route:", err)
      toast.error("Failed to create gateway route")
    }
  }

  // Open the edit dialog pre-populated with the selected route
  const openEditDialog = (route: GatewayRoute) => {
    setSelectedRoute(route)
    setPathPrefix(route.pathPrefix)
    setTargetURL(route.targetURL)
    setDescription(route.description || "")
    setIsEditDialogOpen(true)
  }

  // Update an existing route
  const handleUpdateRoute = async () => {
    if (!selectedRoute || !pathPrefix || !targetURL) {
      toast.error("Path prefix and target URL are required")
      return
    }
    try {
      await client.put(`/update-gateway-route/${selectedRoute.id}`, {
        pathPrefix,
        targetURL,
        description,
      })
      resetForm()
      setSelectedRoute(null)
      setIsEditDialogOpen(false)
      await fetchRoutes()
      toast.success("Gateway route updated successfully")
    } catch (err) {
      console.error("Failed to update gateway route:", err)
      toast.error("Failed to update gateway route")
    }
  }

  // Open the delete confirmation dialog
  const openDeleteDialog = (id: string) => {
    setRouteToDelete(id)
    setIsDeleteDialogOpen(true)
  }

  // Delete a route
  const handleDeleteRoute = async () => {
    if (!routeToDelete) return
    try {
      await client.delete(`/delete-gateway-route/${routeToDelete}`)
      toast.success("Gateway route deleted successfully")
      await fetchRoutes()
      setIsDeleteDialogOpen(false)
      setRouteToDelete(null)
    } catch (err) {
      console.error("Failed to delete gateway route:", err)
      toast.error("Failed to delete gateway route")
    }
  }

  // Loading state while checking service status
  if (serviceEnabled === null) {
    return (
      <DashboardShell>
        <div className="flex items-center justify-center h-64">
          <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      </DashboardShell>
    )
  }

  // Prompt to enable the service
  if (!serviceEnabled) {
    return (
      <DashboardShell>
        <DashboardHeader heading="Gateway" text="Route traffic to your services" />
        <div className="flex items-center justify-center min-h-[400px]">
          <Card className="max-w-md w-full">
            <CardHeader className="text-center">
              <div className="mx-auto p-3 rounded-full bg-blue-50 w-fit mb-4">
                <Route className="h-8 w-8 text-blue-600" />
              </div>
              <CardTitle>Enable Gateway Service</CardTitle>
              <CardDescription>
                The Gateway service is not yet enabled. Enable it to start routing
                traffic to your functions and containers.
              </CardDescription>
            </CardHeader>
            <CardContent className="flex justify-center">
              <Button onClick={handleEnableService} disabled={enablingService} size="lg">
                <Power className="mr-2 h-4 w-4" />
                {enablingService ? "Enabling..." : "Enable Gateway"}
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
        heading="Gateway"
        text="Manage routing rules that forward incoming requests to your services"
      >
        <div className="flex items-center space-x-2">
          <Button variant="outline" onClick={fetchRoutes} disabled={loading}>
            <RefreshCw className={`mr-2 h-4 w-4 ${loading ? "animate-spin" : ""}`} />
            {loading ? "Refreshing..." : "Refresh"}
          </Button>

          {/* Create Route Dialog */}
          <Dialog
            open={isCreateDialogOpen}
            onOpenChange={(open) => {
              setIsCreateDialogOpen(open)
              if (!open) resetForm()
            }}
          >
            <DialogTrigger asChild>
              <Button>
                <Plus className="mr-2 h-4 w-4" />
                New Route
              </Button>
            </DialogTrigger>
            <DialogContent className="max-w-lg">
              <DialogHeader>
                <DialogTitle>Create Gateway Route</DialogTitle>
                <DialogDescription>
                  Define a URL path prefix and the upstream service to forward
                  matching requests to.
                </DialogDescription>
              </DialogHeader>

              <div className="grid gap-4 py-4">
                <div className="grid gap-2">
                  <Label htmlFor="pathPrefix">Path Prefix *</Label>
                  <Input
                    id="pathPrefix"
                    placeholder="/my-app/"
                    value={pathPrefix}
                    onChange={(e) => setPathPrefix(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    Incoming requests whose URL starts with this prefix will be
                    forwarded to the target. Must begin with&nbsp;
                    <code className="rounded bg-muted px-1">/</code>.
                  </p>
                </div>

                <div className="grid gap-2">
                  <Label htmlFor="targetURL">Target URL *</Label>
                  <Input
                    id="targetURL"
                    placeholder="http://localhost:8080"
                    value={targetURL}
                    onChange={(e) => setTargetURL(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    The upstream service URL to proxy requests to (e.g. a
                    container port or external endpoint).
                  </p>
                </div>

                <div className="grid gap-2">
                  <Label htmlFor="description">Description</Label>
                  <Input
                    id="description"
                    placeholder="Optional description"
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                  />
                </div>
              </div>

              <DialogFooter>
                <Button
                  variant="outline"
                  onClick={() => {
                    setIsCreateDialogOpen(false)
                    resetForm()
                  }}
                >
                  Cancel
                </Button>
                <Button
                  onClick={handleCreateRoute}
                  disabled={!pathPrefix || !targetURL}
                >
                  <Plus className="mr-2 h-4 w-4" />
                  Create Route
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </DashboardHeader>

      {/* Summary card */}
      <div className="grid gap-6 md:grid-cols-1 lg:grid-cols-2 mb-6">
        <Card className="border-l-4 border-l-blue-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Routes</CardTitle>
            <Route className="h-4 w-4 text-blue-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{routes.length}</div>
            <p className="text-xs text-muted-foreground">Configured routing rules</p>
          </CardContent>
        </Card>
      </div>

      {/* Route list */}
      {routes.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12 text-center">
            <Route className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold mb-2">No routes configured</h3>
            <p className="text-muted-foreground mb-4">
              Create your first gateway route to start routing traffic to your
              services.
            </p>
            <Button onClick={() => setIsCreateDialogOpen(true)}>
              <Plus className="mr-2 h-4 w-4" />
              New Route
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4">
          {routes.map((route) => (
            <Card key={route.id}>
              <CardContent className="flex items-center justify-between py-4">
                <div className="flex items-center gap-4 min-w-0">
                  <div className="p-2 rounded-full bg-blue-50 shrink-0">
                    <Route className="h-5 w-5 text-blue-600" />
                  </div>
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <code className="rounded bg-muted px-1.5 py-0.5 text-sm font-mono">
                        {route.pathPrefix}
                      </code>
                      <ArrowRight className="h-4 w-4 text-muted-foreground shrink-0" />
                      <code className="rounded bg-muted px-1.5 py-0.5 text-sm font-mono truncate max-w-xs">
                        {route.targetURL}
                      </code>
                    </div>
                    {route.description && (
                      <p className="text-xs text-muted-foreground mt-1">
                        {route.description}
                      </p>
                    )}
                  </div>
                </div>

                <div className="flex items-center gap-2 shrink-0 ml-4">
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => openEditDialog(route)}
                    aria-label="Edit route"
                  >
                    <Edit className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => openDeleteDialog(route.id)}
                    aria-label="Delete route"
                  >
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Edit Route Dialog */}
      <Dialog
        open={isEditDialogOpen}
        onOpenChange={(open) => {
          setIsEditDialogOpen(open)
          if (!open) {
            resetForm()
            setSelectedRoute(null)
          }
        }}
      >
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Edit Gateway Route</DialogTitle>
            <DialogDescription>Update the routing rule configuration.</DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="editPathPrefix">Path Prefix *</Label>
              <Input
                id="editPathPrefix"
                placeholder="/my-app/"
                value={pathPrefix}
                onChange={(e) => setPathPrefix(e.target.value)}
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="editTargetURL">Target URL *</Label>
              <Input
                id="editTargetURL"
                placeholder="http://localhost:8080"
                value={targetURL}
                onChange={(e) => setTargetURL(e.target.value)}
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="editDescription">Description</Label>
              <Input
                id="editDescription"
                placeholder="Optional description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>
          </div>

          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setIsEditDialogOpen(false)
                resetForm()
                setSelectedRoute(null)
              }}
            >
              Cancel
            </Button>
            <Button
              onClick={handleUpdateRoute}
              disabled={!pathPrefix || !targetURL}
            >
              Save Changes
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Gateway Route</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete this route? This action cannot be
              undone and the nginx configuration will be updated immediately.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setIsDeleteDialogOpen(false)
                setRouteToDelete(null)
              }}
            >
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleDeleteRoute}>
              <Trash2 className="mr-2 h-4 w-4" />
              Delete Route
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </DashboardShell>
  )
}
