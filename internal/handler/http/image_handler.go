package http

import (
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	MaxUploadSize = 10 << 20 // 10 MB
	MaxWidth      = 600
	UploadDir     = "/tmp"
)

type ImageHandler struct {
	uploadDir string
}

func NewImageHandler() *ImageHandler {
	// Ensure upload directory exists
	if _, err := os.Stat(UploadDir); os.IsNotExist(err) {
		_ = os.MkdirAll(UploadDir, 0755)
	}
	return &ImageHandler{
		uploadDir: UploadDir,
	}
}

func (h *ImageHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	// 1. Parse Multipart Form
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize)
	if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "File too large or invalid form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file")
		return
	}
	defer file.Close()

	// 2. Validate Content Type (Basic check + decoding)
	// Read first 512 bytes for content type detection
	buff := make([]byte, 512)
	if _, err := file.Read(buff); err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to read file")
		return
	}
	// Reset file pointer
	file.Seek(0, 0)

	// Detect content type
	fileType := http.DetectContentType(buff)
	if !strings.HasPrefix(fileType, "image/") {
		h.respondWithError(w, http.StatusBadRequest, "Invalid file format: not an image")
		return
	}

	// 3. Decode & Resize
	img, err := imaging.Decode(file)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Failed to decode image")
		return
	}

	var finalImg image.Image = img
	if img.Bounds().Dx() > MaxWidth {
		// Optimization: high-quality resizing
		finalImg = imaging.Resize(img, MaxWidth, 0, imaging.Lanczos)
	}

	// 4. Generate Filename & Save
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg" // Default to jpg if no extension (or infer from content type)
	}

	newFileName := uuid.New().String() + "_" + header.Filename
	// Clean filename to remove directory paths just in case, though uuid + original should be safe-ish
	// Better: Use just UUID + extension to be totally safe, but requirements say UUID + OriginalName.
	// We sanitize original name effectively by prepending UUID and taking basename if we wanted to be strict,
	// but here we follow the Java logic: uuid + original.
	// Let's ensure no path traversal in the "original" part if it interprets paths?
	// actually filepath.Base is good practice.
	newFileName = uuid.New().String() + "_" + filepath.Base(header.Filename)

	destPath := filepath.Join(h.uploadDir, newFileName)

	// Save as JPEG if it was resized or just generally to normalize?
	// The Java code saves as JPG if resized used ImageIO.write(..., "jpg", ...).
	// If it wasn't resized, it just did Files.copy().
	// To match Java logic exactly: Copy if <= 600, Resize & Save as JPG if > 600.
	// BUT, mixing formats is annoying. Let's stick to saving whatever we have, OR enforce JPG for resized.
	// The prompt asked for optimizations. Converting everything to a consistent format (like JPG or WebP) is usually better.
	// But let's respect the input format for small images to avoid unnecessary re-encoding loss,
	// and use JPEG for resized ones to ensure compression.

	// Actually, `imaging.Save` handles formats based on extension.
	// Let's just save.

	err = imaging.Save(finalImg, destPath)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to save image")
		return
	}

	// Response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"name": newFileName})
}

func (h *ImageHandler) GetImage(w http.ResponseWriter, r *http.Request) {
	fileName := chi.URLParam(r, "fileName")
	if fileName == "" {
		h.respondWithError(w, http.StatusBadRequest, "Filename is required")
		return
	}

	// Security: Prevent path traversal
	cleanName := filepath.Base(fileName)

	// If the user requests ".." or similar, Base() strips it or returns just the last part.
	// This forces the file to be looked up ONLY in h.uploadDir.
	targetPath := filepath.Join(h.uploadDir, cleanName)

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		h.respondWithError(w, http.StatusNotFound, "Image not found")
		return
	}

	// Check for resize parameter
	widthParam := r.URL.Query().Get("width")
	if widthParam != "" {
		var width int
		if _, err := fmt.Sscanf(widthParam, "%d", &width); err == nil && width > 0 {
			// Limit max width to prevent abuse (e.g., trying to generate 10k pixel images)
			if width > 2000 {
				width = 2000
			}

			// Open original file
			file, err := os.Open(targetPath)
			if err != nil {
				h.respondWithError(w, http.StatusInternalServerError, "Failed to open image")
				return
			}
			defer file.Close()

			img, err := imaging.Decode(file)
			if err != nil {
				h.respondWithError(w, http.StatusInternalServerError, "Failed to decode image")
				return
			}

			// Resize
			resizedImg := imaging.Resize(img, width, 0, imaging.Lanczos)

			// Encode to JPEG and serve
			w.Header().Set("Content-Type", "image/jpeg") // Defaulting to JPEG for resized images
			if err := jpeg.Encode(w, resizedImg, nil); err != nil {
				// If we already wrote the header, this might be moot, but good practice to log or handle
				fmt.Println("Error encoding resized image:", err)
			}
			return
		}
	}

	// Default: Serve original file (optimized with caching/ranges)
	http.ServeFile(w, r, targetPath)
}

func (h *ImageHandler) respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
