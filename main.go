package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type response struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   map[string]string `json:"query,omitempty"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body,omitempty"`
}

func handleAll(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Copy headers into a simple map
	headers := make(map[string]string, len(r.Header))
	for name, values := range r.Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}

	// Copy query parameters
	query := make(map[string]string, len(r.URL.Query()))
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			query[key] = values[0]
		}
	}

	// Read body (safe for empty or chunked bodies)
	bodyBytes, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	// Handle OPTIONS (CORS / Allow)
	if r.Method == http.MethodOptions {
		w.Header().Set("Allow", "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS, TRACE")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Build response
	resp := response{
		Method:  r.Method,
		Path:    r.URL.Path,
		Query:   query,
		Headers: headers,
		Body:    string(bodyBytes),
	}

	// HEAD must not have a body
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("encode error: %v", err)
	}
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleAll)

	fmt.Println("Server listening on :8080")
	fmt.Println("Supported methods: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS, TRACE")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
