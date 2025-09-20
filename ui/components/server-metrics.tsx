"use client"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import { Badge } from "@/components/ui/badge"
import { Cpu, HardDrive, MemoryStick, Network, Activity, Clock } from "lucide-react"
import axios from "axios";
import { useEffect } from "react";
import client from "../app/utility/post";

export function ServerMetrics() {

  useEffect(() => {
    const getMetrics = async () => {
      
      client.get("get-server-metrics")
        .then((response) => {
          console.log("Metrics:", response.data);
        })
        .catch((error) => {
          console.error("Error fetching data:", error);
        });
    };

    getMetrics();
  }, []); // empty deps -> runs once on mount

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-semibold">Host Server Metrics</h3>
          <p className="text-sm text-muted-foreground">Real-time performance data from your infrastructure</p>
        </div>
        <Badge variant="outline" className="bg-green-50 text-green-700">
          <Activity className="w-3 h-3 mr-1" />
          All Systems Operational 
        </Badge>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">CPU Usage</CardTitle>
            <Cpu className="h-4 w-4 text-blue-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">67%</div>
            <Progress value={67} className="mt-2" />
            <div className="flex justify-between text-xs text-muted-foreground mt-2">
              <span>8 cores</span>
              <span>2.4 GHz avg</span>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Memory</CardTitle>
            <MemoryStick className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">12.4 GB</div>
            <Progress value={78} className="mt-2" />
            <div className="flex justify-between text-xs text-muted-foreground mt-2">
              <span>78% of 16 GB</span>
              <span>3.6 GB free</span>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Disk Usage</CardTitle>
            <HardDrive className="h-4 w-4 text-purple-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">847 GB</div>
            <Progress value={84} className="mt-2" />
            <div className="flex justify-between text-xs text-muted-foreground mt-2">
              <span>84% of 1 TB</span>
              <span>153 GB free</span>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Network I/O</CardTitle>
            <Network className="h-4 w-4 text-orange-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">2.4 GB/s</div>
            <div className="flex justify-between text-xs text-muted-foreground mt-2">
              <span>↑ 1.2 GB/s</span>
              <span>↓ 1.2 GB/s</span>
            </div>
            <div className="text-xs text-muted-foreground mt-1">Peak: 4.8 GB/s</div>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle className="text-base flex items-center gap-2">
              <Clock className="h-4 w-4" />
              System Uptime
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">47 days</div>
            <p className="text-sm text-muted-foreground">Last restart: Dec 1, 2023</p>
            <div className="mt-4 space-y-2">
              <div className="flex justify-between text-sm">
                <span>Load Average (1m)</span>
                <span className="font-medium">2.34</span>
              </div>
              <div className="flex justify-between text-sm">
                <span>Load Average (5m)</span>
                <span className="font-medium">2.18</span>
              </div>
              <div className="flex justify-between text-sm">
                <span>Load Average (15m)</span>
                <span className="font-medium">1.95</span>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Process Information</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              <div className="flex justify-between items-center">
                <span className="text-sm">Total Processes</span>
                <span className="font-medium">247</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-sm">Running</span>
                <span className="font-medium text-green-600">156</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-sm">Sleeping</span>
                <span className="font-medium text-blue-600">89</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-sm">Zombie</span>
                <span className="font-medium text-red-600">2</span>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Top Processes</CardTitle>
            <CardDescription>By CPU usage</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              <div className="flex justify-between items-center">
                <div>
                  <div className="font-medium text-sm">node</div>
                  <div className="text-xs text-muted-foreground">PID: 1247</div>
                </div>
                <div className="text-right">
                  <div className="font-medium text-sm">23.4%</div>
                  <div className="text-xs text-muted-foreground">2.1 GB</div>
                </div>
              </div>
              <div className="flex justify-between items-center">
                <div>
                  <div className="font-medium text-sm">docker</div>
                  <div className="text-xs text-muted-foreground">PID: 892</div>
                </div>
                <div className="text-right">
                  <div className="font-medium text-sm">18.7%</div>
                  <div className="text-xs text-muted-foreground">1.8 GB</div>
                </div>
              </div>
              <div className="flex justify-between items-center">
                <div>
                  <div className="font-medium text-sm">postgres</div>
                  <div className="text-xs text-muted-foreground">PID: 1456</div>
                </div>
                <div className="text-right">
                  <div className="font-medium text-sm">12.3%</div>
                  <div className="text-xs text-muted-foreground">3.2 GB</div>
                </div>
              </div>
              <div className="flex justify-between items-center">
                <div>
                  <div className="font-medium text-sm">nginx</div>
                  <div className="text-xs text-muted-foreground">PID: 678</div>
                </div>
                <div className="text-right">
                  <div className="font-medium text-sm">8.9%</div>
                  <div className="text-xs text-muted-foreground">256 MB</div>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
