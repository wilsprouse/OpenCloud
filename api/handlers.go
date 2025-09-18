package api

import (
	"fmt"
	"net/http"
	"golang.org/x/sys/unix"
	"os"
)

func GetSystemMetrics(w http.ResponseWriter, r *http.Request) {
	"""
	This function gets system metrics to populate the dashboard with. 
	These metrics include CPU usage, Memory, and Disk Usage

	This function returns a json payload of the metrics it collects
	"""
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting pwd: %v\n", err)
		return
	}

	var statfs unix.Statfs_t
	err = unix.Statfs(wd, &statfs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting statfs for %s: %v\n", wd, err)
		return
	}

	freeBytes := statfs.Bavail * uint64(statfs.Bsize)

	fmt.Fprintf(w, "Hello, Go HTTP Server! You have %d bytes avail", freeBytes)

}