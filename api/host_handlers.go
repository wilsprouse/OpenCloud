package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"strings"
)

// maxCommandLen is the maximum number of bytes accepted for a single shell
// command.  Commands longer than this are rejected to prevent accidental or
// deliberate denial-of-service through unbounded input.
const maxCommandLen = 4096

// cwdSentinel is a unique prefix injected at the end of every executed command
// so that the final working directory can be captured and forwarded to the UI
// as a special SSE event without appearing in normal command output.
const cwdSentinel = "__OC_CWD__"

// HostInfo contains basic host metadata used to render the terminal prompt.
type HostInfo struct {
	User     string `json:"user"`
	Hostname string `json:"hostname"`
	Cwd      string `json:"cwd"`
}

// ExecRequest is the JSON body accepted by the /host/exec endpoint.
type ExecRequest struct {
	// Command is the shell command to execute.
	Command string `json:"command"`
	// Cwd is the optional working directory in which to run the command.
	// A leading "~" is expanded to the user's home directory.
	Cwd string `json:"cwd"`
}

// shortenHome replaces the user's home-directory prefix in path with "~".
func shortenHome(path, home string) string {
	if home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+"/") {
		return "~" + path[len(home):]
	}
	return path
}

// resolveCwd expands a leading "~" in path to the user's home directory.
func resolveCwd(path string) string {
	if path == "" {
		return ""
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		if path == "~" {
			return home
		}
		return home + path[1:] // "~/foo" → "/home/user/foo"
	}
	return path
}

// GetHostInfo handles GET /host/info.
// It returns the current OS user, hostname, and working directory so the
// UI can render an accurate shell prompt (e.g. user@hostname:~$).
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
		cwd = shortenHome(wd, home)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HostInfo{
		User:     username,
		Hostname: hostname,
		Cwd:      cwd,
	})
}

// ExecuteHostCommand handles POST /host/exec.
// It runs the requested shell command on the host machine and streams the
// combined stdout/stderr output back to the caller as an SSE
// (text/event-stream) response so the UI can display output line-by-line.
//
// Security: OpenCloud is designed as a local infrastructure-management tool;
// the backend listens only on localhost:3030 and is accessed exclusively
// through the Next.js frontend proxy (which enforces session authentication).
// Commands are validated for length before execution to prevent denial-of-
// service through unbounded input.
//
// The command is wrapped to always capture the final working directory so that
// cd commands are reflected in the terminal prompt on the next keypress.
//
// Request body (JSON):
//
//	{"command": "<shell command>", "cwd": "<optional working directory>"}
//
// SSE events emitted:
//
//	data: <output line>
//	event: cwd\ndata: <new working directory>  – after every command
//	event: error\ndata: <error message>        – on non-zero exit
//	event: done\ndata: exit code 0             – on successful completion
func ExecuteHostCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Command == "" {
		http.Error(w, "command is required", http.StatusBadRequest)
		return
	}

	if len(req.Command) > maxCommandLen {
		http.Error(w, "command exceeds maximum allowed length", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sendLine := func(line string) {
		fmt.Fprintf(w, "data: %s\n\n", line)
		flusher.Flush()
	}

	home, _ := os.UserHomeDir()

	// Wrap the user command so the final working directory is always captured.
	// A leading '\n' in the printf ensures the sentinel line starts on its own
	// line even when the command does not end with a newline.
	// __oc_exit saves the user command's exit status so the sentinel printf
	// does not mask a failing command.
	wrappedCmd := fmt.Sprintf(
		`%s; __oc_exit=$?; printf '\n%s:%%s\n' "$(pwd)"; exit $__oc_exit`,
		req.Command, cwdSentinel,
	)

	// Use an io.Pipe so that both stdout and stderr from the subprocess are
	// merged into a single stream and forwarded to the SSE client line-by-line.
	pr, pw := io.Pipe()

	cmdErrCh := make(chan error, 1)
	go func() {
		cmd := exec.CommandContext(r.Context(), "sh", "-c", wrappedCmd)
		// Run the command in the directory the UI reports as current so that
		// relative paths and 'cd ..' work correctly across commands.
		if dir := resolveCwd(req.Cwd); dir != "" {
			cmd.Dir = dir
		}
		cmd.Stdout = pw
		cmd.Stderr = pw
		err := cmd.Run()
		pw.CloseWithError(err)
		cmdErrCh <- err
	}()

	// Forward each output line to the SSE client as it arrives.
	// Lines matching the sentinel are converted to a special "cwd" event so
	// the UI can update the prompt without displaying the sentinel itself.
	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, cwdSentinel+":") {
			newCwd := shortenHome(strings.TrimPrefix(line, cwdSentinel+":"), home)
			fmt.Fprintf(w, "event: cwd\ndata: %s\n\n", newCwd)
			flusher.Flush()
		} else {
			sendLine(line)
		}
	}

	if err := <-cmdErrCh; err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	fmt.Fprintf(w, "event: done\ndata: exit code 0\n\n")
	flusher.Flush()
}
