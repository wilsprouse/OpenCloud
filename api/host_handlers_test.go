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
