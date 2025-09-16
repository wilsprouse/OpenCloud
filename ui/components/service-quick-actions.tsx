import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Container, Database, Zap, Code, Shield, BarChart3 } from "lucide-react"

export function ServiceQuickActions() {
  const quickActions = [
    {
      name: "Deploy Container",
      description: "Launch a new containerized application",
      icon: Container,
      color: "text-blue-600",
      bgColor: "bg-blue-50 hover:bg-blue-100",
    },
    {
      name: "Create Function",
      description: "Deploy a serverless function",
      icon: Zap,
      color: "text-green-600",
      bgColor: "bg-green-50 hover:bg-green-100",
    },
    {
      name: "Setup Database",
      description: "Create a managed database instance",
      icon: Database,
      color: "text-purple-600",
      bgColor: "bg-purple-50 hover:bg-purple-100",
    },
    {
      name: "API Gateway",
      description: "Configure API routing and management",
      icon: Code,
      color: "text-orange-600",
      bgColor: "bg-orange-50 hover:bg-orange-100",
    },
    {
      name: "Security Scan",
      description: "Run security analysis on services",
      icon: Shield,
      color: "text-red-600",
      bgColor: "bg-red-50 hover:bg-red-100",
    },
    {
      name: "Analytics",
      description: "View service metrics and logs",
      icon: BarChart3,
      color: "text-indigo-600",
      bgColor: "bg-indigo-50 hover:bg-indigo-100",
    },
  ]

  return (
    <Card>
      <CardHeader>
        <CardTitle>Quick Actions</CardTitle>
        <CardDescription>Common tasks and service deployments</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {quickActions.map((action, index) => {
          const IconComponent = action.icon
          return (
            <Button key={index} variant="ghost" className={`w-full justify-start h-auto p-4 ${action.bgColor}`}>
              <div className="flex items-center space-x-3">
                <div className={`p-2 rounded-lg bg-white ${action.color}`}>
                  <IconComponent className="h-4 w-4" />
                </div>
                <div className="text-left">
                  <div className="font-medium text-sm">{action.name}</div>
                  <div className="text-xs text-muted-foreground">{action.description}</div>
                </div>
              </div>
            </Button>
          )
        })}
      </CardContent>
    </Card>
  )
}
