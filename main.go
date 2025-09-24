package main

import (
	"fmt"
	"net/http"
	"github.com/WavexSoftware/OpenCloud/api"
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
	mux := http.NewServeMux()
	mux.HandleFunc("/get-server-metrics", api.GetSystemMetrics)

	// Wrap all routes with CORS middleware
	handler := withCORS(mux)

	fmt.Println("Server running on :3030")
	http.ListenAndServe(":3030", handler)
}
