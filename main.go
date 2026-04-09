package main

import (
	"fmt"
	"github.com/WavexSoftware/OpenCloud/api"
	computeapi "github.com/WavexSoftware/OpenCloud/api/compute"
	storageapi "github.com/WavexSoftware/OpenCloud/api/storage"
	"github.com/WavexSoftware/OpenCloud/service_ledger"
	"github.com/WavexSoftware/OpenCloud/utils"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
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

	// Verify the origin is from the same host or is trusted
	// This ensures only requests from frontends served by this server are allowed

	// Extract host from request to check if it's localhost
	isRequestLocalhost := isLocalhost(requestHostname)

	// IMPORTANT: If the request comes TO localhost, trust it
	// This handles Next.js rewrites where the frontend at http://IP:3000 rewrites
	// API requests to http://localhost:3030, causing Origin != Host mismatch
	//
	// Security: This is safe because:
	// 1. The backend ONLY listens on localhost:3030 (not on external interfaces)
	// 2. Only local processes (nginx, Next.js) can reach localhost:3030
	// 3. External attackers cannot directly access localhost:3030
	// 4. Port validation (80, 3000, 443) still applies (see lines 104-113 below)
	//
	// Note: In containerized environments, ensure proper network configuration
	// to maintain localhost isolation
	if isRequestLocalhost {
		// Request is to localhost (via Next.js rewrite or nginx proxy)
		// Allow any origin on valid ports (validated below)
	} else if originHost == requestHostname {
		// Exact hostname match (e.g., both are the same IP or domain) - allow
	} else {
		// Hostname mismatch and not to localhost - reject
		return false
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

	// Allow specific ports where the frontend is accessible:
	// - 80: nginx reverse proxy (production)
	//       Note: If using HTTPS, configure nginx to redirect HTTP to HTTPS
	// - 3000: direct Next.js frontend access (development/fallback)
	// - 443: nginx with HTTPS (production with SSL)
	return port == "80" || port == "3000" || port == "443"
}

// isLocalhost checks if a hostname is a localhost variant
// Note: url.URL.Hostname() automatically strips brackets from IPv6 addresses,
// so we only need to check '::1' without brackets
func isLocalhost(hostname string) bool {
	// Normalize to lowercase for case-insensitive comparison
	h := strings.ToLower(hostname)
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
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
	mux.HandleFunc("/user/login", api.Login)
	mux.HandleFunc("/user/get-auth/", api.RefreshAuth)
	mux.HandleFunc("/get-server-metrics", api.GetSystemMetrics)
	mux.HandleFunc("/get-containers", computeapi.GetContainers)
	mux.HandleFunc("/get-images", storageapi.GetContainerRegistry)
	mux.HandleFunc("/list-blob-buckets", storageapi.ListBlobBuckets)
	mux.HandleFunc("/get-blobs", storageapi.GetBlobBuckets)
	mux.HandleFunc("/create-bucket", storageapi.CreateBucket)
	mux.HandleFunc("/upload-object", storageapi.UploadObject)
	mux.HandleFunc("/delete-object", storageapi.DeleteObject)
	mux.HandleFunc("/delete-bucket", storageapi.DeleteBucket)
	mux.HandleFunc("/download-object", storageapi.DownloadObject)
	mux.HandleFunc("/rename-bucket", storageapi.RenameBucket)
	mux.HandleFunc("/list-functions", computeapi.ListFunctions)
	mux.HandleFunc("/invoke-function", computeapi.InvokeFunction)
	mux.HandleFunc("/create-function", computeapi.CreateFunction)
	mux.HandleFunc("/delete-function", computeapi.DeleteFunction)
	mux.HandleFunc("/update-function/", computeapi.UpdateFunction)
	mux.HandleFunc("/get-function-logs/", computeapi.GetFunctionLogs)
	mux.HandleFunc("/get-service-status", service_ledger.GetServiceStatusHandler)
	mux.HandleFunc("/enable-service", service_ledger.EnableServiceHandler)
	mux.HandleFunc("/enable-service-stream", service_ledger.EnableServiceStreamHandler)
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
	mux.HandleFunc("/build-image", storageapi.BuildImage)
	mux.HandleFunc("/build-image-stream", storageapi.BuildImageStream)
	mux.HandleFunc("/delete-image", storageapi.DeleteImage)
	mux.HandleFunc("/pull-image", storageapi.PullImage)
	mux.HandleFunc("/pull-image-stream", storageapi.PullImageStream)
	mux.HandleFunc("/get-image", storageapi.GetImage)
	mux.HandleFunc("/get-image-logs", storageapi.GetImageLogs)
	mux.HandleFunc("/delete-container", computeapi.DeleteContainer)
	mux.HandleFunc("/get-container", computeapi.GetContainer)
	mux.HandleFunc("/container-logs", computeapi.GetContainerLogs)
	mux.HandleFunc("/containers/", computeapi.ContainerAction)
	mux.HandleFunc("/pull-and-run", computeapi.PullAndRun)
	mux.HandleFunc("/pull-and-run-stream", computeapi.PullAndRunStream)
	mux.HandleFunc("/update-container", computeapi.UpdateContainer)
	mux.HandleFunc("/", computeapi.GetFunction)

	// Wrap all routes with CORS middleware
	handler := withCORS(mux)

	fmt.Println("Server running on localhost:3030")
	// IMPORTANT: Only listen on localhost for security
	// External access should go through nginx (port 80/443) or Next.js (port 3000)
	http.ListenAndServe("localhost:3030", handler)
}
