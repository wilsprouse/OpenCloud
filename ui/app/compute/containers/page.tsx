/*
This page displays the Containers, and the user can managed them in here as well

import DisplayContainers from "@/components/display-containers"

export default function Containers() {
  return (
    <DisplayContainers />

  )
}*/

'use client'

import { useEffect, useState } from "react"
import axios from "axios"
import { Button } from "@/components/ui/button"
import client from "@/app/utility/post";

type Container = {
  Id: string
  Names: string[]
  Image: string
  State: string
  Status: string
}

export default function ContainersPage() {
  const [containers, setContainers] = useState<Container[]>([])
  const [loading, setLoading] = useState(false)

  // Fetch containers
  const fetchContainers = async () => {
    setLoading(true)
    try {
      const res = await client.get<Container[]>("/get-containers")
      setContainers(res.data)
    } catch (err) {
      console.error("Failed to fetch containers:", err)
    } finally {
      setLoading(false)
    }
    console.log("Console log: " + containers)
  }

  useEffect(() => {
    fetchContainers()
  }, [])

  // Manage container actions
  const handleAction = async (id: string, action: "start" | "stop" | "remove") => {
    try {
      if (action === "remove") {
        await axios.delete(`/api/containers/${id}`)
      } else {
        await axios.post(`/api/containers/${id}/${action}`)
      }
      fetchContainers() // refresh list
    } catch (err) {
      console.error(`Failed to ${action} container:`, err)
    }
  }

  return (
    <div className="p-6">
      <h1 className="text-2xl font-bold mb-4">Container</h1>
      <Button onClick={fetchContainers} disabled={loading}>
        {loading ? "Refreshing..." : "Refresh"}
      </Button>

      <div className="mt-6 overflow-x-auto">
        <table className="min-w-full border border-gray-200 rounded-md">
          <thead className="bg-gray-100">
            <tr>
              <th className="px-4 py-2 text-left">ID</th>
              <th className="px-4 py-2 text-left">Name</th>
              <th className="px-4 py-2 text-left">Image ID</th>
              <th className="px-4 py-2 text-left">State</th>
              <th className="px-4 py-2 text-left">Status</th>
              <th className="px-4 py-2 text-left">Actions</th>
            </tr>
          </thead>
          <tbody>
            {containers.map((c) => (
              <tr key={c.Id} className="border-t">
                <td className="px-4 py-2">{c.Id.slice(7, 19)}</td>
                <td className="px-4 py-2">{c.Names?.[0]?.replace(/^\//, "")}</td>
                <td className="px-4 py-2">{c.Image}</td>
                <td className="px-4 py-2">{c.State}</td>
                <td className="px-4 py-2">{c.Status}</td>
                <td className="px-4 py-2 flex gap-2">
                  {c.State !== "running" && (
                    <Button
                      variant="default"
                      size="sm"
                      onClick={() => handleAction(c.Id, "start")}
                    >
                      Start
                    </Button>
                  )}
                  {c.State === "running" && (
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => handleAction(c.Id, "stop")}
                    >
                      Stop
                    </Button>
                  )}
                  <Button
                    variant="destructive"
                    size="sm"
                    onClick={() => handleAction(c.Id, "remove")}
                  >
                    Remove
                  </Button>
                </td>
              </tr>
            ))}
            {containers.length === 0 && !loading && (
              <tr>
                <td className="px-4 py-4 text-center text-gray-500" colSpan={6}>
                  No containers found
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
