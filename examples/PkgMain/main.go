package main

import (
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		htmlTemplate := `<html>
<head><title>Health Check</title></head>
<body>
<h1>Service is Healthy</h1>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(htmlTemplate))
	})
	http.ListenAndServe(":8080", mux)
}
