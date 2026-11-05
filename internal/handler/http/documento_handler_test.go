package http

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// El archivo del material de un workshop: solo PDF, Word, Excel, PowerPoint o
// ZIP, comprobando el contenido y no solo la extensión, y servido como
// descarga.

func subir(t *testing.T, h *DocumentHandler, nombre string, contenido []byte) *httptest.ResponseRecorder {
	t.Helper()
	var cuerpo bytes.Buffer
	mw := multipart.NewWriter(&cuerpo)
	parte, _ := mw.CreateFormFile("file", nombre)
	parte.Write(contenido)
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/documents/upload", &cuerpo)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.UploadDocument(rec, req)
	return rec
}

func TestSubirDocumento(t *testing.T) {
	dir := t.TempDir()
	h := NewDocumentHandler(dir)

	t.Run("un PDF de verdad se guarda", func(t *testing.T) {
		rec := subir(t, h, "Presentación final (v2).pdf", []byte("%PDF-1.7\ncontenido"))
		if rec.Code != http.StatusOK {
			t.Fatalf("HTTP %d: %s", rec.Code, rec.Body)
		}
		var r struct {
			Name         string `json:"name"`
			OriginalName string `json:"originalName"`
			Size         int64  `json:"size"`
		}
		json.Unmarshal(rec.Body.Bytes(), &r)
		if !strings.HasSuffix(r.Name, "_Presentaci-n-final--v2-.pdf") || r.OriginalName != "Presentación final (v2).pdf" || r.Size != 18 {
			t.Errorf("respuesta = %+v", r)
		}
		if _, err := os.Stat(filepath.Join(dir, "documentos", r.Name)); err != nil {
			t.Errorf("no quedó en disco: %v", err)
		}
	})

	t.Run("un docx es un ZIP por dentro", func(t *testing.T) {
		if rec := subir(t, h, "notas.docx", []byte("PK\x03\x04resto")); rec.Code != http.StatusOK {
			t.Fatalf("HTTP %d: %s", rec.Code, rec.Body)
		}
	})

	rechazos := map[string][]byte{
		"falso.pdf":  []byte("MZ ejecutable"), // la extensión no basta
		"virus.exe":  []byte("MZ ejecutable"),
		"imagen.png": []byte("\x89PNG"),
		"sin-ext":    []byte("%PDF-1.7"),
	}
	for nombre, contenido := range rechazos {
		if rec := subir(t, h, nombre, contenido); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: HTTP %d, se esperaba 400", nombre, rec.Code)
		}
	}
}

func TestDescargarDocumento(t *testing.T) {
	dir := t.TempDir()
	h := NewDocumentHandler(dir)
	nombre := "0f8fad5b-d9cb-469f-a165-70867728950e_Agenda.pdf"
	os.WriteFile(filepath.Join(dir, "documentos", nombre), []byte("%PDF-1.7"), 0644)
	os.WriteFile(filepath.Join(dir, "secreto.txt"), []byte("no"), 0644)

	pedir := func(param string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/documents/x", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("fileName", param)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rec := httptest.NewRecorder()
		h.GetDocument(rec, req)
		return rec
	}

	rec := pedir(nombre)
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP %d", rec.Code)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="Agenda.pdf"` {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q", ct)
	}

	// Ni salir de la carpeta ni archivos que no existen.
	for _, p := range []string{"..%2Fsecreto.txt", "../secreto.txt", "no-existe.pdf"} {
		if rec := pedir(p); rec.Code != http.StatusNotFound {
			t.Errorf("%s: HTTP %d, se esperaba 404", p, rec.Code)
		}
	}
}
