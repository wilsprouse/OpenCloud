package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
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
		} else {
			// For requests with an Origin header, we need to validate and set CORS headers
			// We allow requests from the same host on ports 80 (nginx), 3000 (direct frontend), or 443 (https)
			// This prevents arbitrary external sites from making requests while allowing
			// legitimate access from the frontend served by this server
			
			allowed := isAllowedOrigin(origin, r.Host)
			
			if allowed {
				// Allow the request with CORS headers
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				
				// Handle preflight request
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			} else {
				// Unauthorized origin - reject the request
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("CORS policy: origin not allowed"))
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// isAllowedOrigin checks if the origin is from the same host on an allowed port (80, 3000, or 443)
func isAllowedOrigin(origin string, requestHost string) bool {
	// Parse the origin URL
	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}
	
	// Extract host from origin (without port)
	originHost := originURL.Hostname()
	
	// Extract host from request using standard library (handles IPv6 correctly)
	requestHostname, _, err := net.SplitHostPort(requestHost)
	if err != nil {
		// If there's no port, use the whole string as hostname
		requestHostname = requestHost
	}
	
	// Verify the origin is from the same host
	// This ensures only requests from frontends served by this server are allowed
	if originHost != requestHostname {
		// Special case: allow localhost/127.0.0.1 only if request is also to localhost
		isOriginLocalhost := originHost == "localhost" || originHost == "127.0.0.1"
		isRequestLocalhost := requestHostname == "localhost" || requestHostname == "127.0.0.1"
		
		if !(isOriginLocalhost && isRequestLocalhost) {
			return false
		}
	}
	
	// Get the port from origin (defaults to 80 for http, 443 for https)
	port := originURL.Port()
	if port == "" {
		if originURL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	
	// Allow port 80 (nginx), 3000 (direct frontend access), or 443 (https nginx)
	return port == "80" || port == "3000" || port == "443"
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
