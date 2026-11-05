package services

import (
	"applegacy/backend/internal/core/domain"
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
)

// Material de cada workshop (parte A del plan de workshops, decidido el
// 2026-10-02 y ajustado el 2026-10-28): una imagen, un archivo descargable y un
// video de YouTube o Vimeo. Lo carga el panel en cualquier momento; la app lo
// muestra cuando el workshop termina.

// MaxTamanoDocumento es el tope del archivo del material: 25 MB (§5, decisión 6).
const MaxTamanoDocumento = 25 << 20

// ErrorMaterial es un material que no se puede guardar. Motivo va tal cual al
// panel, en español.
type ErrorMaterial struct {
	Motivo string
}

func (e *ErrorMaterial) Error() string { return e.Motivo }

// nombreDeDocumento es como guarda los archivos DocumentHandler: un UUID, «_»
// y el nombre original reducido a caracteres seguros. Cualquier otra cosa no
// es un archivo subido por el panel.
var nombreDeDocumento = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}_[A-Za-z0-9._-]+$`)

// hostsDeVideo: YouTube se reproduce dentro de la app; Vimeo se abre fuera
// (§5, decisión 3). Un enlace de cualquier otro sitio se rechaza.
var hostsDeVideo = map[string]string{
	"youtube.com":      "youtube",
	"www.youtube.com":  "youtube",
	"m.youtube.com":    "youtube",
	"youtu.be":         "youtube",
	"vimeo.com":        "vimeo",
	"www.vimeo.com":    "vimeo",
	"player.vimeo.com": "vimeo",
}

var soloDigitos = regexp.MustCompile(`^[0-9]+$`)

// validarEnlaceDeVideo acepta solo enlaces de YouTube o de Vimeo que apunten a
// un video concreto. No puede saber si un video de Vimeo es «no listado»: el
// panel recuerda que en ese caso el enlace tiene que llevar su código.
func validarEnlaceDeVideo(enlace string) error {
	u, err := url.Parse(enlace)
	if err != nil || u.Host == "" {
		return &ErrorMaterial{Motivo: "el enlace del video no es una dirección válida"}
	}
	if u.Scheme != "https" {
		return &ErrorMaterial{Motivo: "el enlace del video tiene que empezar por https://"}
	}
	sitio, ok := hostsDeVideo[strings.ToLower(u.Host)]
	if !ok {
		return &ErrorMaterial{Motivo: "el video tiene que ser de YouTube o de Vimeo"}
	}

	partes := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch sitio {
	case "youtube":
		switch {
		case strings.EqualFold(u.Host, "youtu.be") && partes[0] != "":
			return nil
		case partes[0] == "watch" && u.Query().Get("v") != "":
			return nil
		case len(partes) >= 2 && (partes[0] == "shorts" || partes[0] == "live" || partes[0] == "embed"):
			return nil
		}
		return &ErrorMaterial{Motivo: "el enlace de YouTube no apunta a un video"}
	default: // vimeo
		if strings.EqualFold(u.Host, "player.vimeo.com") {
			if len(partes) >= 2 && partes[0] == "video" && soloDigitos.MatchString(partes[1]) {
				return nil
			}
		} else if soloDigitos.MatchString(partes[0]) {
			return nil
		}
		return &ErrorMaterial{Motivo: "el enlace de Vimeo no apunta a un video"}
	}
}

// normalizarMaterial quita espacios y valida cada campo.
func normalizarMaterial(m domain.MaterialDeWorkshop) (domain.MaterialDeWorkshop, error) {
	m.ImageUrl = strings.TrimSpace(m.ImageUrl)
	m.FileName = strings.TrimSpace(m.FileName)
	m.FileOriginalName = strings.TrimSpace(m.FileOriginalName)
	m.VideoUrl = strings.TrimSpace(m.VideoUrl)

	if m.VideoUrl != "" {
		if err := validarEnlaceDeVideo(m.VideoUrl); err != nil {
			return m, err
		}
	}

	if m.FileName == "" {
		// Sin archivo no se guardan sus datos sueltos.
		m.FileOriginalName, m.FileSize = "", 0
		return m, nil
	}
	if !nombreDeDocumento.MatchString(m.FileName) {
		return m, &ErrorMaterial{Motivo: "el archivo no es uno subido desde el panel"}
	}
	if m.FileSize <= 0 || m.FileSize > MaxTamanoDocumento {
		return m, &ErrorMaterial{Motivo: "el archivo tiene que pesar como mucho 25 MB"}
	}
	if m.FileOriginalName == "" {
		// Se muestra el nombre guardado sin el UUID de delante.
		m.FileOriginalName = m.FileName[37:]
	}
	return m, nil
}

// ActualizarMaterialWorkshop valida y guarda el material de un workshop, y lo
// devuelve como queda. No toca el resto del workshop.
func (s *EventService) ActualizarMaterialWorkshop(ctx context.Context, id string, m domain.MaterialDeWorkshop) (*domain.Workshop, error) {
	m, err := normalizarMaterial(m)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateWorkshopMaterial(ctx, id, m); err != nil {
		return nil, err
	}
	w, err := s.repo.GetWorkshopByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.ErrNotFound
	}
	return w, err
}
