"use client"

import { useEffect, useRef } from "react"

// HostPage renders a full-height xterm.js terminal connected to a real PTY
// session on the server via WebSocket. The shell (bash/sh) runs as the same
// user as the OpenCloud backend process, giving a genuine interactive terminal
// experience: tab completion, interactive programs (vim, htop, …), ANSI
// colours, Ctrl+C / Ctrl+D, and proper terminal resize all work out of the box.
export default function HostPage() {
  // termRef is attached to the div that xterm.js renders into.
  const termRef = useRef<HTMLDivElement>(null)
  // Holds the resize handler so the effect cleanup can remove the exact same
  // function instance that was added to window.
  const handleResizeRef = useRef<(() => void) | null>(null)

  useEffect(() => {
    // Dynamic imports keep xterm out of the server-side bundle.
    let terminal: import("xterm").Terminal | null = null
    let ws: WebSocket | null = null
    let fitAddon: import("xterm-addon-fit").FitAddon | null = null
    let resizeObserver: ResizeObserver | null = null

    const init = async () => {
      const { Terminal } = await import("xterm")
      const { FitAddon } = await import("xterm-addon-fit")
      const { WebLinksAddon } = await import("xterm-addon-web-links")

      // Import xterm CSS (only runs once in the browser).
      await import("xterm/css/xterm.css")

      if (!termRef.current) return

      terminal = new Terminal({
        cursorBlink: true,
        fontFamily: '"Cascadia Code", "Fira Code", "JetBrains Mono", monospace',
        fontSize: 14,
        lineHeight: 1.2,
        theme: {
          background: "#0a0a0a",
          foreground: "#e5e5e5",
          cursor: "#e5e5e5",
          selectionBackground: "#3d3d3d",
          black: "#0a0a0a",
          red: "#f87171",
          green: "#4ade80",
          yellow: "#facc15",
          blue: "#60a5fa",
          magenta: "#c084fc",
          cyan: "#22d3ee",
          white: "#e5e5e5",
          brightBlack: "#525252",
          brightRed: "#fca5a5",
          brightGreen: "#86efac",
          brightYellow: "#fde047",
          brightBlue: "#93c5fd",
          brightMagenta: "#d8b4fe",
          brightCyan: "#67e8f9",
          brightWhite: "#fafafa",
        },
        allowProposedApi: true,
        scrollback: 5000,
      })

      fitAddon = new FitAddon()
      terminal.loadAddon(fitAddon)
      terminal.loadAddon(new WebLinksAddon())
      terminal.open(termRef.current)
      fitAddon.fit()

      // Build the WebSocket URL.
      // NEXT_PUBLIC_WS_BACKEND_URL is set for local dev (ws://localhost:3030).
      // In production via nginx the /api/ location block proxies WebSocket so
      // we use the same origin.
      const wsBase =
        process.env.NEXT_PUBLIC_WS_BACKEND_URL ||
        `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}/api`
      ws = new WebSocket(`${wsBase}/host/ws`)
      ws.binaryType = "arraybuffer"

      ws.onopen = () => {
        // Send the initial terminal size to the backend so the shell lays out
        // correctly from the very first prompt.
        if (ws && terminal && fitAddon) {
          const dims = fitAddon.proposeDimensions()
          if (dims) {
            ws.send(
              JSON.stringify({ type: "resize", cols: dims.cols, rows: dims.rows }),
            )
          }
        }
        terminal?.focus()
      }

      ws.onmessage = (event) => {
        if (!terminal) return
        if (event.data instanceof ArrayBuffer) {
          terminal.write(new Uint8Array(event.data))
        } else {
          terminal.write(event.data)
        }
      }

      ws.onclose = () => {
        terminal?.writeln("\r\n\x1b[33m[Connection closed]\x1b[0m")
      }

      ws.onerror = () => {
        terminal?.writeln("\r\n\x1b[31m[WebSocket error — is the backend running?]\x1b[0m")
      }

      // Forward every keystroke / paste to the backend PTY.
      terminal.onData((data) => {
        if (ws?.readyState === WebSocket.OPEN) {
          ws.send(data)
        }
      })

      // Resize the PTY whenever the container changes size.
      const handleResize = () => {
        if (!fitAddon || !terminal || !ws) return
        fitAddon.fit()
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(
            JSON.stringify({ type: "resize", cols: terminal.cols, rows: terminal.rows }),
          )
        }
      }

      resizeObserver = new ResizeObserver(handleResize)
      if (termRef.current) resizeObserver.observe(termRef.current)
      handleResizeRef.current = handleResize
      window.addEventListener("resize", handleResize)
    }

    init()

    return () => {
      resizeObserver?.disconnect()
      if (handleResizeRef.current) {
        window.removeEventListener("resize", handleResizeRef.current)
      }
      ws?.close()
      terminal?.dispose()
    }
  }, [])

  return (
    <div
      className="bg-[#0a0a0a]"
      style={{ height: "calc(100vh - 57px)" }}
    >
      <div ref={termRef} className="w-full h-full" />
    </div>
  )
}

