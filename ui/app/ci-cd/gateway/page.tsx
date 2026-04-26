'use client'

import { useEffect, useState, Suspense } from "react"
import { useSearchParams } from "next/navigation"
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
  Container,
  Zap,
  MousePointerClick,
} from "lucide-react"

// GatewayRoute mirrors the backend GatewayRouteEntry type.
type GatewayRoute = {
  id: string
  pathPrefix: string
  targetURL: string
  description?: string
  createdAt: string
}

// ContainerItem from the /get-containers endpoint.
type ContainerItem = {
  Id: string
  Names: string[]
  Image: string
  State: string
  Status: string
}

// ContainerDetail from the /get-container?id=... endpoint (includes ports).
type ContainerDetail = {
  id: string
  name: string
  image: string
  state: string
  ports: string[]
}

// FunctionItem from the /list-functions endpoint.
type FunctionItem = {
  id: string
  name: string
  runtime: string
  status: string
}

/** Parse the host port from a port-mapping string like "8080:80/tcp" or "0.0.0.0:8080:80/tcp". */
function extractHostPort(portStr: string): string | null {
  // Strip protocol suffix (e.g. /tcp, /udp)
  const clean = portStr.split("/")[0]
  const parts = clean.split(":")
  if (parts.length === 1) return null          // bare container port, no host binding
  if (parts.length === 2) return parts[0]      // "hostPort:containerPort"
  if (parts.length === 3) return parts[1]      // "hostIP:hostPort:containerPort"
  return null
}

export default function GatewayPage() {
  return (
    <Suspense>
      <GatewayPageInner />
    </Suspense>
  )
}

