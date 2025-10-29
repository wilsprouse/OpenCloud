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

type Image = {
  Id: string
  Names: string[]
  Image: string
  State: string
  Size: string
  Status: string
}

export default function ContainerRegistry() {
  const [images, setImages] = useState<Image[]>([])
  const [loading, setLoading] = useState(false)

  // Fetch containers
  const fetchImages = async () => {
    setLoading(true)
    try {
      const res = await client.get<Image[]>("/get-containers")
      setImages(res.data)
    } catch (err) {
      console.error("Failed to fetch containers:", err)
    } finally {
      setLoading(false)
    }
    console.log("Console log: " + images)
  }

  useEffect(() => {
    fetchImages()
  }, [])

  // Manage container actions
  const handleAction = async (id: string, action: "start" | "stop" | "remove") => {
    try {
      if (action === "remove") {
        await axios.delete(`/api/containers/${id}`)
      } else {
        await axios.post(`/api/containers/${id}/${action}`)
      }
      fetchImages() // refresh list
    } catch (err) {
      console.error(`Failed to ${action} container:`, err)
    }
  }

  return (
    <div className="p-6">
      <h1 className="text-2xl font-bold mb-4">Container Registry</h1>
      <Button onClick={fetchImages} disabled={loading}>
        {loading ? "Refreshing..." : "Refresh"}
      </Button>

      <div className="mt-6 overflow-x-auto">
        <table className="min-w-full border border-gray-200 rounded-md">
          <thead className="bg-gray-100">
            <tr>
              <th className="px-4 py-2 text-left">ID</th>
              <th className="px-4 py-2 text-left">Name</th>
              <th className="px-4 py-2 text-left">Image ID</th>
              <th className="px-4 py-2 text-left">Size</th>
              <th className="px-4 py-2 text-left">Status</th>
              <th className="px-4 py-2 text-left">Actions</th>
            </tr>
          </thead>
          <tbody>
            {images.map((c) => (
              <tr key={c.Id} className="border-t">
                <td className="px-4 py-2">{c.Id.slice(7, 19)}</td>
                <td className="px-4 py-2">{c.Names?.[0]?.replace(/^\//, "")}</td>
                <td className="px-4 py-2">{c.Id.slice(7, 19)}</td>
                <td className="px-4 py-2">{(Number(c.Size) / 1_000_000).toFixed(2)} MB</td>
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
            {images.length === 0 && !loading && (
              <tr>
                <td className="px-4 py-4 text-center text-gray-500" colSpan={6}>
                  No images found
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
