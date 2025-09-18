package main

import (
	"net/http"
	"github.com/WavexSoftware/OpenCloud/api"
)

func main() { 
	http.HandleFunc("/get-system-metrics", api.GetSystemMetrics)

	http.ListenAndServe(":3030", nil)
}