package main

import (
	"fmt"
	"log"
	"net/http"
	"github.com/WavexSoftware/OpenCloud/api"
	"github.com/WavexSoftware/OpenCloud/service_ledger"
	"github.com/WavexSoftware/OpenCloud/utils"
)

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000") // TODO: Pull from .env file
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// Initialize .opencloud directory structure
	if err := utils.InitializeOpenCloudDirectories(); err != nil {
		log.Fatalf("Failed to initialize OpenCloud directories: %v", err)
	}
	fmt.Println("OpenCloud directories initialized successfully")

	// Initialize service ledger with default services
	if err := service_ledger.InitializeServiceLedger(); err != nil {
		log.Fatalf("Failed to initialize service ledger: %v", err)
	}
	fmt.Println("Service ledger initialized successfully")

	mux := http.NewServeMux()
	mux.HandleFunc("/get-server-metrics", api.GetSystemMetrics)
	mux.HandleFunc("/get-containers", api.GetContainers)
	mux.HandleFunc("/get-images", api.GetContainerRegistry)
	mux.HandleFunc("/list-blob-containers", api.ListBlobContainers)
	mux.HandleFunc("/get-blobs", api.GetBlobBuckets)
	mux.HandleFunc("/create-container", api.CreateBucket)
	mux.HandleFunc("/upload-object", api.UploadObject)
	mux.HandleFunc("/delete-object", api.DeleteObject)
	mux.HandleFunc("/download-object", api.DownloadObject)
	mux.HandleFunc("/list-functions", api.ListFunctions)
	mux.HandleFunc("/invoke-function", api.InvokeFunction)
	mux.HandleFunc("/create-function", api.CreateFunction)
	mux.HandleFunc("/delete-function", api.DeleteFunction)
	mux.HandleFunc("/update-function/", api.UpdateFunction)
	mux.HandleFunc("/get-function-logs/", api.GetFunctionLogs)
	mux.HandleFunc("/get-service-status", service_ledger.GetServiceStatusHandler)
	mux.HandleFunc("/enable-service", service_ledger.EnableServiceHandler)
	mux.HandleFunc("/sync-pipelines", service_ledger.SyncPipelinesHandler)
	mux.HandleFunc("/sync-functions", service_ledger.SyncFunctionsHandler)
	mux.HandleFunc("/create-pipeline", api.CreatePipeline)
	mux.HandleFunc("/get-pipelines", api.GetPipelines)
	mux.HandleFunc("/get-pipeline/", api.GetPipeline)
	mux.HandleFunc("/update-pipeline/", api.UpdatePipeline)
	mux.HandleFunc("/delete-pipeline/", api.DeletePipeline)
	mux.HandleFunc("/run-pipeline/", api.RunPipeline)
	mux.HandleFunc("/stop-pipeline/", api.StopPipeline)
	mux.HandleFunc("/get-pipeline-logs/", api.GetPipelineLogs)
	mux.HandleFunc("/", api.GetFunction)
	//mux.HandleFunc("/build-image", api.BuildImage)

	// Wrap all routes with CORS middleware
	handler := withCORS(mux)

	fmt.Println("Server running on :3030")
	http.ListenAndServe(":3030", handler)
}
