"use client"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import { Badge } from "@/components/ui/badge"
import { Cpu, HardDrive, Activity } from "lucide-react"
import { useState, useEffect } from "react";
import client from "../app/utility/post";

export function ServerMetrics() {

  const [ UsedStorage, setUsedStorage ] = useState("")
  const [ AvailableStorage, setAvailableStorage ] = useState("")
  const [ TotalStorage, setTotalStorage ] = useState("")
  const [ PercentageUsed, setPercentageUsed ] = useState("")
  const [ CpuUsage, setCpuUsage ] = useState<number | null>(null)

  useEffect(() => {
    
    const getMetrics = async () => {

      try {
        const response = await client.get("get-server-metrics")
      
        // update state with UsedStorage field
        setUsedStorage(response.data.STORAGE.UsedStorage)
        setAvailableStorage(response.data.STORAGE.AvailableStorage)
        setTotalStorage(response.data.STORAGE.TotalStorage)
        setPercentageUsed(response.data.STORAGE.PercentageUsed)
        setCpuUsage(response.data.CPU)

      } catch (error) {
        console.error("Error fetching data:", error)
      }
    }

    getMetrics() 
  
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

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">CPU Usage</CardTitle>
            <Cpu className="h-4 w-4 text-blue-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{CpuUsage !== null ? `${CpuUsage}%` : "—"}</div>
            <Progress value={CpuUsage ?? 0} className="mt-2" />
            <div className="flex justify-between text-xs text-muted-foreground mt-2">
              <span>Snapshot on page load</span>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Storage</CardTitle>
            <HardDrive className="h-4 w-4 text-purple-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{UsedStorage} GB</div>
            <Progress value={parseInt(PercentageUsed)} className="mt-2" />
            <div className="flex justify-between text-xs text-muted-foreground mt-2">
              <span>{PercentageUsed}% Used of {TotalStorage} GB</span>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
