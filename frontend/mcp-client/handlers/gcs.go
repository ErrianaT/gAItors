package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

type SignedURLRequest struct {
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
}

func GenerateSignedURL(w http.ResponseWriter, r *http.Request) {
	var req SignedURLRequest
	json.NewDecoder(r.Body).Decode(&req)

	ctx := context.Background()

	// Load service-account.json
	creds := option.WithCredentialsFile("service-account.json")

	client, err := storage.NewClient(ctx, creds)
	if err != nil {
		http.Error(w, "Failed to create GCS client", 500)
		return
	}

	bucket := "one-stop"
	object := fmt.Sprintf("uploads/%s", req.FileName)

	// Load service account email + private key from JSON
	saBytes, err := os.ReadFile("service-account.json")
	if err != nil {
		http.Error(w, "Failed to read service-account.json", 500)
		return
	}

	type saKey struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
	}

	var key saKey
	json.Unmarshal(saBytes, &key)

	url, err := client.Bucket(bucket).SignedURL(object, &storage.SignedURLOptions{
		Method:         "PUT",
		Expires:        time.Now().Add(15 * time.Minute),
		ContentType:    req.ContentType,
		Scheme:         storage.SigningSchemeV4,
		GoogleAccessID: key.ClientEmail,
		PrivateKey:     []byte(key.PrivateKey),
	})
	if err != nil {
		fmt.Println("SIGNED URL ERROR:", err) // <-- ADD THIS
		http.Error(w, err.Error(), 500)       // <-- RETURN REAL ERROR
		return
	}

	gcsUri := fmt.Sprintf("gs://%s/%s", bucket, object)

	json.NewEncoder(w).Encode(map[string]string{
		"uploadUrl": url,
		"gcsUri":    gcsUri,
	})
}
