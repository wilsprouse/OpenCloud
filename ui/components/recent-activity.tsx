import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { AlertTriangle, CheckCircle2, Database, HardDrive, Server, Settings } from "lucide-react"

export function RecentActivity() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Recent Activity</CardTitle>
        <CardDescription>System events and user actions from the last 24 hours</CardDescription>
      </CardHeader>
      <CardContent className="space-y-8">
        <div className="flex items-start space-x-4">
          <div className="bg-green-100 p-2 rounded-full">
            <CheckCircle2 className="h-5 w-5 text-green-600" />
          </div>
          <div className="space-y-1">
            <p className="text-sm font-medium leading-none">Virtual Machine Created</p>
            <p className="text-sm text-muted-foreground">
              New VM "api-server-05" was successfully provisioned in US East region
            </p>
            <div className="flex items-center pt-2">
              <Avatar className="h-8 w-8 mr-2">
                <AvatarImage src="/placeholder-user.jpg" alt="@johndoe" />
                <AvatarFallback>JD</AvatarFallback>
              </Avatar>
              <div className="text-xs text-muted-foreground">
                <span className="font-medium text-foreground">John Doe</span> • 35 minutes ago
              </div>
            </div>
          </div>
        </div>
        <div className="flex items-start space-x-4">
          <div className="bg-yellow-100 p-2 rounded-full">
            <AlertTriangle className="h-5 w-5 text-yellow-600" />
          </div>
          <div className="space-y-1">
            <p className="text-sm font-medium leading-none">High CPU Usage Alert</p>
            <p className="text-sm text-muted-foreground">VM "web-server-03" exceeded 90% CPU usage for 15 minutes</p>
            <div className="flex items-center pt-2">
              <Server className="h-4 w-4 mr-2 text-muted-foreground" />
              <div className="text-xs text-muted-foreground">System Alert • 1 hour ago</div>
            </div>
          </div>
        </div>
        <div className="flex items-start space-x-4">
          <div className="bg-blue-100 p-2 rounded-full">
            <Database className="h-5 w-5 text-blue-600" />
          </div>
          <div className="space-y-1">
            <p className="text-sm font-medium leading-none">Database Backup Completed</p>
            <p className="text-sm text-muted-foreground">Scheduled backup of "customer-db" completed successfully</p>
            <div className="flex items-center pt-2">
              <Settings className="h-4 w-4 mr-2 text-muted-foreground" />
              <div className="text-xs text-muted-foreground">Automated Process • 3 hours ago</div>
            </div>
          </div>
        </div>
        <div className="flex items-start space-x-4">
          <div className="bg-purple-100 p-2 rounded-full">
            <HardDrive className="h-5 w-5 text-purple-600" />
          </div>
          <div className="space-y-1">
            <p className="text-sm font-medium leading-none">Storage Volume Expanded</p>
            <p className="text-sm text-muted-foreground">Storage volume "app-data" expanded from 100GB to 250GB</p>
            <div className="flex items-center pt-2">
              <Avatar className="h-8 w-8 mr-2">
                <AvatarImage src="/placeholder-user.jpg" alt="@sarahjohnson" />
                <AvatarFallback>SJ</AvatarFallback>
              </Avatar>
              <div className="text-xs text-muted-foreground">
                <span className="font-medium text-foreground">Sarah Johnson</span> • 5 hours ago
              </div>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
