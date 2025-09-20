package api

import (
	"fmt"
	"net/http"
	"encoding/json"
	"golang.org/x/sys/unix"
	"os"
)


func GetSystemMetrics(w http.ResponseWriter, r *http.Request) {
	/*
		This function gets system metrics to populate the dashboard with. 
		These metrics include CPU usage, Memory, and Disk Usage

		This function returns a json payload of the metrics it collects
	*/

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

	// Convert free bytes to GB with decimals
	freeBytes := float64(statfs.Bavail) * float64(statfs.Bsize)
	freeGB := freeBytes / (1000 * 1000 * 1000) // divide by GiB (binary GB)

	//freeBytes := ((statfs.Bavail * uint64(statfs.Bsize)) / (1000 * 1000 * 1000))


	ret := map[string]interface{}{
		"STORAGE": fmt.Sprintf("%.2f", freeGB),
		"CPU": 100,
		"MEMORY": 101,
	} // Return Value


	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(ret)

}
