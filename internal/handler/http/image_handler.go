package http

import (
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"log"
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

	// DefaultUploadDir es relativo al directorio de arranque, que siempre es
	// Backend/ porque main.go carga config.yaml con ruta relativa.
	//
	// Antes era "/tmp", que fallaba por los dos lados: en Windows esa ruta no
	// existe, y en el contenedor la borra cualquier reinicio, asi que las
	// imagenes subidas desaparecian en el siguiente despliegue. El directorio
	// real se configura con storage.uploads_dir y en produccion debe apuntar a
	// un volumen montado.
	DefaultUploadDir = "uploads"
)

type ImageHandler struct {
	uploadDir string
}

func NewImageHandler(uploadDir string) *ImageHandler {
	if strings.TrimSpace(uploadDir) == "" {
		uploadDir = DefaultUploadDir
	}
	// MkdirAll no falla si ya existe. Si falla de verdad —permisos, disco— se
	// registra aqui: descubrirlo en el primer intento de subida, con un 500 y
	// sin contexto, cuesta mucho mas.
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("[IMAGENES] no se pudo crear el directorio de subidas %q: %v", uploadDir, err)
	}
	return &ImageHandler{
		uploadDir: uploadDir,
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

	// 4. Nombre del archivo y guardado.
	//
	// filepath.Base descarta cualquier ruta que venga en el nombre original,
	// que es la via clasica para escribir fuera del directorio de subidas, y el
	// UUID delante evita que dos personas que suban "foto.jpg" se pisen.
	nombre := filepath.Base(header.Filename)
	if filepath.Ext(nombre) == "" {
		// imaging.Save elige el formato por la extension: sin ella devuelve
		// "unsupported image format" y la subida moria con un 500 opaco.
		nombre += ".jpg"
	}
	newFileName := uuid.New().String() + "_" + nombre

	destPath := filepath.Join(h.uploadDir, newFileName)

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
