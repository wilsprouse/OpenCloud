import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"

export function UsageMetrics() {
  return (
    <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Container Usage</CardTitle>
          <CardDescription>This month</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">1,247</div>
          <p className="text-xs text-muted-foreground">container hours</p>
          <Progress value={62} className="mt-2" />
          <p className="text-xs text-muted-foreground mt-1">62% of monthly quota</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Function Invocations</CardTitle>
          <CardDescription>This month</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">2.4M</div>
          <p className="text-xs text-muted-foreground">function calls</p>
          <Progress value={48} className="mt-2" />
          <p className="text-xs text-muted-foreground mt-1">48% of monthly quota</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Database Storage</CardTitle>
          <CardDescription>Across all instances</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">847 GB</div>
          <p className="text-xs text-muted-foreground">total storage used</p>
          <Progress value={84} className="mt-2" />
          <p className="text-xs text-muted-foreground mt-1">84% of allocated storage</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">API Requests</CardTitle>
          <CardDescription>Last 24 hours</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">156K</div>
          <p className="text-xs text-muted-foreground">total requests</p>
          <Progress value={31} className="mt-2" />
          <p className="text-xs text-muted-foreground mt-1">31% of daily limit</p>
        </CardContent>
      </Card>
    </div>
  )
}
