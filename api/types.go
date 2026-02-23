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
