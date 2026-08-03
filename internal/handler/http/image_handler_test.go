package http

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestImageHandler_UploadImage(t *testing.T) {
	// Setup temporary upload directory
	tmpDir, err := os.MkdirTemp("", "img_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	handler := &ImageHandler{uploadDir: tmpDir}

	// Create a dummy image
	img := image.NewRGBA(image.Rect(0, 0, 800, 600)) // Width 800 > 600, should trigger resize
	for x := 0; x < 800; x++ {
		for y := 0; y < 600; y++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("file", "test_image.jpg")
	if err != nil {
		t.Fatal(err)
	}
	err = jpeg.Encode(fw, img, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	req := httptest.NewRequest("POST", "/upload", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()

	handler.UploadImage(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", res.Status)
	}

	var respMap map[string]string
	json.NewDecoder(res.Body).Decode(&respMap)
	fileName, ok := respMap["name"]
	if !ok {
		t.Fatal("Response did not contain 'name'")
	}

	// Verify file exists
	filePath := filepath.Join(tmpDir, fileName)
	savedFile, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("File was not saved: %v", err)
	}
	defer savedFile.Close()

	// Verify resize
	savedImg, _, err := image.Decode(savedFile)
	if err != nil {
		t.Fatal(err)
	}
	if savedImg.Bounds().Dx() > 600 {
		t.Errorf("Image was not resized. Width: %d", savedImg.Bounds().Dx())
	}
}

func TestImageHandler_GetImage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "img_test_get")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fileName := "test.txt"
	content := []byte("hello world")
	os.WriteFile(filepath.Join(tmpDir, fileName), content, 0644)

	handler := &ImageHandler{uploadDir: tmpDir}

	// Test valid file
	req := httptest.NewRequest("GET", "/images/{fileName}", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fileName", fileName)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.GetImage(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", rec.Code)
	}
	if rec.Body.String() != string(content) {
		t.Errorf("Expected content %s, got %s", string(content), rec.Body.String())
	}

	// Test Path Traversal Protection
	reqTraversal := httptest.NewRequest("GET", "/images/{fileName}", nil)
	rctxTraversal := chi.NewRouteContext()
	rctxTraversal.URLParams.Add("fileName", "../secret.txt")
	reqTraversal = reqTraversal.WithContext(context.WithValue(reqTraversal.Context(), chi.RouteCtxKey, rctxTraversal))

	recTraversal := httptest.NewRecorder()
	handler.GetImage(recTraversal, reqTraversal)

	// Should either be 404 (because it looks for "secret.txt" in uploadDir) or 400.
	// Our logic: filepath.Base("../secret.txt") -> "secret.txt".
	// It basically ignores the directory traversal and looks in the safe dir.
	// Since "secret.txt" doesn't exist in tmpDir, it should return 404.
	if recTraversal.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found for traversal attempt (sanitized to non-existent file), got %d", recTraversal.Code)
	}

	// Test Resize on Get
	reqResize := httptest.NewRequest("GET", "/images/{fileName}?width=100", nil)
	rctxResize := chi.NewRouteContext()
	rctxResize.URLParams.Add("fileName", "test_image.jpg") // Need a real image here
	reqResize = reqResize.WithContext(context.WithValue(reqResize.Context(), chi.RouteCtxKey, rctxResize))

	// Create a real image for this test
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	imgFile, _ := os.Create(filepath.Join(tmpDir, "test_image.jpg"))
	jpeg.Encode(imgFile, img, nil)
	imgFile.Close()

	recResize := httptest.NewRecorder()
	handler.GetImage(recResize, reqResize)

	if recResize.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for resize, got %d", recResize.Code)
	}

	resized, _, err := image.Decode(recResize.Body)
	if err != nil {
		t.Fatalf("Failed to decode resized image: %v", err)
	}
	if resized.Bounds().Dx() != 100 {
		t.Errorf("Expected width 100, got %d", resized.Bounds().Dx())
	}
}
