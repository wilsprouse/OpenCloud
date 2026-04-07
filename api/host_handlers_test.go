package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// responseRecorderFlusher embeds httptest.ResponseRecorder and implements
// http.Flusher so that the SSE handler can call Flush without panicking.
type responseRecorderFlusher struct {
	*httptest.ResponseRecorder
}

func (r *responseRecorderFlusher) Flush() {
	// httptest.ResponseRecorder buffers all writes; a no-op Flush is enough
	// to satisfy the http.Flusher interface for unit tests.
	r.ResponseRecorder.Flush()
}

// newFlusherRecorder returns a ResponseRecorder that also satisfies http.Flusher.
func newFlusherRecorder() *responseRecorderFlusher {
	return &responseRecorderFlusher{httptest.NewRecorder()}
}

// --- GetHostInfo tests -------------------------------------------------------

// TestGetHostInfoMethodNotAllowed verifies that POST requests are rejected.
func TestGetHostInfoMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/host/info", nil)
	w := httptest.NewRecorder()

	GetHostInfo(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestGetHostInfoReturnsJSON verifies that the endpoint returns a valid JSON
// payload containing at least user, hostname, and cwd fields.
func TestGetHostInfoReturnsJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/host/info", nil)
	w := httptest.NewRecorder()

	GetHostInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d — body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var info HostInfo
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if info.User == "" {
		t.Error("expected non-empty user")
	}
	if info.Hostname == "" {
		t.Error("expected non-empty hostname")
	}
	if info.Cwd == "" {
		t.Error("expected non-empty cwd")
	}
}

// --- shortenHome / resolveCwd unit tests ------------------------------------

// TestShortenHome verifies that the home-directory prefix is replaced with "~".
func TestShortenHome(t *testing.T) {
	cases := []struct {
		path, home, want string
	}{
		{"/home/alice", "/home/alice", "~"},
		{"/home/alice/projects", "/home/alice", "~/projects"},
		{"/tmp", "/home/alice", "/tmp"},
		{"/home/alicefoo", "/home/alice", "/home/alicefoo"}, // prefix-only, no slash
		{"", "/home/alice", ""},
	}
	for _, c := range cases {
		if got := shortenHome(c.path, c.home); got != c.want {
			t.Errorf("shortenHome(%q, %q) = %q, want %q", c.path, c.home, got, c.want)
		}
	}
}

// TestResolveCwd verifies that "~" and "~/..." are expanded correctly.
func TestResolveCwd(t *testing.T) {
	if got := resolveCwd(""); got != "" {
		t.Errorf("resolveCwd(%q) = %q, want %q", "", got, "")
	}
	if got := resolveCwd("/tmp"); got != "/tmp" {
		t.Errorf("resolveCwd(%q) = %q, want %q", "/tmp", got, "/tmp")
	}
	// "~" should expand to a non-empty absolute path.
	if got := resolveCwd("~"); got == "" || got == "~" {
		t.Errorf("resolveCwd(%q) = %q, want non-empty absolute path", "~", got)
	}
	// "~/foo" should expand without a double slash.
	if got := resolveCwd("~/foo"); strings.Contains(got, "~") {
		t.Errorf("resolveCwd(%q) = %q, still contains ~", "~/foo", got)
	}
}

// --- ExecuteHostCommand tests ------------------------------------------------

