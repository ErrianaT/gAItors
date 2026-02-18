package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"uf/mcp/mcp-client/handlers"
	"uf/mcp/mcp-client/utils"
	"uf/mcp/pkg/mcp"
)

// CORS middleware wrapper
func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Required CORS headers for browser-based UI
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")

		// Handle preflight OPTIONS request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Continue to actual handler
		next.ServeHTTP(w, r)
	}
}

func main() {
	// Serve static files
	http.Handle("/", http.FileServer(http.Dir("./AppRoot/static")))

	// REST API endpoint for chat (wrapped with CORS)
	http.HandleFunc("/chat", withCORS(handlers.ChatHandler))

	// Signed URL endpoint for GCS uploads
	http.HandleFunc("/gcs/signed-url", withCORS(handlers.GenerateSignedURL))

	// Bring up the http listener
	address := ":8080"
	if a, ok := os.LookupEnv("WEB_PORT"); ok {
		address = fmt.Sprintf(":%s", a)
	}

	log.Printf("Listening on %s", address)

	if err := http.ListenAndServe(address, nil); err != nil {
		panic(err)
	}
}

func init() {
	// Initialize the list of tools available to this application
	utils.InitializeConfiguration()
	mcp.InitalizeTools()
}
