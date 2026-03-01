package api

import (
	"fmt"
	"net/http"
	"encoding/json"
	"golang.org/x/sys/unix"
	"os"
)

type Storage struct {
	UsedStorage      string `json:"UsedStorage"`
	AvailableStorage string `json:"AvailableStorage"`
	TotalStorage string `json:"TotalStorage"`
	PercentageUsed   string `json:"PercentageUsed"`
}

type Metrics struct {
	Storage Storage `json:"STORAGE"`
	CPU     int     `json:"CPU"`
	Memory  int     `json:"MEMORY"`
}

func getStorageMetrics() (float64, float64, float64) {
	/*
		getStorageMetrics retrieves disk usage statistics for the current working directory.

		Returns:
  			- usedGB:        The amount of storage currently used (in gigabytes)
  			- availableGB:   The amount of storage available (in gigabytes)
  			- totalGB:       The total storage capacity (in gigabytes)

		The function uses the unix.Statfs system call to gather filesystem information.
		If an error occurs (for example, failing to get the working directory or filesystem stats),
		the function prints the error to stderr and returns zeros for all values.
	*/

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting pwd: %v\n", err)
		return 0, 0, 0
	}

	var statfs unix.Statfs_t
	err = unix.Statfs(wd, &statfs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting statfs for %s: %v\n", wd, err)
		return 0, 0, 0
	}

	// Convert free bytes to GB with decimals
	freeBytes := float64(statfs.Bavail) * float64(statfs.Bsize)
	availableStorage := freeBytes / (1000 * 1000 * 1000) // divide by GiB (binary GB)	

	totalStorage := (float64(statfs.Blocks) * float64(statfs.Bsize)) / (1000 * 1000 * 1000)
	
	return (totalStorage-availableStorage), availableStorage, totalStorage
}


func GetSystemMetrics(w http.ResponseWriter, r *http.Request) {
	/*
		This function gets system metrics to populate the dashboard with. 
		These metrics include CPU usage, Memory, and Disk Usage

		This function returns a json payload of the metrics it collects
	*/

	usedStorage, availableStorage, totalStorage := getStorageMetrics()

	ret := Metrics{
		Storage: Storage{
			UsedStorage:      	fmt.Sprintf("%.2f", usedStorage),
			AvailableStorage: 	fmt.Sprintf("%.2f", availableStorage),
			TotalStorage:   	fmt.Sprintf("%.2f", totalStorage),
			PercentageUsed:   	fmt.Sprintf("%.2f", (usedStorage/(totalStorage))*100),
		},
		CPU:    100,
		Memory: 101,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(ret)

}
