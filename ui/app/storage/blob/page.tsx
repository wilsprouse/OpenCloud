'use client'

import { useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import client from "@/app/utility/post"
import { Upload, Download, Trash2, FolderPlus } from "lucide-react"

type Blob = {
  id: string
  name: string
  size: number
  contentType: string
  lastModified: string
  container: string
}

export default function BlobStorage() {
  const [blobs, setBlobs] = useState<Blob[]>([])
  const [loading, setLoading] = useState(false)
  const [selectedContainer, setSelectedContainer] = useState<string>("default")

  // Fetch blobs
  const fetchBlobs = async () => {
    setLoading(true)
    try {
      const res = await client.get<Blob[]>("/get-blobs")
      setBlobs(res.data)
    } catch (err) {
      console.error("Failed to fetch blobs:", err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchBlobs()
  }, [])

  // Handle blob actions
  const handleDownload = async (id: string, name: string) => {
    try {
      console.log(`Downloading blob: ${name}`)
      // Backend implementation will handle actual download
    } catch (err) {
      console.error("Failed to download blob:", err)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      console.log(`Deleting blob: ${id}`)
      // Backend implementation will handle actual delete
      fetchBlobs() // refresh list
    } catch (err) {
      console.error("Failed to delete blob:", err)
    }
  }

  const handleUpload = () => {
    console.log("Upload dialog will be triggered")
    // Backend implementation will handle file upload
  }

  const handleCreateContainer = () => {
    console.log("Create container dialog will be triggered")
    // Backend implementation will handle container creation
  }

  // Format file size
  const formatSize = (bytes: number): string => {
    if (bytes === 0) return "0 B"
    const k = 1024
    const sizes = ["B", "KB", "MB", "GB", "TB"]
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i]
  }

  // Format date
  const formatDate = (dateString: string): string => {
    try {
      const date = new Date(dateString)
      return date.toLocaleString()
    } catch {
      return dateString
    }
  }

  return (
    <div className="p-6">
      <h1 className="text-2xl font-bold mb-4">Blob Storage</h1>
      
      <div className="flex gap-2 mb-6">
        <Button onClick={fetchBlobs} disabled={loading}>
          {loading ? "Refreshing..." : "Refresh"}
        </Button>
        <Button onClick={handleUpload} variant="default">
          <Upload className="mr-2 h-4 w-4" />
          Upload Blob
        </Button>
        <Button onClick={handleCreateContainer} variant="outline">
          <FolderPlus className="mr-2 h-4 w-4" />
          Create Container
        </Button>
      </div>

      <div className="mt-6 overflow-x-auto">
        <table className="min-w-full border border-gray-200 rounded-md">
          <thead className="bg-gray-100">
            <tr>
              <th className="px-4 py-2 text-left">Name</th>
              <th className="px-4 py-2 text-left">Container</th>
              <th className="px-4 py-2 text-left">Size</th>
              <th className="px-4 py-2 text-left">Content Type</th>
              <th className="px-4 py-2 text-left">Last Modified</th>
              <th className="px-4 py-2 text-left">Actions</th>
            </tr>
          </thead>
          <tbody>
            {blobs.map((blob) => (
              <tr key={blob.id} className="border-t">
                <td className="px-4 py-2">{blob.name}</td>
                <td className="px-4 py-2">{blob.container}</td>
                <td className="px-4 py-2">{formatSize(blob.size)}</td>
                <td className="px-4 py-2">{blob.contentType}</td>
                <td className="px-4 py-2">{formatDate(blob.lastModified)}</td>
                <td className="px-4 py-2 flex gap-2">
                  <Button
                    variant="default"
                    size="sm"
                    onClick={() => handleDownload(blob.id, blob.name)}
                  >
                    <Download className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="destructive"
                    size="sm"
                    onClick={() => handleDelete(blob.id)}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </td>
              </tr>
            ))}
            {blobs.length === 0 && !loading && (
              <tr>
                <td className="px-4 py-4 text-center text-gray-500" colSpan={6}>
                  No blobs found. Upload your first blob to get started.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
