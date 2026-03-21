package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/podman/v5/pkg/bindings"
)

const podmanSocket = "/run/podman/podman.sock"

func podmanConnection(ctx context.Context) (context.Context, error) {
	var errs []string

	for _, uri := range podmanSocketCandidates() {
		conn, err := bindings.NewConnection(ctx, uri)
		if err == nil {
			return conn, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", uri, err))
	}

	if len(errs) == 0 {
		return nil, fmt.Errorf("no Podman connection candidates available")
	}

	return nil, fmt.Errorf("failed to connect to Podman: %s", strings.Join(errs, "; "))
}

func hasPodmanSocket() bool {
	for _, uri := range podmanSocketCandidates() {
		socketPath := podmanSocketPath(uri)
		if socketPath == "" {
			return true
		}
		if _, err := os.Stat(socketPath); err == nil {
			return true
		}
	}

	return false
}

func podmanSocketCandidates() []string {
	if containerHost := strings.TrimSpace(os.Getenv("CONTAINER_HOST")); containerHost != "" {
		return []string{containerHost}
	}

	candidates := make([]string, 0, 4)
	seen := make(map[string]struct{})

	add := func(uri string) {
		if uri == "" {
			return
		}
		if _, ok := seen[uri]; ok {
			return
		}
		seen[uri] = struct{}{}
		candidates = append(candidates, uri)
	}

	if xdgRuntimeDir := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR")); xdgRuntimeDir != "" {
		add("unix://" + filepath.Join(xdgRuntimeDir, "podman", "podman.sock"))
	}

	add("unix:///run/user/" + strconv.Itoa(os.Getuid()) + "/podman/podman.sock")
	// Root can also manage the shared system Podman service. Non-root callers stay
	// on their rootless store unless they explicitly override CONTAINER_HOST.
	if os.Geteuid() == 0 {
		add("unix://" + podmanSocket)
	}

	return candidates
}

func podmanSocketPath(uri string) string {
	switch {
	case strings.HasPrefix(uri, "unix://"):
		return strings.TrimPrefix(uri, "unix://")
	case strings.HasPrefix(uri, "unix:"):
		return strings.TrimPrefix(uri, "unix:")
	default:
		return ""
	}
}
