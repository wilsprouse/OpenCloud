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
  Globe,
  Plus,
  Trash2,
  Edit,
  RefreshCw,
  ArrowRight,
  Power,
  Network,
} from "lucide-react"

// GatewayRoute mirrors the backend GatewayRouteEntry type.
type GatewayRoute = {
  id: string
  pathPrefix: string
  targetURL: string
  description?: string
  createdAt: string
}

export default function GatewayPage() {
  const [routes, setRoutes] = useState<GatewayRoute[]>([])
  const [isEnabled, setIsEnabled] = useState(false)
  const [isEnabling, setIsEnabling] = useState(false)
  const [isLoading, setIsLoading] = useState(true)

  // Create dialog state
  const [createOpen, setCreateOpen] = useState(false)
  const [createPathPrefix, setCreatePathPrefix] = useState("")
  const [createTargetURL, setCreateTargetURL] = useState("")
  const [createDescription, setCreateDescription] = useState("")
  const [isCreating, setIsCreating] = useState(false)

  // Edit dialog state
  const [editOpen, setEditOpen] = useState(false)
  const [editRoute, setEditRoute] = useState<GatewayRoute | null>(null)
  const [editPathPrefix, setEditPathPrefix] = useState("")
  const [editTargetURL, setEditTargetURL] = useState("")
  const [editDescription, setEditDescription] = useState("")
  const [isSavingEdit, setIsSavingEdit] = useState(false)

  // Delete confirmation state
  const [deleteRoute, setDeleteRoute] = useState<GatewayRoute | null>(null)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)

  /** Check whether the gateway service is enabled. */
  const fetchStatus = async () => {
    try {
      const res = await client.get<{ enabled: boolean }>("/get-service-status?service=gateway")
      setIsEnabled(res.data?.enabled ?? false)
    } catch {
      setIsEnabled(false)
    }
  }

  /** Fetch all gateway routes from the backend. */
  const fetchRoutes = async () => {
    setIsLoading(true)
    try {
      const res = await client.get<GatewayRoute[]>("/list-gateway-routes")
      setRoutes(res.data ?? [])
    } catch {
      setRoutes([])
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    fetchStatus()
    fetchRoutes()
  }, [])

  /** Enable the gateway service via the streaming enable endpoint. */
  const handleEnable = async () => {
    setIsEnabling(true)
    try {
      await client.post("/enable-service", { service: "gateway" })
      toast.success("Gateway service enabled")
      setIsEnabled(true)
      fetchRoutes()
    } catch {
      toast.error("Failed to enable Gateway service")
    } finally {
      setIsEnabling(false)
    }
  }

  /** Create a new gateway route. */
  const handleCreate = async () => {
    if (!createPathPrefix || !createTargetURL) {
      toast.error("Path prefix and target URL are required")
      return
    }
    if (!createPathPrefix.startsWith("/")) {
      toast.error("Path prefix must start with '/'")
      return
    }
    setIsCreating(true)
    try {
      await client.post("/create-gateway-route", {
        pathPrefix: createPathPrefix,
        targetURL: createTargetURL,
        description: createDescription,
      })
      toast.success("Route created successfully")
      setCreateOpen(false)
      setCreatePathPrefix("")
      setCreateTargetURL("")
      setCreateDescription("")
      fetchRoutes()
    } catch {
      toast.error("Failed to create gateway route")
    } finally {
      setIsCreating(false)
    }
  }

  /** Open the edit dialog pre-populated with the selected route. */
  const openEdit = (route: GatewayRoute) => {
    setEditRoute(route)
    setEditPathPrefix(route.pathPrefix)
    setEditTargetURL(route.targetURL)
    setEditDescription(route.description ?? "")
    setEditOpen(true)
  }

  /** Save edits to an existing gateway route. */
  const handleSaveEdit = async () => {
    if (!editRoute) return
    if (!editPathPrefix || !editTargetURL) {
      toast.error("Path prefix and target URL are required")
      return
    }
    if (!editPathPrefix.startsWith("/")) {
      toast.error("Path prefix must start with '/'")
      return
    }
    setIsSavingEdit(true)
    try {
      await client.put(`/update-gateway-route/${editRoute.id}`, {
        pathPrefix: editPathPrefix,
        targetURL: editTargetURL,
        description: editDescription,
      })
      toast.success("Route updated successfully")
      setEditOpen(false)
      setEditRoute(null)
      fetchRoutes()
    } catch {
      toast.error("Failed to update gateway route")
    } finally {
      setIsSavingEdit(false)
    }
  }

  /** Delete a gateway route. */
  const handleDelete = async () => {
    if (!deleteRoute) return
    setIsDeleting(true)
    try {
      await client.delete(`/delete-gateway-route/${deleteRoute.id}`)
      toast.success("Route deleted")
      setDeleteOpen(false)
      setDeleteRoute(null)
      fetchRoutes()
    } catch {
      toast.error("Failed to delete gateway route")
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <DashboardShell>
      <DashboardHeader
        heading="Gateway"
        text="Route incoming traffic to individual services or containers using path-prefix rules."
      >
        {isEnabled ? (
          <Dialog open={createOpen} onOpenChange={setCreateOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="mr-2 h-4 w-4" />
                New Route
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create Gateway Route</DialogTitle>
                <DialogDescription>
                  Map an incoming URL path prefix to a target service URL.
                </DialogDescription>
              </DialogHeader>
              <div className="grid gap-4 py-4">
                <div className="grid gap-2">
                  <Label htmlFor="pathPrefix">Path Prefix</Label>
                  <Input
                    id="pathPrefix"
                    placeholder="/my-service"
                    value={createPathPrefix}
                    onChange={(e) => setCreatePathPrefix(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    Must start with &apos;/&apos;. Requests matching this prefix will be forwarded.
                  </p>
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="targetURL">Target URL</Label>
                  <Input
                    id="targetURL"
                    placeholder="http://localhost:8080"
                    value={createTargetURL}
                    onChange={(e) => setCreateTargetURL(e.target.value)}
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="description">Description (optional)</Label>
                  <Textarea
                    id="description"
                    placeholder="Brief description of this route"
                    value={createDescription}
                    onChange={(e) => setCreateDescription(e.target.value)}
                  />
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setCreateOpen(false)}>
                  Cancel
                </Button>
                <Button onClick={handleCreate} disabled={isCreating}>
                  {isCreating ? "Creating..." : "Create Route"}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        ) : (
          <Button onClick={handleEnable} disabled={isEnabling}>
            <Power className="mr-2 h-4 w-4" />
            {isEnabling ? "Enabling..." : "Enable Gateway"}
          </Button>
        )}
      </DashboardHeader>

      {/* Service not enabled banner */}
      {!isEnabled && (
        <Card className="border-yellow-300 bg-yellow-50 dark:bg-yellow-900/20">
          <CardContent className="flex items-center gap-4 pt-6">
            <Network className="h-8 w-8 text-yellow-600 shrink-0" />
            <div>
              <p className="font-semibold text-yellow-800 dark:text-yellow-200">
                Gateway service is not enabled
              </p>
              <p className="text-sm text-yellow-700 dark:text-yellow-300">
                Enable the Gateway service to start routing traffic. Once enabled you can define path-prefix
                rules that proxy requests to your running services or containers.
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Routes list */}
      {isEnabled && (
        <>
          <div className="flex items-center justify-between">
            <p className="text-sm text-muted-foreground">
              {routes.length === 0
                ? "No routes configured. Create one to get started."
                : `${routes.length} route${routes.length !== 1 ? "s" : ""} configured`}
            </p>
            <Button variant="ghost" size="sm" onClick={fetchRoutes} disabled={isLoading}>
              <RefreshCw className={`h-4 w-4 ${isLoading ? "animate-spin" : ""}`} />
              <span className="ml-2">Refresh</span>
            </Button>
          </div>

          <div className="grid gap-4">
            {isLoading ? (
              <Card>
                <CardContent className="pt-6 text-center text-muted-foreground">
                  Loading routes…
                </CardContent>
              </Card>
            ) : routes.length === 0 ? (
              <Card className="border-dashed">
                <CardContent className="flex flex-col items-center justify-center py-12 text-center">
                  <Globe className="mb-4 h-12 w-12 text-muted-foreground" />
                  <h3 className="mb-2 text-lg font-semibold">No routes yet</h3>
                  <p className="mb-4 text-sm text-muted-foreground">
                    Add your first gateway route to start routing traffic.
                  </p>
                  <Button onClick={() => setCreateOpen(true)}>
                    <Plus className="mr-2 h-4 w-4" />
                    Create Route
                  </Button>
                </CardContent>
              </Card>
            ) : (
              routes.map((route) => (
                <Card key={route.id}>
                  <CardHeader className="flex flex-row items-start justify-between space-y-0 pb-2">
                    <div className="space-y-1">
                      <CardTitle className="flex items-center gap-2 text-base font-mono">
                        <Badge variant="secondary" className="font-mono text-sm">
                          {route.pathPrefix}
                        </Badge>
                        <ArrowRight className="h-4 w-4 text-muted-foreground" />
                        <span className="text-sm text-muted-foreground break-all">
                          {route.targetURL}
                        </span>
                      </CardTitle>
                      {route.description && (
                        <CardDescription>{route.description}</CardDescription>
                      )}
                    </div>
                    <div className="flex gap-2 shrink-0">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => openEdit(route)}
                        title="Edit route"
                      >
                        <Edit className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-destructive hover:text-destructive"
                        onClick={() => {
                          setDeleteRoute(route)
                          setDeleteOpen(true)
                        }}
                        title="Delete route"
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </CardHeader>
                  <CardContent>
                    <p className="text-xs text-muted-foreground">
                      Created: {new Date(route.createdAt).toLocaleString()}
                    </p>
                  </CardContent>
                </Card>
              ))
            )}
          </div>
        </>
      )}

      {/* Edit dialog */}
      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit Gateway Route</DialogTitle>
            <DialogDescription>Update the path prefix or target URL for this route.</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="editPathPrefix">Path Prefix</Label>
              <Input
                id="editPathPrefix"
                value={editPathPrefix}
                onChange={(e) => setEditPathPrefix(e.target.value)}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="editTargetURL">Target URL</Label>
              <Input
                id="editTargetURL"
                value={editTargetURL}
                onChange={(e) => setEditTargetURL(e.target.value)}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="editDescription">Description (optional)</Label>
              <Textarea
                id="editDescription"
                value={editDescription}
                onChange={(e) => setEditDescription(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleSaveEdit} disabled={isSavingEdit}>
              {isSavingEdit ? "Saving..." : "Save Changes"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete confirmation dialog */}
      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Route</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete the route for{" "}
              <span className="font-mono font-semibold">{deleteRoute?.pathPrefix}</span>? This
              action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteOpen(false)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleDelete} disabled={isDeleting}>
              {isDeleting ? "Deleting..." : "Delete Route"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </DashboardShell>
  )
}
