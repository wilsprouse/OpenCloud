import { ChevronRight, Container, Database, Globe, Zap } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { DashboardHeader } from "@/components/dashboard-header"
import { DashboardShell } from "@/components/dashboard-shell"
import { RecentServices } from "@/components/recent-services"
import { ServiceQuickActions } from "@/components/service-quick-actions"
import { ServerMetrics } from "@/components/server-metrics"

export default function DashboardPage() {
  return (
    <>
      <DashboardShell>
        <DashboardHeader heading="Welcome back, John" text="Here's what's happening with your cloud services today.">
          <Button>
            Deploy Service <ChevronRight className="ml-2 h-4 w-4" />
          </Button>
        </DashboardHeader>

        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
          <Card className="border-l-4 border-l-blue-500">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Container Runtime</CardTitle>
              <Container className="h-4 w-4 text-blue-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">24</div>
              <p className="text-xs text-muted-foreground">Active containers</p>
              <div className="mt-2">
                <Badge variant="outline" className="text-xs">
                  Last used: 5 min ago
                </Badge>
              </div>
            </CardContent>
          </Card>

          <Card className="border-l-4 border-l-green-500">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Serverless Functions</CardTitle>
              <Zap className="h-4 w-4 text-green-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">12</div>
              <p className="text-xs text-muted-foreground">Functions deployed</p>
              <div className="mt-2">
                <Badge variant="outline" className="text-xs">
                  Last used: 2 min ago
                </Badge>
              </div>
            </CardContent>
          </Card>

          <Card className="border-l-4 border-l-purple-500">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Databases</CardTitle>
              <Database className="h-4 w-4 text-purple-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">8</div>
              <p className="text-xs text-muted-foreground">Active databases</p>
              <div className="mt-2">
                <Badge variant="outline" className="text-xs">
                  Last used: 1 hour ago
                </Badge>
              </div>
            </CardContent>
          </Card>

          <Card className="border-l-4 border-l-orange-500">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">API Gateway</CardTitle>
              <Globe className="h-4 w-4 text-orange-500" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">156K</div>
              <p className="text-xs text-muted-foreground">Requests today</p>
              <div className="mt-2">
                <Badge variant="outline" className="text-xs">
                  Last used: 30 sec ago
                </Badge>
              </div>
            </CardContent>
          </Card>
        </div>

        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          <div className="lg:col-span-2">
            <RecentServices />
          </div>
          <div>
            <ServiceQuickActions />
          </div>
        </div>

        <ServerMetrics />
      </DashboardShell>
    </>
  )
}
