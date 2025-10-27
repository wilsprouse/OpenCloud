package api

import (
	"fmt"
	"net/http"
	"context"
	"encoding/json"
	"github.com/docker/docker/api/types/image"
    "github.com/docker/docker/client"
)

func GetContainerRegistry(w http.ResponseWriter, r *http.Request) {

	ctx := context.Background()

    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        panic(err)
    }

    images, err := cli.ImageList(ctx, image.ListOptions{
        All: true, // include intermediate images
    })
    if err != nil {
        panic(err)
    }

    for _, img := range images {
		fmt.Printf("ID: %s\n", img.ID[7:19])
		fmt.Printf("RepoTags: %v\n", img.RepoTags)
		fmt.Printf("RepoDigests: %v\n", img.RepoDigests)
		fmt.Printf("Created: %d\n", img.Created)
		fmt.Printf("Size: %.2f MB\n", float64(img.Size)/1_000_000)
		fmt.Printf("Virtual Size: %.2f MB\n", float64(img.VirtualSize)/1_000_000)
		fmt.Printf("Labels: %v\n", img.Labels)
		fmt.Printf("Containers: %d\n\n", img.Containers)
    }

	// Encode the images as JSON and write to response
	if err := json.NewEncoder(w).Encode(images); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}
