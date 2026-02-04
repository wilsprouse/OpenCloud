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
		// Get the origin from the request
		origin := r.Header.Get("Origin")
		
		// Browsers only send the Origin header for cross-origin requests.
		// When nginx serves both the frontend and proxies API requests on the same host:port,
		// browsers treat these as same-origin requests and don't include the Origin header.
		// Therefore, requests without an Origin header are from nginx (production) or same-origin.
		
		if origin == "" {
			// Same-origin request (from nginx proxy in production)
			// No CORS headers needed - browser already allows same-origin requests
			// Don't set any CORS headers
		} else if origin == "http://localhost:3000" {
			// Development environment - allow localhost with credentials
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {
			// Unauthorized origin - reject the request
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("CORS policy: origin not allowed"))
			return
		}
		
		// Set allowed methods and headers for CORS requests
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
