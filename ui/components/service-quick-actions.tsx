import Link from "next/link"
import { Archive, Container, GitBranch, Package, Zap } from "lucide-react"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"

export function ServiceQuickActions() {
  const quickActions = [
    {
      name: "Container Registry",
      description: "Manage and push container images to your private registry",
      icon: Package,
      color: "text-blue-600",
      bgColor: "bg-blue-50 hover:bg-blue-100",
      href: "/storage/containers",
    },
    {
      name: "Container Runtime",
      description: "Deploy and manage running containerized applications",
      icon: Container,
      color: "text-cyan-600",
      bgColor: "bg-cyan-50 hover:bg-cyan-100",
      href: "/compute/containers",
    },
    {
      name: "Blob Storage",
      description: "Store and retrieve unstructured data and files",
      icon: Archive,
      color: "text-purple-600",
      bgColor: "bg-purple-50 hover:bg-purple-100",
      href: "/storage/blob",
    },
    {
      name: "CI/CD Pipelines",
      description: "Automate builds, tests, and deployments with pipelines",
      icon: GitBranch,
      color: "text-orange-600",
      bgColor: "bg-orange-50 hover:bg-orange-100",
      href: "/ci-cd/pipelines",
    },
    {
      name: "Functions",
      description: "Run event-driven serverless functions at scale",
      icon: Zap,
      color: "text-green-600",
      bgColor: "bg-green-50 hover:bg-green-100",
      href: "/compute/functions",
    },
  ]

  return (
    <Card>
      <CardHeader>
        <CardTitle>Quick Actions</CardTitle>
        <CardDescription>Navigate to a service to get started</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {quickActions.map((action, index) => {
          const IconComponent = action.icon
          return (
            <Button
              key={index}
              variant="ghost"
              className={`w-full justify-start h-auto p-4 ${action.bgColor}`}
              asChild
            >
              <Link href={action.href}>
                <div className="flex items-start space-x-3 w-full">
                  <div className={`shrink-0 p-2 rounded-lg bg-white ${action.color}`}>
                    <IconComponent className="h-4 w-4" />
                  </div>
                  <div className="text-left min-w-0">
                    <div className="font-medium text-sm text-foreground whitespace-normal break-words">{action.name}</div>
                    <div className="text-xs text-muted-foreground whitespace-normal break-words">{action.description}</div>
                  </div>
                </div>
              </Link>
            </Button>
          )
        })}
      </CardContent>
    </Card>
  )
}