// TestExecuteHostCommandMethodNotAllowed verifies that GET requests are rejected.
func TestExecuteHostCommandMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/host/exec", nil)
	w := newFlusherRecorder()

	ExecuteHostCommand(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestExecuteHostCommandInvalidJSON verifies that malformed JSON is rejected.
func TestExecuteHostCommandInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/host/exec", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := newFlusherRecorder()

	ExecuteHostCommand(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestExecuteHostCommandMissingCommand verifies that an empty command is rejected.
func TestExecuteHostCommandMissingCommand(t *testing.T) {
	body, _ := json.Marshal(ExecRequest{Command: ""})
	req := httptest.NewRequest(http.MethodPost, "/host/exec", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := newFlusherRecorder()

	ExecuteHostCommand(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestExecuteHostCommandSuccess verifies that a simple echo command streams
// its output and terminates with an "event: done" SSE event.
func TestExecuteHostCommandSuccess(t *testing.T) {
	body, _ := json.Marshal(ExecRequest{Command: "echo hello"})
	req := httptest.NewRequest(http.MethodPost, "/host/exec", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := newFlusherRecorder()

	ExecuteHostCommand(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d — body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	response := w.Body.String()

	// The output line should appear as an SSE data event.
	if !strings.Contains(response, "data: hello") {
		t.Errorf("expected 'data: hello' in response, got:\n%s", response)
	}

	// A successful command must terminate with the done event.
	if !strings.Contains(response, "event: done") {
		t.Errorf("expected 'event: done' in response, got:\n%s", response)
	}

	// The sentinel line must NOT appear in the plain data output.
	if strings.Contains(response, "data: "+cwdSentinel) {
		t.Errorf("sentinel leaked into data output:\n%s", response)
	}
}

// TestExecuteHostCommandFailure verifies that a failing command produces an
// "event: error" SSE event.
func TestExecuteHostCommandFailure(t *testing.T) {
	body, _ := json.Marshal(ExecRequest{Command: "exit 1"})
	req := httptest.NewRequest(http.MethodPost, "/host/exec", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := newFlusherRecorder()

	ExecuteHostCommand(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 (SSE upgrade), got %d", w.Code)
	}

	response := w.Body.String()
	if !strings.Contains(response, "event: error") {
		t.Errorf("expected 'event: error' in response, got:\n%s", response)
	}
}

// TestExecuteHostCommandTooLong verifies that a command exceeding maxCommandLen
// is rejected before execution.
func TestExecuteHostCommandTooLong(t *testing.T) {
	longCmd := strings.Repeat("a", maxCommandLen+1)
	body, _ := json.Marshal(ExecRequest{Command: longCmd})
	req := httptest.NewRequest(http.MethodPost, "/host/exec", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := newFlusherRecorder()

	ExecuteHostCommand(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestExecuteHostCommandMultiLineOutput verifies that multi-line output is
// split correctly into individual SSE data events.
func TestExecuteHostCommandMultiLineOutput(t *testing.T) {
	body, _ := json.Marshal(ExecRequest{Command: "printf 'line1\\nline2\\nline3\\n'"})
	req := httptest.NewRequest(http.MethodPost, "/host/exec", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := newFlusherRecorder()

	ExecuteHostCommand(w, req)

	response := w.Body.String()
	for _, want := range []string{"data: line1", "data: line2", "data: line3"} {
		if !strings.Contains(response, want) {
			t.Errorf("expected %q in response, got:\n%s", want, response)
		}
	}
}

// TestExecuteHostCommandCwdTracking verifies that after a cd command the
// response contains an "event: cwd" SSE event with the new directory, and
// that the sentinel line does not appear as plain data.
func TestExecuteHostCommandCwdTracking(t *testing.T) {
	body, _ := json.Marshal(ExecRequest{Command: "cd /tmp"})
	req := httptest.NewRequest(http.MethodPost, "/host/exec", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := newFlusherRecorder()

	ExecuteHostCommand(w, req)

	response := w.Body.String()
	if !strings.Contains(response, "event: cwd") {
		t.Errorf("expected 'event: cwd' in response, got:\n%s", response)
	}
	if !strings.Contains(response, "data: /tmp") {
		t.Errorf("expected 'data: /tmp' in cwd event, got:\n%s", response)
	}
	// Sentinel must not be visible to the user.
	if strings.Contains(response, "data: "+cwdSentinel) {
		t.Errorf("sentinel leaked into data output:\n%s", response)
	}
}

// TestExecuteHostCommandCwdParam verifies that commands are run in the
// directory supplied via the Cwd request field.
func TestExecuteHostCommandCwdParam(t *testing.T) {
	body, _ := json.Marshal(ExecRequest{Command: "pwd", Cwd: "/tmp"})
	req := httptest.NewRequest(http.MethodPost, "/host/exec", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := newFlusherRecorder()

	ExecuteHostCommand(w, req)

	response := w.Body.String()
	if !strings.Contains(response, "data: /tmp") {
		t.Errorf("expected 'data: /tmp' in response (pwd output), got:\n%s", response)
	}
}

