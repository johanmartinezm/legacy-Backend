package http

import (
	"applegacy/backend/internal/core/services"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// DocumentHandler sube y sirve el archivo del material de un workshop (parte A
// del plan de workshops). ImageHandler solo acepta imágenes y las redimensiona,
// así que los documentos van aparte: otra carpeta, otros tipos, y se sirven
// como descarga.
//
// Tipos admitidos (§5, decisión 6): PDF, Word, Excel, PowerPoint y ZIP, hasta
// 25 MB. Se comprueba la extensión **y** el contenido: un .pdf que no empieza
// como un PDF se rechaza.
type DocumentHandler struct {
	dir string
}

// tiposDeDocumento: extensión → cómo empieza el archivo.
//   - PDF: «%PDF».
//   - docx, xlsx, pptx y zip son ZIP por dentro: «PK\x03\x04».
//   - doc, xls y ppt antiguos son OLE: «\xD0\xCF\x11\xE0».
var tiposDeDocumento = map[string][]byte{
	".pdf":  []byte("%PDF"),
	".docx": []byte("PK\x03\x04"),
	".xlsx": []byte("PK\x03\x04"),
	".pptx": []byte("PK\x03\x04"),
	".zip":  []byte("PK\x03\x04"),
	".doc":  {0xD0, 0xCF, 0x11, 0xE0},
	".xls":  {0xD0, 0xCF, 0x11, 0xE0},
	".ppt":  {0xD0, 0xCF, 0x11, 0xE0},
}

var contentTypeDeDocumento = map[string]string{
	".pdf":  "application/pdf",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".zip":  "application/zip",
	".doc":  "application/msword",
	".xls":  "application/vnd.ms-excel",
	".ppt":  "application/vnd.ms-powerpoint",
}

// NewDocumentHandler guarda los documentos en <uploadsDir>/documentos.
func NewDocumentHandler(uploadsDir string) *DocumentHandler {
	if strings.TrimSpace(uploadsDir) == "" {
		uploadsDir = DefaultUploadDir
	}
	dir := filepath.Join(uploadsDir, "documentos")
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[DOCUMENTOS] no se pudo crear el directorio %q: %v", dir, err)
	}
	return &DocumentHandler{dir: dir}
}

// UploadDocument: POST /api/documents/upload, multipart con el campo «file».
// Responde {name, originalName, size}: lo que el panel guarda después con
// PUT /api/workshops/{id}/material.
func (h *DocumentHandler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	// Un poco de margen sobre los 25 MB para las cabeceras del multipart.
	r.Body = http.MaxBytesReader(w, r.Body, services.MaxTamanoDocumento+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Error(w, "el archivo tiene que pesar como mucho 25 MB", http.StatusRequestEntityTooLarge)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "falta el archivo", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if header.Size > services.MaxTamanoDocumento {
		http.Error(w, "el archivo tiene que pesar como mucho 25 MB", http.StatusRequestEntityTooLarge)
		return
	}

	original := filepath.Base(header.Filename)
	ext := strings.ToLower(filepath.Ext(original))
	firma, ok := tiposDeDocumento[ext]
	if !ok {
		http.Error(w, "solo se admiten PDF, Word, Excel, PowerPoint o ZIP", http.StatusBadRequest)
		return
	}

	inicio := make([]byte, len(firma))
	if _, err := io.ReadFull(file, inicio); err != nil || !bytes.Equal(inicio, firma) {
		http.Error(w, "el archivo no es lo que dice su extensión", http.StatusBadRequest)
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "no se pudo leer el archivo", http.StatusInternalServerError)
		return
	}

	nombre := uuid.NewString() + "_" + nombreSeguro(original)
	destino, err := os.Create(filepath.Join(h.dir, nombre))
	if err != nil {
		http.Error(w, "no se pudo guardar el archivo", http.StatusInternalServerError)
		return
	}
	escritos, err := io.Copy(destino, file)
	destino.Close()
	if err != nil {
		os.Remove(filepath.Join(h.dir, nombre))
		http.Error(w, "no se pudo guardar el archivo", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"name":         nombre,
		"originalName": original,
		"size":         escritos,
	})
}

// GetDocument: GET /api/documents/{fileName} y /api/documents/{fileName}/{descarga}.
// Público, como las imágenes: el material lo ven todos los usuarios (§5,
// decisión 1), y el nombre lleva un UUID que no se puede adivinar. Se sirve
// como descarga.
//
// {descarga} es el nombre original («Guía del workshop.pdf»), que la app y el
// panel ponen al final de la URL desde el 2026-11-09: los visores de PDF
// titulan con el último tramo de la dirección, y con un solo tramo mostraban
// «c7812220-ef19-…». Además el nombre guardado pierde las tildes
// («Gu-a-del-workshop.pdf»); el original las conserva y va en `filename*`.
// Sin {descarga} —apps ya instaladas— se usa el guardado sin el UUID, como
// antes.
func (h *DocumentHandler) GetDocument(w http.ResponseWriter, r *http.Request) {
	nombre := chi.URLParam(r, "fileName")
	if d, err := url.PathUnescape(nombre); err == nil {
		nombre = d
	}
	// Base después de decodificar: un «..%2F» tampoco sale de la carpeta.
	nombre = filepath.Base(nombre)
	ruta := filepath.Join(h.dir, nombre)
	if _, err := os.Stat(ruta); err != nil || nombre == "." || nombre == string(filepath.Separator) {
		http.Error(w, "no existe", http.StatusNotFound)
		return
	}

	ext := strings.ToLower(filepath.Ext(nombre))
	visible := nombreVisible(chi.URLParam(r, "descarga"), ext)
	if visible == "" {
		visible = nombre
		if len(visible) > 37 && visible[36] == '_' {
			visible = visible[37:]
		}
	}
	if ct, ok := contentTypeDeDocumento[ext]; ok {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Content-Disposition", contentDisposition(visible))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, ruta)
}

// nombreVisible limpia el {descarga} de la URL: sin carpetas, sin caracteres
// de control ni comillas, y con la extensión del archivo real, para que un
// enlace manipulado no haga pasar un .pdf por otra cosa. "" si no sirve.
func nombreVisible(descarga, ext string) string {
	if d, err := url.PathUnescape(descarga); err == nil {
		descarga = d
	}
	descarga = filepath.Base(strings.ReplaceAll(descarga, `\`, "/"))
	var b strings.Builder
	for _, c := range descarga {
		if c < 0x20 || c == 0x7f || c == '"' {
			continue
		}
		b.WriteRune(c)
	}
	limpio := strings.TrimSpace(b.String())
	if limpio == "" || limpio == "." || limpio == "/" {
		return ""
	}
	if strings.ToLower(filepath.Ext(limpio)) != ext {
		limpio += ext
	}
	return limpio
}

// contentDisposition pone el nombre dos veces: `filename` en ASCII para los
// clientes viejos y `filename*` en UTF-8 (RFC 6266) con las tildes.
func contentDisposition(visible string) string {
	return `attachment; filename="` + nombreSeguro(visible) + `"; filename*=UTF-8''` +
		codificarRFC5987(visible)
}

// codificarRFC5987 deja tal cual solo los attr-char del RFC 5987 y codifica
// en %XX el resto de bytes UTF-8. url.PathEscape no vale: deja «:», «=» o «@».
func codificarRFC5987(s string) string {
	const permitidos = "!#$&+-.^_`|~"
	var b strings.Builder
	for _, c := range []byte(s) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			strings.IndexByte(permitidos, c) >= 0 {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}