function GatewayPageInner() {
  const searchParams = useSearchParams()
  const [routes, setRoutes] = useState<GatewayRoute[]>([])
  const [isEnabled, setIsEnabled] = useState(false)
  const [isEnabling, setIsEnabling] = useState(false)
  const [isLoading, setIsLoading] = useState(true)

  // Services available to click-to-route
  const [containers, setContainers] = useState<ContainerItem[]>([])
  const [functions, setFunctions] = useState<FunctionItem[]>([])
  const [isFetchingServices, setIsFetchingServices] = useState(false)

  // Create / quick-route dialog state (shared)
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

  /** Fetch running containers and functions so the user can click-to-route. */
  const fetchServices = async () => {
    setIsFetchingServices(true)
    try {
      const [cRes, fRes] = await Promise.allSettled([
        client.get<ContainerItem[]>("/get-containers"),
        client.get<FunctionItem[]>("/list-functions"),
      ])
      if (cRes.status === "fulfilled") {
        // Only show running containers.
        setContainers((cRes.value.data ?? []).filter((c) => c.State?.toLowerCase() === "running"))
      }
      if (fRes.status === "fulfilled") {
        setFunctions(fRes.value.data ?? [])
      }
    } finally {
      setIsFetchingServices(false)
    }
  }

  useEffect(() => {
    fetchStatus()
    fetchRoutes()
    fetchServices()
  }, [])

  // If the page was opened with ?create=true (e.g. from the function trigger dropdown),
  // pre-fill the Create Route dialog and open it automatically.
  useEffect(() => {
    if (searchParams.get("create") === "true") {
      const pathPrefix = searchParams.get("pathPrefix") ?? ""
      const targetURL = searchParams.get("targetURL") ?? ""
      const description = searchParams.get("description") ?? ""
      setCreatePathPrefix(pathPrefix)
      setCreateTargetURL(targetURL)
      setCreateDescription(description)
      setCreateOpen(true)
    }
  }, [searchParams])

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

  /** Open the Create Route dialog pre-filled for a running container.
   *  Fetches the container detail to determine the host port. */
  const openRouteForContainer = async (container: ContainerItem) => {
    const name = (container.Names[0] ?? container.Id).replace(/^\//, "")
    // Fetch detail to get port info.
    let targetURL = ""
    try {
      const res = await client.get<ContainerDetail>(`/get-container?id=${container.Id}`)
      const detail = res.data
      if (detail?.ports && detail.ports.length > 0) {
        const hostPort = extractHostPort(detail.ports[0])
        if (hostPort) {
          targetURL = `http://localhost:${hostPort}`
        }
      }
    } catch {
      // If we can't fetch details, leave targetURL blank so the user fills it in.
    }
    setCreatePathPrefix(`/${name}`)
    setCreateTargetURL(targetURL)
    setCreateDescription(`Route to container: ${name}`)
    setCreateOpen(true)
  }

  /** Open the Create Route dialog pre-filled for a function. */
  const openRouteForFunction = (fn: FunctionItem) => {
    // Strip the file extension to get a clean path prefix (e.g. "hello.py" → "/hello").
    const baseName = fn.name.replace(/\.[^.]+$/, "")
    setCreatePathPrefix(`/${baseName}`)
    setCreateTargetURL(`http://localhost:3030/invoke-function?name=${encodeURIComponent(fn.name)}`)
    setCreateDescription(`Route to function: ${fn.name}`)
    setCreateOpen(true)
  }

  /** Reset and open a blank Create Route dialog. */
  const openBlankCreate = () => {
    setCreatePathPrefix("")
    setCreateTargetURL("")
    setCreateDescription("")
    setCreateOpen(true)
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

  // Helper: check whether a route already exists for a given pathPrefix
  const hasRoute = (prefix: string) => routes.some((r) => r.pathPrefix === prefix)

  return (
    <DashboardShell>
      <DashboardHeader
        heading="Gateway"
        text="Route incoming traffic to individual services or containers using path-prefix rules."
      >
        {isEnabled ? (
          <Button onClick={openBlankCreate}>
            <Plus className="mr-2 h-4 w-4" />
            New Route
          </Button>
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
                Enable the Gateway service to start routing traffic. Once enabled you can click any
                running container or function below to add a route in seconds.
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* ── Click-to-route: available services ── */}
      {isEnabled && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-lg font-semibold flex items-center gap-2">
                <MousePointerClick className="h-5 w-5 text-primary" />
                Click a Service to Route
              </h2>
              <p className="text-sm text-muted-foreground">
                Select a running container or function to automatically create a gateway route for it.
              </p>
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={fetchServices}
              disabled={isFetchingServices}
            >
              <RefreshCw className={`h-4 w-4 ${isFetchingServices ? "animate-spin" : ""}`} />
              <span className="ml-2">Refresh</span>
            </Button>
          </div>

          {/* Containers */}
          <div>
            <h3 className="mb-2 text-sm font-medium text-muted-foreground flex items-center gap-1">
              <Container className="h-4 w-4" /> Running Containers
            </h3>
            {isFetchingServices ? (
              <p className="text-sm text-muted-foreground">Loading containers…</p>
            ) : containers.length === 0 ? (
              <p className="text-sm text-muted-foreground">No running containers found.</p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {containers.map((c) => {
                  const name = (c.Names[0] ?? c.Id).replace(/^\//, "")
                  const prefix = `/${name}`
                  const alreadyRouted = hasRoute(prefix)
                  return (
                    <Button
                      key={c.Id}
                      variant={alreadyRouted ? "secondary" : "outline"}
                      size="sm"
                      disabled={alreadyRouted}
                      onClick={() => openRouteForContainer(c)}
                      title={alreadyRouted ? `Route already exists for ${prefix}` : `Add route for ${name}`}
                      className="gap-2"
                    >
                      <Container className="h-4 w-4 shrink-0" />
                      <span>{name}</span>
                      {alreadyRouted && (
                        <Badge variant="outline" className="ml-1 text-xs">routed</Badge>
                      )}
                    </Button>
                  )
                })}
              </div>
            )}
          </div>

          {/* Functions */}
          <div>
            <h3 className="mb-2 text-sm font-medium text-muted-foreground flex items-center gap-1">
              <Zap className="h-4 w-4" /> Functions
            </h3>
            {isFetchingServices ? (
              <p className="text-sm text-muted-foreground">Loading functions…</p>
            ) : functions.length === 0 ? (
              <p className="text-sm text-muted-foreground">No functions found.</p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {functions.map((fn) => {
                  const baseName = fn.name.replace(/\.[^.]+$/, "")
                  const prefix = `/${baseName}`
                  const alreadyRouted = hasRoute(prefix)
                  return (
                    <Button
                      key={fn.id}
                      variant={alreadyRouted ? "secondary" : "outline"}
                      size="sm"
                      disabled={alreadyRouted}
                      onClick={() => openRouteForFunction(fn)}
                      title={alreadyRouted ? `Route already exists for ${prefix}` : `Add route for ${fn.name}`}
                      className="gap-2"
                    >
                      <Zap className="h-4 w-4 shrink-0" />
                      <span>{fn.name}</span>
                      {alreadyRouted && (
                        <Badge variant="outline" className="ml-1 text-xs">routed</Badge>
                      )}
                    </Button>
                  )
                })}
              </div>
            )}
          </div>
        </div>
      )}

      {/* ── Active routes list ── */}
      {isEnabled && (
        <>
          <div className="flex items-center justify-between">
            <p className="text-sm text-muted-foreground">
              {routes.length === 0
                ? "No routes configured. Click a service above or create one manually."
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
                    Click a service above, or create a route manually.
                  </p>
                  <Button onClick={openBlankCreate}>
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

      {/* ── Create / quick-route dialog ── */}
      <Dialog open={createOpen} onOpenChange={(open) => {
        setCreateOpen(open)
        if (!open) {
          setCreatePathPrefix("")
          setCreateTargetURL("")
          setCreateDescription("")
        }
      }}>
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

      {/* ── Edit dialog ── */}
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

      {/* ── Delete confirmation dialog ── */}
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

