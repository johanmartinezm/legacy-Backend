package http

import (
	"applegacy/backend/internal/core/ports"
	"encoding/json"
	"net/http"
	"strings"
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
	if err := h.contactoService.EnviarMensaje(r.Context(), req.Asunto, nombre, remitente.Email, req.Mensaje); err != nil {
		// Lo que rechaza el servicio son datos del cliente —mensaje vacío o
		// demasiado largo—, así que es un 400 y no un 500. La única excepción
		// es la falta de configuración, que el arranque ya deja en el log.
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Mensaje enviado con éxito"})
}
