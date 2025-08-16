import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { AlertCircle, CheckCircle2 } from "lucide-react"

export function Overview() {
  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">System Status</CardTitle>
          <CheckCircle2 className="h-4 w-4 text-green-500" />
        </CardHeader>
        <CardContent>
          <div className="text-sm font-medium">All systems operational</div>
          <p className="text-xs text-muted-foreground">Last checked: 5 minutes ago</p>
          <div className="mt-4 space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-xs">US East</span>
              <Badge variant="outline" className="bg-green-50 text-green-700 hover:bg-green-50 hover:text-green-700">
                Operational
              </Badge>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-xs">US West</span>
              <Badge variant="outline" className="bg-green-50 text-green-700 hover:bg-green-50 hover:text-green-700">
                Operational
              </Badge>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-xs">EU Central</span>
              <Badge variant="outline" className="bg-green-50 text-green-700 hover:bg-green-50 hover:text-green-700">
                Operational
              </Badge>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-xs">Asia Pacific</span>
              <Badge variant="outline" className="bg-green-50 text-green-700 hover:bg-green-50 hover:text-green-700">
                Operational
              </Badge>
            </div>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Resource Quotas</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div>
              <div className="flex items-center justify-between text-xs">
                <span>CPU Cores</span>
                <span>24/32</span>
              </div>
              <div className="mt-1 h-2 w-full rounded-full bg-muted">
                <div className="h-full w-[75%] rounded-full bg-blue-500"></div>
              </div>
            </div>
            <div>
              <div className="flex items-center justify-between text-xs">
                <span>Memory (GB)</span>
                <span>64/128</span>
              </div>
              <div className="mt-1 h-2 w-full rounded-full bg-muted">
                <div className="h-full w-[50%] rounded-full bg-blue-500"></div>
              </div>
            </div>
            <div>
              <div className="flex items-center justify-between text-xs">
                <span>Storage (TB)</span>
                <span>2.4/5</span>
              </div>
              <div className="mt-1 h-2 w-full rounded-full bg-muted">
                <div className="h-full w-[48%] rounded-full bg-blue-500"></div>
              </div>
            </div>
            <div>
              <div className="flex items-center justify-between text-xs">
                <span>Network Egress (TB)</span>
                <span>4.2/10</span>
              </div>
              <div className="mt-1 h-2 w-full rounded-full bg-muted">
                <div className="h-full w-[42%] rounded-full bg-blue-500"></div>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Alerts & Notifications</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <Alert variant="destructive">
            <AlertCircle className="h-4 w-4" />
            <AlertTitle>High CPU Usage</AlertTitle>
            <AlertDescription>VM instance "web-server-03" has exceeded 90% CPU usage for 15 minutes.</AlertDescription>
          </Alert>
          <Alert>
            <AlertCircle className="h-4 w-4" />
            <AlertTitle>Storage Approaching Limit</AlertTitle>
            <AlertDescription>Database "analytics-db" storage usage at 85% of allocated capacity.</AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    </div>
  )
}