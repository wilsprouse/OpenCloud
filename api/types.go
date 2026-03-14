package api

// ImageInfo represents container image information compatible with the frontend.
// This structure maps containerd/Docker image metadata to a common format.
type ImageInfo struct {
	ID           string            `json:"Id"`
	RepoTags     []string          `json:"RepoTags"`
	RepoDigests  []string          `json:"RepoDigests"`
	Created      int64             `json:"Created"`
	Size         int64             `json:"Size"`
	VirtualSize  int64             `json:"VirtualSize"`
	Labels       map[string]string `json:"Labels"`
	Names        []string          `json:"Names"`
	Image        string            `json:"Image"`
	State        string            `json:"State"`
	Status       string            `json:"Status"`
}

// ContainerInfo represents a running (or stopped) container along with common
// runtime metrics such as memory usage and the host PID of its main process.
type ContainerInfo struct {
	// ID is the containerd container ID.
	ID string `json:"Id"`
	// Names holds human-readable names for the container. Falls back to the
	// container ID when no explicit name label is present.
	Names []string `json:"Names"`
	// Image is the name of the image the container was created from.
	Image string `json:"Image"`
	// State is the current lifecycle state reported by containerd
	// (e.g. "running", "stopped", "paused").
	State string `json:"State"`
	// Status is a human-readable status string including the host PID when
	// the container is running.
	Status string `json:"Status"`
	// Created is the Unix timestamp of when the container was created.
	Created int64 `json:"Created"`
	// Labels contains all metadata labels attached to the container.
	Labels map[string]string `json:"Labels"`
	// PID is the host-level process ID of the container's init process.
	// Zero when the container is not running.
	PID uint32 `json:"Pid"`
	// MemoryUsageBytes is the resident set size (RSS) of the container's
	// main process in bytes, read from the Linux /proc filesystem.
	// Zero when the container is not running or the value cannot be read.
	MemoryUsageBytes int64 `json:"MemoryUsageBytes"`
}
