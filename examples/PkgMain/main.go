package main

import (
	"net/http"

	"github.com/weeback/bko-bankpkg/pkg"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthcheck", pkg.HealthCheckHandler)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/permission-denied", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/permission-denied", pkg.PermissionDeniedHandler)

	http.ListenAndServe(":8080", mux)
}
