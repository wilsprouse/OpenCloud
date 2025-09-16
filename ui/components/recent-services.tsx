import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Container, Database, Zap, Code, ExternalLink, Play, Square } from "lucide-react"

export function RecentServices() {
  const recentServices = [
    {
      name: "web-app-container",
      type: "Container Runtime",
      icon: Container,
      status: "Running",
      statusColor: "bg-green-100 text-green-800",
      lastUsed: "5 minutes ago",
      description: "Node.js web application container",
      metrics: "CPU: 45% | Memory: 2.1GB",
    },
    {
      name: "user-auth-function",
      type: "Serverless Function",
      icon: Zap,
      status: "Active",
      statusColor: "bg-blue-100 text-blue-800",
      lastUsed: "2 minutes ago",
      description: "Authentication service handler",
      metrics: "Invocations: 1.2K today",
    },
    {
      name: "analytics-pipeline",
      type: "Container Runtime",
      icon: Container,
      status: "Running",
      statusColor: "bg-green-100 text-green-800",
      lastUsed: "8 minutes ago",
      description: "Data processing pipeline",
      metrics: "CPU: 78% | Memory: 4.8GB",
    },
    {
      name: "customer-database",
      type: "Managed Database",
      icon: Database,
      status: "Online",
      statusColor: "bg-green-100 text-green-800",
      lastUsed: "1 hour ago",
      description: "PostgreSQL customer data store",
      metrics: "Connections: 24/100",
    },
    {
      name: "image-processor",
      type: "Serverless Function",
      icon: Zap,
      status: "Active",
      statusColor: "bg-blue-100 text-blue-800",
      lastUsed: "15 minutes ago",
      description: "Image optimization service",
      metrics: "Invocations: 456 today",
    },
    {
      name: "api-gateway",
      type: "API Service",
      icon: Code,
      status: "Healthy",
      statusColor: "bg-green-100 text-green-800",
      lastUsed: "30 seconds ago",
      description: "Main API gateway service",
      metrics: "Requests: 156K today",
    },
  ]

  return (
    <Card>
      <CardHeader>
        <CardTitle>Recent Services</CardTitle>
        <CardDescription>Services you've used recently, sorted by last activity</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {recentServices.map((service, index) => {
          const IconComponent = service.icon
          return (
            <div
              key={index}
              className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
            >
              <div className="flex items-center space-x-4">
                <div className="bg-muted p-2 rounded-lg">
                  <IconComponent className="h-5 w-5" />
                </div>
                <div className="space-y-1">
                  <div className="flex items-center space-x-2">
                    <h4 className="font-medium">{service.name}</h4>
                    <Badge variant="outline" className={service.statusColor}>
                      {service.status}
                    </Badge>
                  </div>
                  <p className="text-sm text-muted-foreground">{service.description}</p>
                  <div className="flex items-center space-x-4 text-xs text-muted-foreground">
                    <span>{service.type}</span>
                    <span>•</span>
                    <span>{service.lastUsed}</span>
                    <span>•</span>
                    <span>{service.metrics}</span>
                  </div>
                </div>
              </div>
              <div className="flex items-center space-x-2">
                <Button variant="ghost" size="icon">
                  <ExternalLink className="h-4 w-4" />
                </Button>
                {service.status === "Running" ? (
                  <Button variant="ghost" size="icon">
                    <Square className="h-4 w-4" />
                  </Button>
                ) : (
                  <Button variant="ghost" size="icon">
                    <Play className="h-4 w-4" />
                  </Button>
                )}
              </div>
            </div>
          )
        })}
      </CardContent>
    </Card>
  )
}
