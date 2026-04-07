"use client"

import { useCallback, useEffect, useRef, useState } from "react"

// TerminalLine represents a single rendered line in the terminal output.
type TerminalLine = {
  id: number
  text: string
  // "prompt" = the echoed command line (green), "output" = normal stdout/stderr,
  // "error" = error text from a failing command (red)
  type: "prompt" | "output" | "error"
}

// HostInfo is returned by GET /api/host/info and used to build the shell prompt.
type HostInfo = {
  user: string
  hostname: string
  cwd: string
}

// HostPage renders a full-height terminal that feels like a real shell.
// The user types directly at the prompt line embedded inside the terminal
// output area — there is no separate input box.
export default function HostPage() {
  const [lines, setLines] = useState<TerminalLine[]>([])
  const [input, setInput] = useState("")
  const [running, setRunning] = useState(false)
  const [hostInfo, setHostInfo] = useState<HostInfo>({
    user: "user",
    hostname: "localhost",
    cwd: "~",
  })
  const [history, setHistory] = useState<string[]>([])
  const [historyIndex, setHistoryIndex] = useState(-1)
  // savedInput preserves whatever the user had typed before navigating history.
  const [savedInput, setSavedInput] = useState("")

  const terminalRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const lineCounter = useRef(0)

  // Build the prompt string from current host info, e.g. "user@host:~$".
  const promptText = `${hostInfo.user}@${hostInfo.hostname}:${hostInfo.cwd}$`

  // Fetch initial host info (user, hostname, cwd) to display an accurate prompt.
  useEffect(() => {
    fetch("/api/host/info")
      .then((res) => (res.ok ? res.json() : null))
      .then((data: HostInfo | null) => {
        if (data) setHostInfo(data)
      })
      .catch(() => {
        /* keep defaults if the backend is unavailable */
      })
  }, [])

  // Auto-scroll to the bottom whenever new lines are added or the running state changes.
  useEffect(() => {
    if (terminalRef.current) {
      terminalRef.current.scrollTop = terminalRef.current.scrollHeight
    }
  }, [lines, running])

  // Focus the input on mount so the user can start typing immediately, and
  // keep focus there whenever a key is pressed anywhere on the page.
  // This means the user never has to click the terminal to start typing —
  // any keystroke, from any focus state, lands in the command input.
  useEffect(() => {
    inputRef.current?.focus()

    const handleGlobalKey = (e: KeyboardEvent) => {
      // Skip modifier-only keypresses (Ctrl, Alt, Meta combos used for shortcuts).
      if (e.ctrlKey || e.altKey || e.metaKey) return
      // Only steal focus when the active element is outside the input itself.
      if (document.activeElement !== inputRef.current) {
        inputRef.current?.focus()
      }
    }

    document.addEventListener("keydown", handleGlobalKey)
    return () => document.removeEventListener("keydown", handleGlobalKey)
  }, [])

  const appendLine = (text: string, type: TerminalLine["type"] = "output") => {
    lineCounter.current += 1
    setLines((prev) => [...prev, { id: lineCounter.current, text, type }])
  }

  const runCommand = useCallback(async () => {
    const cmd = input.trim()
    if (!cmd || running) return

    // "clear" is handled entirely in the browser — no round-trip needed.
    if (cmd === "clear") {
      setLines([])
      lineCounter.current = 0
      setInput("")
      setHistoryIndex(-1)
      setSavedInput("")
      return
    }

    // Echo the typed command into the terminal output with the full prompt prefix
    // so the history looks exactly like a real shell session.
    appendLine(`${promptText} ${cmd}`, "prompt")
    setInput("")
    setRunning(true)

    // Add to command history (deduplicated, newest first).
    setHistory((prev) => {
      const filtered = prev.filter((c) => c !== cmd)
      return [cmd, ...filtered].slice(0, 100)
    })
    setHistoryIndex(-1)
    setSavedInput("")

    try {
      const response = await fetch("/api/host/exec", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ command: cmd, cwd: hostInfo.cwd }),
      })

      if (!response.ok) {
        appendLine(await response.text(), "error")
        return
      }

      if (!response.body) {
        appendLine("Error: no response body from server", "error")
        return
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ""
      // Track the most recently seen SSE event name so we can route the
      // subsequent data: line to the correct handler.
      let pendingEvent = ""

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const parts = buffer.split("\n")
        buffer = parts.pop() ?? ""

        for (const part of parts) {
          if (part.startsWith("event: ")) {
            pendingEvent = part.slice(7).trim()
          } else if (part.startsWith("data: ")) {
            const data = part.slice(6)
            if (pendingEvent === "cwd") {
              // Backend has captured the new working directory — update the prompt.
              setHostInfo((prev) => ({ ...prev, cwd: data }))
            } else if (pendingEvent === "error") {
              if (data) appendLine(data, "error")
            } else if (data) {
              appendLine(data)
            }
            pendingEvent = ""
          } else if (part === "") {
            pendingEvent = ""
          }
        }
      }
    } catch (err) {
      appendLine(
        `Error: ${err instanceof Error ? err.message : String(err)}`,
        "error",
      )
    } finally {
      setRunning(false)
      inputRef.current?.focus()
    }
  }, [input, running, promptText, hostInfo.cwd])

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault()
      runCommand()
      return
    }

    // ↑ navigates older history entries; save the current input first.
    if (e.key === "ArrowUp") {
      e.preventDefault()
      if (historyIndex === -1) setSavedInput(input)
      const next = historyIndex + 1
      if (next < history.length) {
        setHistoryIndex(next)
        setInput(history[next])
      }
      return
    }

    // ↓ navigates toward the most-recent entry, then restores the saved input.
    if (e.key === "ArrowDown") {
      e.preventDefault()
      const next = historyIndex - 1
      if (next < 0) {
        setHistoryIndex(-1)
        setInput(savedInput)
      } else {
        setHistoryIndex(next)
        setInput(history[next])
      }
    }
  }

  // Clicking anywhere in the terminal refocuses the hidden input so keystrokes
  // are always captured without the user having to click on a specific field.
  const focusInput = () => inputRef.current?.focus()

  return (
    // Full-height terminal that sits directly below the sticky navbar (h-14 = 56px,
    // plus 1px border-b = 57px total).  No extra page chrome — just the terminal.
    <div
      className="flex flex-col bg-neutral-950 font-mono text-sm"
      style={{ height: "calc(100vh - 57px)" }}
    >
      {/* ── Scrollable output history ── */}
      <div
        ref={terminalRef}
        className="flex-1 overflow-y-auto p-4 cursor-text select-text"
        onClick={focusInput}
      >
        {/* Welcome hint — only shown when the terminal is empty */}
        {lines.length === 0 && !running && (
          <p className="text-neutral-500 text-xs select-none mb-1">
            OpenCloud Host Terminal — type a command and press Enter.{" "}
            <span className="text-neutral-400">clear</span> clears the screen.
          </p>
        )}

        {lines.map((line) => (
          <div
            key={line.id}
            className={
              line.type === "prompt"
                ? "text-green-400 whitespace-pre-wrap break-all leading-5"
                : line.type === "error"
                  ? "text-red-400 whitespace-pre-wrap break-all leading-5"
                  : "text-neutral-100 whitespace-pre-wrap break-all leading-5"
            }
          >
            {/* Preserve blank lines by rendering a non-breaking space */}
            {line.text || "\u00A0"}
          </div>
        ))}

        {/* Pulsing block cursor shown while a command is executing */}
        {running && (
          <span className="inline-block w-2 h-[1em] bg-neutral-400 animate-pulse align-text-bottom" />
        )}
      </div>

      {/* ── Inline prompt + input — pinned to the bottom of the terminal ── */}
      <div
        className="flex items-center px-4 py-2 border-t border-neutral-800 shrink-0"
        onClick={focusInput}
      >
        <span className="text-green-400 select-none shrink-0 whitespace-nowrap mr-1">
          {promptText}&nbsp;
        </span>
        <input
          ref={inputRef}
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          disabled={running}
          autoFocus
          spellCheck={false}
          autoComplete="off"
          autoCorrect="off"
          autoCapitalize="off"
          aria-label="Terminal input"
          className="flex-1 min-w-0 bg-transparent text-neutral-100 outline-none caret-neutral-100 disabled:opacity-40"
        />
      </div>
    </div>
  )
}

