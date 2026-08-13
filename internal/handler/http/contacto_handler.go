package http

import (
	"applegacy/backend/internal/core/ports"
	"applegacy/backend/internal/core/services"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type ContactoHandler struct {
	contactoService ports.ContactoService
	authService     ports.AuthService
}

func NewContactoHandler(contactoService ports.ContactoService, authService ports.AuthService) *ContactoHandler {
	return &ContactoHandler{
		contactoService: contactoService,
		authService:     authService,
	}
}

// Enviar atiende POST /api/contacto.
//
// El remitente NO viaja en el cuerpo: se saca del perfil de quien está
// autenticado. Aceptarlo del cliente permitiría escribir al buzón de soporte
// haciéndose pasar por cualquiera.
func (h *ContactoHandler) Enviar(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Asunto  string `json:"asunto"`
		Mensaje string `json:"mensaje"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Lo barato primero: un mensaje en blanco se rechaza sin gastar una consulta
	// a la base. El servicio lo vuelve a comprobar, que es donde manda.
	if strings.TrimSpace(req.Mensaje) == "" {
		http.Error(w, "el mensaje no puede estar vacío", http.StatusBadRequest)
		return
	}

	remitente, err := h.authService.GetProfile(r.Context(), userID)
	if err != nil {
		http.Error(w, "no se pudo leer tu perfil, vuelve a iniciar sesión", http.StatusInternalServerError)
		return
	}

	nombre := remitente.FirstName + " " + remitente.LastName
	err = h.contactoService.EnviarMensaje(r.Context(), userID, req.Asunto, nombre, remitente.Email, req.Mensaje)
	if err != nil {
		// Haber escrito demasiado seguido no es un error del mensaje: merece su
		// propio código para que la app pueda decir "espera un momento".
		if errors.Is(err, services.ErrDemasiadosMensajes) {
			http.Error(w, err.Error(), http.StatusTooManyRequests)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Mensaje enviado con éxito"})
}

// Listar atiende GET /api/admin/contacto?estado=nuevo (bajo AdminOnly).
func (h *ContactoHandler) Listar(w http.ResponseWriter, r *http.Request) {
	mensajes, err := h.contactoService.Listar(r.Context(), r.URL.Query().Get("estado"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, mensajes)
}

// CambiarEstado atiende PATCH /api/admin/contacto/{id} (bajo AdminOnly).
func (h *ContactoHandler) CambiarEstado(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Estado string `json:"estado"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"message": "Cuerpo de la petición no válido"})
		return
	}

	if err := h.contactoService.CambiarEstado(r.Context(), chi.URLParam(r, "id"), req.Estado); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "Estado actualizado"})
}
