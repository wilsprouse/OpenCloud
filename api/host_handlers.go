package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
)

// maxCommandLen is the maximum number of bytes accepted for a single shell
// command.  Commands longer than this are rejected to prevent accidental or
// deliberate denial-of-service through unbounded input.
const maxCommandLen = 4096

// ExecRequest is the JSON body accepted by the /host/exec endpoint.
type ExecRequest struct {
	Command string `json:"command"`
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
// Request body (JSON):
//
//	{"command": "<shell command>"}
//
// SSE events emitted:
//
//	data: <output line>
//	event: error\ndata: <error message>   – on non-zero exit or startup error
//	event: done\ndata: exit code 0        – on successful completion
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

	// Use an io.Pipe so that both stdout and stderr from the subprocess are
	// merged into a single stream.  The pipe reader is scanned line-by-line
	// in the handler goroutine and forwarded to the SSE client.
	pr, pw := io.Pipe()

	cmdErrCh := make(chan error, 1)
	go func() {
		cmd := exec.CommandContext(r.Context(), "sh", "-c", req.Command)
		// Route both stdout and stderr through the write end of the pipe so
		// that output arrives in the order it was produced.
		cmd.Stdout = pw
		cmd.Stderr = pw
		err := cmd.Run()
		// Close the write end with the run error (nil on success) so that the
		// scanner loop below terminates and the error can be inspected.
		pw.CloseWithError(err)
		cmdErrCh <- err
	}()

	// Forward each output line to the SSE client as it arrives.
	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		sendLine(scanner.Text())
	}

	if err := <-cmdErrCh; err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	fmt.Fprintf(w, "event: done\ndata: exit code 0\n\n")
	flusher.Flush()
}
