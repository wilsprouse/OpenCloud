package api

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/api/types"
)

type BuildImageRequest struct {
	Dockerfile string `json:"dockerfile"`
	ImageName  string `json:"imageName"`
	Context    string `json:"context"`
	NoCache    bool   `json:"nocache"`
	Platform   string `json:"platform"`
}

func BuildImage(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Building Image!")
	var req BuildImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	fmt.Println("Building Image2!")

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		http.Error(w, "Failed to connect to Docker daemon", http.StatusInternalServerError)
		return
	}

	// Create a tar archive of the build context
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)

	// Write Dockerfile
	tw.WriteHeader(&tar.Header{
		Name: "Dockerfile",
		Size: int64(len(req.Dockerfile)),
	})
	tw.Write([]byte(req.Dockerfile))
	tw.Close()
	fmt.Println("Building Image3!")

	// Prepare build options
	opts := types.ImageBuildOptions{
		Tags:           []string{req.ImageName},
		NoCache:        req.NoCache,
		Platform:       req.Platform,
		Remove:         true,
		SuppressOutput: false,
	}
	fmt.Println("Building Image3.1!")

	// Trigger build
	buildResponse, err := cli.ImageBuild(ctx, buf, opts)
	fmt.Println("Building Image3.1.1!")
	if err != nil {
		fmt.Println("Build error: %v", err)
		http.Error(w, fmt.Sprintf("Build error: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Println("Building Image3.1.2!")
	defer buildResponse.Body.Close()
	fmt.Println("Building Image3.2!")

	// Stream Docker's output to the client or log
	io.Copy(os.Stdout, buildResponse.Body)
	fmt.Println("Building Image4!")

	// Send success back
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "Build complete",
		"image":  req.ImageName,
	})

}

func GetContainerRegistry(w http.ResponseWriter, r *http.Request) {

	ctx := context.Background()

    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        panic(err)
    }

 //   images, err := cli.ImageList(ctx, types.ImageListOptions{
	//images, err := cli.ImageList(ctx, types.ImageListOptions{
	images, err := cli.ImageList(ctx, image.ListOptions{
        All: true, // include intermediate images
    })
    if err != nil {
        panic(err)
    }

    /*for _, img := range images {
		fmt.Printf("ID: %s\n", img.ID[7:19])
		fmt.Printf("RepoTags: %v\n", img.RepoTags)
		fmt.Printf("RepoDigests: %v\n", img.RepoDigests)
		fmt.Printf("Created: %d\n", img.Created)
		fmt.Printf("Size: %.2f MB\n", float64(img.Size)/1_000_000)
		fmt.Printf("Virtual Size: %.2f MB\n", float64(img.VirtualSize)/1_000_000)
		fmt.Printf("Labels: %v\n", img.Labels)
		fmt.Printf("Containers: %d\n\n", img.Containers)
    }*/

	// Encode the images as JSON and write to response
	if err := json.NewEncoder(w).Encode(images); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}
