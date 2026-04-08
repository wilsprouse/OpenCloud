package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"syscall"
	"unsafe"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

// HostInfo contains basic host metadata returned by GET /host/info.
type HostInfo struct {
	User     string `json:"user"`
	Hostname string `json:"hostname"`
	Cwd      string `json:"cwd"`
}

// resizeMsg is the JSON message sent by the frontend to resize the PTY.
type resizeMsg struct {
	Type string `json:"type"` // "resize"
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// wsUpgrader allows WebSocket connections from the same origin.
// CheckOrigin always returns true because the backend already enforces
// localhost-only listening and session auth at the Next.js proxy layer.
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// GetHostInfo handles GET /host/info.
// Returns the current OS user, hostname, and working directory so the
// UI can display contextual information if needed.
func GetHostInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := "user"
	if u, err := user.Current(); err == nil && u.Username != "" {
		username = u.Username
	}

	hostname := "localhost"
	if h, err := os.Hostname(); err == nil {
		hostname = h
	}

	home, _ := os.UserHomeDir()
	cwd := "~"
	if wd, err := os.Getwd(); err == nil {
		if home != "" && wd == home {
			cwd = "~"
		} else if home != "" && strings.HasPrefix(wd, home+"/") {
			cwd = "~" + wd[len(home):]
		} else {
			cwd = wd
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HostInfo{
		User:     username,
		Hostname: hostname,
		Cwd:      cwd,
	})
}

// HostTerminal handles GET /host/ws.
// It upgrades the connection to WebSocket, spawns a real login shell inside a
// PTY, and then bidirectionally proxies data between the WebSocket and the PTY.
//
// The frontend sends two kinds of messages:
//   - Binary / text frames containing raw terminal input bytes (keystrokes).
//   - JSON text frames of the form {"type":"resize","cols":N,"rows":N} which
//     resize the PTY window (SIGWINCH) so that programs like vim lay out
//     correctly.
//
// The backend sends binary frames containing raw PTY output (ANSI sequences,
// colours, cursor movement, etc.) which xterm.js renders natively.
//
// Security: the backend listens only on localhost:3030 and all routes are
// protected by session authentication enforced at the Next.js proxy.
func HostTerminal(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("host/ws: WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Determine the user's preferred shell, falling back to /bin/bash.
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	// Build the shell environment.
	//
	// Start from the current process environment so that all variables set
	// before the backend started (PATH, HOME, USER, GOPATH, …) are inherited.
	// Then inject or override the session-level variables that PAM / systemd-
	// logind normally add during an interactive login but that are absent when
	// OpenCloud is started as a system service (e.g. via systemd).  Without
	// these, rootless tools such as Podman, DBus clients, and snap cannot
	// locate their runtime sockets.
	envMap := make(map[string]string, len(os.Environ())+2)
	for _, e := range os.Environ() {
		if idx := strings.IndexByte(e, '='); idx >= 0 {
			envMap[e[:idx]] = e[idx+1:]
		}
	}

	// XDG_RUNTIME_DIR  — rootless Podman, DBus, pipewire, etc. use
	// /run/user/<UID>/… for their sockets.  PAM sets this automatically for
	// interactive sessions; derive it from the current UID when it is absent
	// (service start) or incorrect.
	if _, ok := envMap["XDG_RUNTIME_DIR"]; !ok {
		envMap["XDG_RUNTIME_DIR"] = fmt.Sprintf("/run/user/%d", os.Getuid())
	}

	// TERM must be xterm-256color so that ANSI sequences, 256-colour palettes,
	// and cursor-movement codes work correctly in xterm.js.
	envMap["TERM"] = "xterm-256color"

	shellEnv := make([]string, 0, len(envMap))
	for k, v := range envMap {
		shellEnv = append(shellEnv, k+"="+v)
	}

	// Spawn the shell as a login shell (-l) so ~/.bashrc / ~/.profile are loaded.
	cmd := exec.CommandContext(r.Context(), shell, "-l")
	cmd.Env = shellEnv

	// Start the shell inside a PTY.
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Printf("host/ws: pty.Start failed: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte("\r\nFailed to start shell: "+err.Error()+"\r\n"))
		return
	}
	defer func() {
		ptmx.Close()
		cmd.Wait() //nolint:errcheck
	}()

	// PTY → WebSocket: stream all shell output to the browser.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				if err2 := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err2 != nil {
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					log.Printf("host/ws: pty read: %v", err)
				}
				conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
		}
	}()

	// WebSocket → PTY: forward keystrokes and handle resize messages.
	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// Try to decode resize messages (always sent as text JSON).
		if msgType == websocket.TextMessage {
			var rm resizeMsg
			if json.Unmarshal(msg, &rm) == nil && rm.Type == "resize" && rm.Cols > 0 && rm.Rows > 0 {
				setWinsize(ptmx, rm.Cols, rm.Rows)
				continue
			}
		}

		// Everything else is raw input for the shell.
		if _, err := ptmx.Write(msg); err != nil {
			log.Printf("host/ws: pty write: %v", err)
			break
		}
	}
}

// setWinsize sends a SIGWINCH to the PTY to resize the terminal window.
func setWinsize(f *os.File, cols, rows uint16) {
	type winsize struct {
		Rows uint16
		Cols uint16
		Xpix uint16
		Ypix uint16
	}
	ws := winsize{Rows: rows, Cols: cols}
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(),
		uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&ws)))
}
