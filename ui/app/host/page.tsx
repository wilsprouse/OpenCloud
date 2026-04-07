"use client"

import { useEffect, useRef, useState } from "react"
import { Terminal, Play, Trash2 } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { DashboardHeader } from "@/components/dashboard-header"
import { DashboardShell } from "@/components/dashboard-shell"

// HostOutputLine represents a single line of terminal output with its type.
type HostOutputLine = {
  id: number
  text: string
  type: "output" | "error" | "info"
}

// HostPage provides an interactive terminal interface for the underlying host
// machine.  The user types a shell command into the input field and clicks
// "Run" (or presses Enter).  Output is streamed from the backend line-by-line
// via SSE and displayed in the output panel below the input.
export default function HostPage() {
  const [command, setCommand] = useState("")
  const [lines, setLines] = useState<HostOutputLine[]>([])
  const [running, setRunning] = useState(false)
  const [history, setHistory] = useState<string[]>([])
  const [historyIndex, setHistoryIndex] = useState<number>(-1)
  const lineCounter = useRef(0)
  const outputRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  // Scroll the output panel to the bottom whenever new lines arrive.
  useEffect(() => {
    if (outputRef.current) {
      outputRef.current.scrollTop = outputRef.current.scrollHeight
    }
  }, [lines])

  // Append a new line to the output panel.
  const appendLine = (text: string, type: HostOutputLine["type"] = "output") => {
    lineCounter.current += 1
    setLines((prev) => [...prev, { id: lineCounter.current, text, type }])
  }

  // Execute the current command by streaming its output from the backend.
  const runCommand = async () => {
    const cmd = command.trim()
    if (!cmd || running) return

    // Echo the command into the output panel so the user can see what ran.
    appendLine(`$ ${cmd}`, "info")
    setCommand("")
    setRunning(true)

    // Keep command history (most-recent first deduplication).
    setHistory((prev) => {
      const filtered = prev.filter((c) => c !== cmd)
      return [cmd, ...filtered].slice(0, 100)
    })
    setHistoryIndex(-1)

    try {
      const response = await fetch("/api/host/exec", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ command: cmd }),
      })

      if (!response.ok) {
        const errText = await response.text()
        appendLine(`Error: ${errText}`, "error")
        return
      }

      if (!response.body) {
        appendLine("Error: No response body from server", "error")
        return
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ""

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const parts = buffer.split("\n")
        buffer = parts.pop() ?? ""

        for (const part of parts) {
          if (part.startsWith("data: ")) {
            const text = part.slice(6)
            if (text) appendLine(text)
          } else if (part.startsWith("event: error")) {
            // The error detail arrives on the next data: line — handled above.
          } else if (part.startsWith("event: done")) {
            // Command finished successfully; no extra output needed.
          }
        }
      }
    } catch (err) {
      appendLine(
        `Failed to execute command: ${err instanceof Error ? err.message : String(err)}`,
        "error",
      )
    } finally {
      setRunning(false)
      // Return focus to the input after execution completes.
      inputRef.current?.focus()
    }
  }

  // Handle keyboard shortcuts in the command input.
  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault()
      runCommand()
      return
    }

    // Navigate command history with the arrow keys.
    if (e.key === "ArrowUp") {
      e.preventDefault()
      const nextIndex = historyIndex + 1
      if (nextIndex < history.length) {
        setHistoryIndex(nextIndex)
        setCommand(history[nextIndex])
      }
      return
    }

    if (e.key === "ArrowDown") {
      e.preventDefault()
      const nextIndex = historyIndex - 1
      if (nextIndex < 0) {
        setHistoryIndex(-1)
        setCommand("")
      } else {
        setHistoryIndex(nextIndex)
        setCommand(history[nextIndex])
      }
    }
  }

  const clearOutput = () => {
    setLines([])
    lineCounter.current = 0
  }

  return (
    <DashboardShell>
      <DashboardHeader
        heading="Host Terminal"
        text="Run shell commands directly on the host machine."
      >
        <Button variant="outline" size="sm" onClick={clearOutput} disabled={running}>
          <Trash2 className="mr-2 h-4 w-4" />
          Clear
        </Button>
      </DashboardHeader>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <Terminal className="h-4 w-4" />
            Terminal
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {/* Output display */}
          <div
            ref={outputRef}
            className="h-[480px] overflow-y-auto rounded-md bg-neutral-950 p-4 font-mono text-sm text-neutral-100 dark:bg-black"
          >
            {lines.length === 0 ? (
              <span className="text-neutral-500">
                Enter a command below and press Run or hit Enter.
              </span>
            ) : (
              lines.map((line) => (
                <div
                  key={line.id}
                  className={
                    line.type === "error"
                      ? "text-red-400"
                      : line.type === "info"
                        ? "text-green-400"
                        : "text-neutral-100"
                  }
                >
                  {line.text}
                </div>
              ))
            )}
            {running && (
              <span className="animate-pulse text-neutral-500">▋</span>
            )}
          </div>

          {/* Command input row */}
          <div className="flex gap-2">
            <span className="flex items-center font-mono text-sm text-muted-foreground">
              $
            </span>
            <input
              ref={inputRef}
              type="text"
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Enter shell command…"
              disabled={running}
              autoFocus
              className="flex-1 rounded-md border bg-background px-3 py-2 font-mono text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
            />
            <Button onClick={runCommand} disabled={running || !command.trim()}>
              <Play className="mr-2 h-4 w-4" />
              {running ? "Running…" : "Run"}
            </Button>
          </div>
        </CardContent>
      </Card>
    </DashboardShell>
  )
}
