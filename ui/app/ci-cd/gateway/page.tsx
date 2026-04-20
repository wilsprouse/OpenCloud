'use client'

import { useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
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
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { toast } from "sonner"
import client from "@/app/utility/post"
import {
  RefreshCw,
  Trash2,
  Edit,
  Power,
  Route,
  ArrowRight,
  Zap,
  Container,
  Plus,
} from "lucide-react"

// ─── Types ────────────────────────────────────────────────────────────────────

type GatewayRoute = {
  id: string
  pathPrefix: string
  targetURL: string
  description?: string
  serviceType?: string   // "function" | "container" | "custom"
  serviceName?: string
  createdAt: string
}

type FunctionItem = {
  id: string
  name: string
  runtime: string
  status: string
}

type ContainerItem = {
  Id: string
  Names: string[]
  Image: string
  State: string
}

type ContainerDetail = {
  id: string
  name: string
  ports: string[]   // "hostIP:hostPort:containerPort/proto" form
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

/** Extract the first numeric host port from a ports array, or return "". */
function firstHostPort(ports: string[]): string {
  for (const p of ports) {
    // Formats: "0.0.0.0:8080:80/tcp"  or  "8080:80"
    const parts = p.split(":")
    if (parts.length >= 2) {
      const candidate = parts[parts.length - 2].replace(/[^0-9]/g, "")
      if (candidate) return candidate
    }
  }
  return ""
}

/** Convert a file-like name to a safe URL slug (strip extension, kebab-case). */
function nameToSlug(name: string): string {
  return name
    .replace(/\.[^.]+$/, "")   // strip extension
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "")
}

// ─── Component ────────────────────────────────────────────────────────────────

export default function Gateway() {
  // Service enabled state
  const [serviceEnabled, setServiceEnabled] = useState<boolean | null>(null)
  const [enablingService, setEnablingService] = useState(false)

  // Data
  const [routes, setRoutes] = useState<GatewayRoute[]>([])
  const [functions, setFunctions] = useState<FunctionItem[]>([])
  const [containers, setContainers] = useState<ContainerItem[]>([])
  const [loading, setLoading] = useState(false)

  // Route configuration dialog
  const [isRouteDialogOpen, setIsRouteDialogOpen] = useState(false)
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [selectedRoute, setSelectedRoute] = useState<GatewayRoute | null>(null)
  const [routeToDelete, setRouteToDelete] = useState<string | null>(null)
  const [configuringServiceType, setConfiguringServiceType] = useState<string>("custom")
  const [configuringServiceName, setConfiguringServiceName] = useState<string>("")

  // Form fields
  const [pathPrefix, setPathPrefix] = useState("")
  const [targetURL, setTargetURL] = useState("")
  const [description, setDescription] = useState("")
  const [targetLoading, setTargetLoading] = useState(false)

  // ── Data fetching ──

  const checkServiceStatus = async () => {
    try {
      const res = await client.get<{ service: string; enabled: boolean }>(
        "/get-service-status?service=gateway"
      )
      setServiceEnabled(res.data.enabled)
    } catch {
      setServiceEnabled(false)
    }
  }

  const fetchAll = async () => {
    setLoading(true)
    try {
      const [routesRes, functionsRes, containersRes] = await Promise.all([
        client.get<GatewayRoute[]>("/list-gateway-routes").catch(() => ({ data: [] })),
        client.get<FunctionItem[]>("/list-functions").catch(() => ({ data: [] })),
        client.get<ContainerItem[]>("/get-containers").catch(() => ({ data: [] })),
      ])
      setRoutes(routesRes.data || [])
      setFunctions(functionsRes.data || [])
      setContainers(containersRes.data || [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    checkServiceStatus()
  }, [])

  useEffect(() => {
    if (serviceEnabled) fetchAll()
  }, [serviceEnabled])

  // ── Service enable ──

  const handleEnableService = async () => {
    setEnablingService(true)
    try {
      await client.post("/enable-service", { service: "gateway" })
      setServiceEnabled(true)
    } catch {
      toast.error("Failed to enable Gateway service")
    } finally {
      setEnablingService(false)
    }
  }

  // ── Route form helpers ──

  const resetForm = () => {
    setPathPrefix("")
    setTargetURL("")
    setDescription("")
    setConfiguringServiceType("custom")
    setConfiguringServiceName("")
  }

  /** Open the "configure route" dialog pre-filled for a function service. */
  const openRouteDialogForFunction = (fn: FunctionItem) => {
    const slug = nameToSlug(fn.name)
    setConfiguringServiceType("function")
    setConfiguringServiceName(fn.name)
    setPathPrefix(`/${slug}/`)
    // Functions are invoked via the OpenCloud backend
    setTargetURL(`http://localhost:3030/invoke-function?name=${fn.name}`)
    setDescription(`Routes traffic to function ${fn.name}`)
    setIsRouteDialogOpen(true)
  }

  /** Open the "configure route" dialog pre-filled for a container service. */
  const openRouteDialogForContainer = async (ctr: ContainerItem) => {
    const name = (ctr.Names[0] || ctr.Id).replace(/^\//, "")
    const slug = nameToSlug(name)
    setConfiguringServiceType("container")
    setConfiguringServiceName(name)
    setPathPrefix(`/${slug}/`)
    setDescription(`Routes traffic to container ${name}`)
    setTargetURL("")
    setIsRouteDialogOpen(true)

    // Fetch the container's port mappings to pre-fill the target URL.
    setTargetLoading(true)
    try {
      const res = await client.get<ContainerDetail>(`/get-container?id=${ctr.Id}`)
      const port = firstHostPort(res.data.ports || [])
      if (port) {
        setTargetURL(`http://localhost:${port}`)
      }
    } catch {
      // Not fatal — user can enter target URL manually.
    } finally {
      setTargetLoading(false)
    }
  }

  /** Open the "configure route" dialog for a fully manual custom route. */
  const openRouteDialogForCustom = () => {
    resetForm()
    setConfiguringServiceType("custom")
    setIsRouteDialogOpen(true)
  }

  // ── CRUD handlers ──

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
        serviceType: configuringServiceType,
        serviceName: configuringServiceName,
      })
      resetForm()
      setIsRouteDialogOpen(false)
      await fetchAll()
      toast.success("Gateway route created")
    } catch {
      toast.error("Failed to create gateway route")
    }
  }

  const openEditDialog = (route: GatewayRoute) => {
    setSelectedRoute(route)
    setPathPrefix(route.pathPrefix)
    setTargetURL(route.targetURL)
    setDescription(route.description || "")
    setConfiguringServiceType(route.serviceType || "custom")
    setConfiguringServiceName(route.serviceName || "")
    setIsEditDialogOpen(true)
  }

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
        serviceType: configuringServiceType,
        serviceName: configuringServiceName,
      })
      resetForm()
      setSelectedRoute(null)
      setIsEditDialogOpen(false)
      await fetchAll()
      toast.success("Gateway route updated")
    } catch {
      toast.error("Failed to update gateway route")
    }
  }

  const handleDeleteRoute = async () => {
    if (!routeToDelete) return
    try {
      await client.delete(`/delete-gateway-route/${routeToDelete}`)
      toast.success("Gateway route deleted")
      setIsDeleteDialogOpen(false)
      setRouteToDelete(null)
      await fetchAll()
    } catch {
      toast.error("Failed to delete gateway route")
    }
  }

  // ── Derived helpers ──

  /** Return the route that targets a given service, or undefined. */
  const routeForService = (type: string, name: string) =>
    routes.find((r) => r.serviceType === type && r.serviceName === name)

  // ─────────────────────────────────────────────────────────────────────────────
  // Render: loading / not enabled
  // ─────────────────────────────────────────────────────────────────────────────

  if (serviceEnabled === null) {
    return (
      <DashboardShell>
        <div className="flex items-center justify-center h-64">
          <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      </DashboardShell>
    )
  }

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

  // ─────────────────────────────────────────────────────────────────────────────
  // Render: main page
  // ─────────────────────────────────────────────────────────────────────────────

  return (
    <DashboardShell>
      <DashboardHeader
        heading="Gateway"
        text="Click a service below to configure routing, or add a custom route."
      >
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={fetchAll} disabled={loading}>
            <RefreshCw className={`mr-2 h-4 w-4 ${loading ? "animate-spin" : ""}`} />
            {loading ? "Refreshing…" : "Refresh"}
          </Button>
          <Button variant="outline" onClick={openRouteDialogForCustom}>
            <Plus className="mr-2 h-4 w-4" />
            Custom Route
          </Button>
        </div>
      </DashboardHeader>

      {/* ── Available services ─────────────────────────────────────────────── */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold mb-4">Available Services</h2>
        <Tabs defaultValue="functions">
          <TabsList className="mb-4">
            <TabsTrigger value="functions">
              <Zap className="mr-2 h-4 w-4" />
              Functions ({functions.length})
            </TabsTrigger>
            <TabsTrigger value="containers">
              <Container className="mr-2 h-4 w-4" />
              Containers ({containers.length})
            </TabsTrigger>
          </TabsList>

          {/* Functions tab */}
          <TabsContent value="functions">
            {functions.length === 0 ? (
              <Card>
                <CardContent className="py-8 text-center text-muted-foreground">
                  No functions deployed yet. Deploy a function first to route to it.
                </CardContent>
              </Card>
            ) : (
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {functions.map((fn) => {
                  const existing = routeForService("function", fn.name)
                  return (
                    <Card
                      key={fn.id}
                      className="flex flex-col justify-between"
                    >
                      <CardHeader className="pb-2">
                        <div className="flex items-start justify-between gap-2">
                          <div className="flex items-center gap-2">
                            <Zap className="h-4 w-4 text-yellow-500 shrink-0" />
                            <CardTitle className="text-sm font-medium break-all">
                              {fn.name}
                            </CardTitle>
                          </div>
                          {existing && (
                            <Badge variant="secondary" className="shrink-0 text-xs">
                              Routed
                            </Badge>
                          )}
                        </div>
                        <CardDescription className="text-xs capitalize">
                          {fn.runtime}
                        </CardDescription>
                      </CardHeader>
                      <CardContent className="pt-0">
                        {existing ? (
                          <div className="flex items-center gap-1 text-xs text-muted-foreground">
                            <code className="rounded bg-muted px-1">{existing.pathPrefix}</code>
                            <ArrowRight className="h-3 w-3" />
                            <span className="truncate">{existing.targetURL}</span>
                          </div>
                        ) : (
                          <Button
                            size="sm"
                            className="w-full"
                            onClick={() => openRouteDialogForFunction(fn)}
                          >
                            <Route className="mr-2 h-3 w-3" />
                            Route
                          </Button>
                        )}
                      </CardContent>
                    </Card>
                  )
                })}
              </div>
            )}
          </TabsContent>

          {/* Containers tab */}
          <TabsContent value="containers">
            {containers.length === 0 ? (
              <Card>
                <CardContent className="py-8 text-center text-muted-foreground">
                  No containers found. Start a container first to route to it.
                </CardContent>
              </Card>
            ) : (
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {containers.map((ctr) => {
                  const name = (ctr.Names[0] || ctr.Id).replace(/^\//, "")
                  const existing = routeForService("container", name)
                  const isRunning = ctr.State === "running"
                  return (
                    <Card
                      key={ctr.Id}
                      className="flex flex-col justify-between"
                    >
                      <CardHeader className="pb-2">
                        <div className="flex items-start justify-between gap-2">
                          <div className="flex items-center gap-2">
                            <Container className="h-4 w-4 text-blue-500 shrink-0" />
                            <CardTitle className="text-sm font-medium break-all">
                              {name}
                            </CardTitle>
                          </div>
                          <div className="flex items-center gap-1 shrink-0">
                            {existing && (
                              <Badge variant="secondary" className="text-xs">
                                Routed
                              </Badge>
                            )}
                            <Badge
                              variant={isRunning ? "default" : "outline"}
                              className="text-xs"
                            >
                              {ctr.State}
                            </Badge>
                          </div>
                        </div>
                        <CardDescription className="text-xs truncate">
                          {ctr.Image}
                        </CardDescription>
                      </CardHeader>
                      <CardContent className="pt-0">
                        {existing ? (
                          <div className="flex items-center gap-1 text-xs text-muted-foreground">
                            <code className="rounded bg-muted px-1">{existing.pathPrefix}</code>
                            <ArrowRight className="h-3 w-3" />
                            <span className="truncate">{existing.targetURL}</span>
                          </div>
                        ) : (
                          <Button
                            size="sm"
                            className="w-full"
                            disabled={!isRunning}
                            onClick={() => openRouteDialogForContainer(ctr)}
                          >
                            <Route className="mr-2 h-3 w-3" />
                            {isRunning ? "Route" : "Not Running"}
                          </Button>
                        )}
                      </CardContent>
                    </Card>
                  )
                })}
              </div>
            )}
          </TabsContent>
        </Tabs>
      </section>

      {/* ── Configured routes ──────────────────────────────────────────────── */}
      <section>
        <h2 className="text-lg font-semibold mb-4">
          Configured Routes ({routes.length})
        </h2>
        {routes.length === 0 ? (
          <Card>
            <CardContent className="py-8 text-center text-muted-foreground">
              No routes configured yet. Click a service above to create one.
            </CardContent>
          </Card>
        ) : (
          <div className="grid gap-3">
            {routes.map((route) => (
              <Card key={route.id}>
                <CardContent className="flex items-center justify-between py-4 gap-4">
                  <div className="flex items-center gap-3 min-w-0">
                    <div className="p-2 rounded-full bg-blue-50 shrink-0">
                      {route.serviceType === "function" ? (
                        <Zap className="h-4 w-4 text-yellow-500" />
                      ) : route.serviceType === "container" ? (
                        <Container className="h-4 w-4 text-blue-600" />
                      ) : (
                        <Route className="h-4 w-4 text-blue-600" />
                      )}
                    </div>
                    <div className="min-w-0">
                      {route.serviceName && (
                        <p className="text-xs font-medium text-muted-foreground mb-0.5 capitalize">
                          {route.serviceType}: {route.serviceName}
                        </p>
                      )}
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
                        <p className="text-xs text-muted-foreground mt-0.5">
                          {route.description}
                        </p>
                      )}
                    </div>
                  </div>

                  <div className="flex items-center gap-1 shrink-0">
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
                      onClick={() => {
                        setRouteToDelete(route.id)
                        setIsDeleteDialogOpen(true)
                      }}
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
      </section>

      {/* ── Create/Configure Route Dialog ──────────────────────────────────── */}
      <Dialog
        open={isRouteDialogOpen}
        onOpenChange={(open) => {
          setIsRouteDialogOpen(open)
          if (!open) resetForm()
        }}
      >
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {configuringServiceType === "function"
                ? `Route Function: ${configuringServiceName}`
                : configuringServiceType === "container"
                ? `Route Container: ${configuringServiceName}`
                : "Add Custom Route"}
            </DialogTitle>
            <DialogDescription>
              {configuringServiceType === "custom"
                ? "Specify a URL path prefix and the upstream target to proxy requests to."
                : "Review the suggested values and adjust if needed, then click Create Route."}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="pathPrefix">Path Prefix *</Label>
              <Input
                id="pathPrefix"
                placeholder="/my-service/"
                value={pathPrefix}
                onChange={(e) => setPathPrefix(e.target.value)}
              />
              <p className="text-xs text-muted-foreground">
                Requests whose URL starts with this prefix will be forwarded.
                Must begin with&nbsp;
                <code className="rounded bg-muted px-1">/</code>.
              </p>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="targetURL">Target URL *</Label>
              <div className="relative">
                <Input
                  id="targetURL"
                  placeholder={
                    targetLoading ? "Loading port info…" : "http://localhost:8080"
                  }
                  value={targetURL}
                  onChange={(e) => setTargetURL(e.target.value)}
                  disabled={targetLoading}
                />
                {targetLoading && (
                  <RefreshCw className="absolute right-3 top-2.5 h-4 w-4 animate-spin text-muted-foreground" />
                )}
              </div>
              {configuringServiceType === "container" && !targetURL && !targetLoading && (
                <p className="text-xs text-amber-600">
                  No port mapping found. Enter the host port manually
                  (e.g.&nbsp;<code className="rounded bg-muted px-1">http://localhost:8080</code>).
                </p>
              )}
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
                setIsRouteDialogOpen(false)
                resetForm()
              }}
            >
              Cancel
            </Button>
            <Button
              onClick={handleCreateRoute}
              disabled={!pathPrefix || !targetURL || targetLoading}
            >
              <Route className="mr-2 h-4 w-4" />
              Create Route
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ── Edit Route Dialog ───────────────────────────────────────────────── */}
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
                placeholder="/my-service/"
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

      {/* ── Delete Confirmation Dialog ──────────────────────────────────────── */}
      <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Gateway Route</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete this route? This action cannot be
              undone and the NGINX configuration will be updated immediately.
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
