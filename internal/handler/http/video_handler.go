package http

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"encoding/json"
	"net/http"
)

// VideoHandler sirve los videos de los canales de YouTube de Legacy Network y
// LSO para la sección de contenido de la app.
type VideoHandler struct {
	// videoService admite nil: sin clave de API configurada, main.go no lo
	// construye y el endpoint responde una lista vacía en vez de 500.
	videoService ports.VideoService
}

func NewVideoHandler(videoService ports.VideoService) *VideoHandler {
	return &VideoHandler{videoService: videoService}
}

// ListVideos responde la lista de videos, del más reciente al más antiguo.
//
// **Siempre 200 con una lista.** La app une esta fuente con las otras dos de la
// sección de contenido; devolver un error dejaría la pantalla entera en blanco
// por un problema de un tercero.
func (h *VideoHandler) ListVideos(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h.videoService == nil {
		json.NewEncoder(w).Encode([]domain.VideoDeCanal{})
		return
	}

	videos, err := h.videoService.ListarVideos(r.Context())
	if err != nil || videos == nil {
		json.NewEncoder(w).Encode([]domain.VideoDeCanal{})
		return
	}

	json.NewEncoder(w).Encode(videos)
}
