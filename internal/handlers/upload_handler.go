package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxUploadSize = 5 << 20 // 5MB

var allowedImageExt = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

// UploadImageHandler accepts a multipart/form-data image upload (field name
// "file") and stores it under ./web/uploads/<folder>/, returning a URL the
// frontend can use directly (e.g. as student_card_url or profile_photo_url).
//
// This is intentionally public/unauthenticated for the "kyc" folder, since it
// needs to work during registration before the user has an account/token yet.
// folder is restricted to a fixed allow-list to prevent path traversal.
func UploadImageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		folder := r.URL.Query().Get("type")
		if folder != "kyc" && folder != "avatars" {
			folder = "kyc"
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			http.Error(w, "File too large (max 5MB) or invalid form data", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "No file uploaded (expected form field 'file')", http.StatusBadRequest)
			return
		}
		defer file.Close()

		ext := strings.ToLower(filepath.Ext(header.Filename))
		if !allowedImageExt[ext] {
			http.Error(w, "Only JPG, PNG, and WEBP images are allowed", http.StatusBadRequest)
			return
		}

		randBytes := make([]byte, 16)
		if _, err := rand.Read(randBytes); err != nil {
			http.Error(w, "Failed to generate filename", http.StatusInternalServerError)
			return
		}
		filename := hex.EncodeToString(randBytes) + ext

		destDir := filepath.Join("web", "uploads", folder)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			http.Error(w, "Server storage error", http.StatusInternalServerError)
			return
		}

		destPath := filepath.Join(destDir, filename)
		dest, err := os.Create(destPath)
		if err != nil {
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			return
		}
		defer dest.Close()

		if _, err := io.Copy(dest, file); err != nil {
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			return
		}

		url := fmt.Sprintf("/uploads/%s/%s", folder, filename)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"success","url":"%s"}`, url)
	}
}
