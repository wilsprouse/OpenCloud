package api

import (
	"fmt"
	"net/http"
)

func GetSystemMetrics(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, Go HTTP Server!")
}