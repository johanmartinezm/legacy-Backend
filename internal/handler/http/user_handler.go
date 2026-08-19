package http

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type UserHandler struct {
	authService ports.AuthService
}

func NewUserHandler(authService ports.AuthService) *UserHandler {
	return &UserHandler{
		authService: authService,
	}
}

type RegisterRequest struct {
	Email                      string   `json:"email"`
	Password                   string   `json:"password"`
	FirstName                  string   `json:"first_name"`
	LastName                   string   `json:"last_name"`
	Phone                      string   `json:"phone"`
	Location                   string   `json:"location"`
	Bio                        string   `json:"bio"`
	CompanyName                string   `json:"company_name"`
	JobTitle                   string   `json:"job_title"`
	Country                    string   `json:"country"`
	IdentificationType         string   `json:"identification_type"`
	IdentificationNumber       string   `json:"identification_number"`
	CustomerStatus             string   `json:"customer_status"`
	Industry                   string   `json:"industry"`
	ProfileImageUrl            string   `json:"profile_image_url"`
	Generation                 string   `json:"generation"`
	IsPublicProfile            bool     `json:"is_public_profile"`
	AllowMessagesFromStrangers bool     `json:"allow_messages_from_strangers"`
	ShowActivity               bool     `json:"show_activity"`
	BirthDate                  string   `json:"birth_date"`
	Alias                      string   `json:"alias"`
	TermsAccepted              bool     `json:"terms_accepted"`
	DataSharingAccepted        bool     `json:"data_sharing_accepted"`
	Interests                  []string `json:"interests"`
	Role                       string   `json:"role"`
	GoogleID                   string   `json:"google_id,omitempty"`
	AppleID                    string   `json:"apple_id,omitempty"`
}

func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user := &domain.User{
		Email:                      req.Email,
		FirstName:                  req.FirstName,
		LastName:                   req.LastName,
		Phone:                      req.Phone,
		Location:                   req.Location,
		Bio:                        req.Bio,
		CompanyName:                req.CompanyName,
		JobTitle:                   req.JobTitle,
		Country:                    req.Country,
		IdentificationType:         req.IdentificationType,
		IdentificationNumber:       req.IdentificationNumber,
		CustomerStatus:             req.CustomerStatus,
		Industry:                   req.Industry,
		ProfileImageUrl:            req.ProfileImageUrl,
		Generation:                 req.Generation,
		IsPublicProfile:            req.IsPublicProfile,
		AllowMessagesFromStrangers: req.AllowMessagesFromStrangers,
		ShowActivity:               req.ShowActivity,
		Alias:                      req.Alias,
		TermsAccepted:              req.TermsAccepted,
		DataSharingAccepted:        req.DataSharingAccepted,
		Interests:                  req.Interests,
		Role:                       req.Role, // From frontend
	}

	if req.GoogleID != "" {
		user.GoogleID = &req.GoogleID
	}
	if req.AppleID != "" {
		user.AppleID = &req.AppleID
	}

	if user.Role == "" {
		user.Role = domain.RoleDefault
	}
	// Sin esto el rol baja tal cual al INSERT y es Postgres quien lo rechaza,
	// con un 500 que le enseñaba el SQLSTATE al usuario (ver más abajo).
	if !domain.IsValidRole(user.Role) {
		h.respondWithError(w, http.StatusBadRequest, "Perfil de usuario no válido")
		return
	}

	if req.BirthDate != "" {
		// Expecting DD/MM/YYYY from Flutter
		t, err := time.Parse("02/01/2006", req.BirthDate)
		if err != nil {
			h.respondWithError(w, http.StatusBadRequest, "Formato de fecha de nacimiento inválido (esperado DD/MM/YYYY)")
			return
		}
		user.BirthDate = &t
	}

	if err := h.authService.Register(r.Context(), user, req.Password); err != nil {
		if err.Error() == "user already exists" {
			h.respondWithError(w, http.StatusConflict, "El usuario ya existe")
			return
		}
		// La contraseña corta es culpa de quien pide, no del servidor: sin esta
		// rama caería en el 500 genérico de abajo y el formulario diría
		// "Inténtalo de nuevo" sin explicar qué corregir.
		if errors.Is(err, domain.ErrContrasenaCorta) {
			h.respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		// El detalle va al log, no a la pantalla: devolver err.Error() es lo que
		// le mostró al usuario "ERROR: invalid input value for enum
		// core.user_role: junta (SQLSTATE 22P02)" el 2026-08-18.
		log.Printf("Register: %v", err)
		h.respondWithError(w, http.StatusInternalServerError, "No se pudo crear la cuenta. Inténtalo de nuevo.")
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "User registered successfully", "id": user.ID})
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	token, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		// Hasta el 2026-08-18 todo error caía en "Credenciales inválidas". El
		// servicio ya distinguía los casos y el handler los aplanaba en la
		// última línea: quien no había visto el correo de verificación recibía
		// el mismo texto que si se hubiera equivocado de contraseña, y no tenía
		// forma de saber qué le faltaba.
		//
		// Estos dos mensajes solo salen tras acertar la contraseña, así que no
		// revelan a un tercero si una cuenta existe.
		switch {
		case errors.Is(err, domain.ErrCorreoSinVerificar):
			h.respondWithErrorCode(w, http.StatusForbidden,
				"Tu cuenta todavía no está verificada. Revisa tu correo —incluida la carpeta de spam— y confirma tu dirección.",
				"email_not_verified")
			return
		case errors.Is(err, domain.ErrCuentaSocial):
			h.respondWithErrorCode(w, http.StatusForbidden,
				"Esta cuenta se creó con Google o Apple. Entra con ese mismo botón.",
				"social_account")
			return
		}
		h.respondWithError(w, http.StatusUnauthorized, "Credenciales inválidas")
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

type SocialLoginRequest struct {
	Provider string `json:"provider"`
	IDToken  string `json:"id_token"`
}

func (h *UserHandler) SocialLogin(w http.ResponseWriter, r *http.Request) {
	var req SocialLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	token, user, err := h.authService.SocialLogin(r.Context(), req.Provider, req.IDToken)
	if err != nil {
		if err.Error() == "user not registered" {
			// Return 404 to let frontend know it must register, but include social details
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "user not registered",
				"email": user.Email,
				"name":  user.FirstName,
			})
			return
		}
		// El motivo real (audiencia incorrecta, token expirado, firma invalida...) solo
		// existe aqui: al cliente se le responde un 401 generico a proposito.
		log.Printf("[SocialLogin] rechazado (provider=%s): %v", req.Provider, err)
		h.respondWithError(w, http.StatusUnauthorized, "Credenciales inválidas de red social")
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.authService.ListUsers(r.Context())
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		h.respondWithError(w, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	user, err := h.authService.GetProfile(r.Context(), userID)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		h.respondWithError(w, http.StatusBadRequest, "Missing user ID")
		return
	}
	h.performUpdate(w, r, id)
}

func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		h.respondWithError(w, http.StatusUnauthorized, "User ID not found in context")
		return
	}
	h.performUpdate(w, r, userID)
}

func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		h.respondWithError(w, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.authService.ChangePassword(r.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		h.respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Password updated successfully"})
}

func (h *UserHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.authService.RequestPasswordReset(r.Context(), req.Email); err != nil {
		// Even if error is internal, we usually don't reveal too much
		log.Printf("Error requesting password reset: %v", err)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "If the email is registered, a reset link will be sent."})
}

func (h *UserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.authService.ResetPassword(r.Context(), req.Email, req.Token, req.NewPassword); err != nil {
		h.respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Password has been reset successfully"})
}

func (h *UserHandler) performUpdate(w http.ResponseWriter, r *http.Request, id string) {
	user, err := h.authService.GetProfile(r.Context(), id)
	if err != nil {
		h.respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	if err := json.NewDecoder(r.Body).Decode(user); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	user.ID = id
	// El decode de arriba vuelca el JSON sobre el usuario ya cargado, así que el
	// cliente también puede cambiar el rol por aquí y llegar al mismo 22P02.
	if !domain.IsValidRole(user.Role) {
		h.respondWithError(w, http.StatusBadRequest, "Perfil de usuario no válido")
		return
	}
	// Solo el id. El `%+v` que había aquí volcaba el struct entero en cada
	// edición de perfil, y eso dejaba en el log el hash bcrypt, el correo
	// cifrado y el email_blind_index —el HMAC del que depende el inicio de
	// sesión por correo—.
	log.Printf("Updating user %s", id)

	if err := h.authService.UpdateUser(r.Context(), user); err != nil {
		if err.Error() == "alias_in_use" {
			h.respondWithError(w, http.StatusConflict, "El alias ya está en uso. Por favor elige otro.")
			return
		}
		if err.Error() == "invalid_alias_format" {
			h.respondWithError(w, http.StatusBadRequest, "El alias solo puede contener letras y números, sin espacios ni caracteres especiales.")
			return
		}
		h.respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		h.respondWithError(w, http.StatusBadRequest, "Missing user ID")
		return
	}

	if err := h.authService.DeleteUser(r.Context(), id); err != nil {
		h.respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"message": message})
}

// respondWithErrorCode añade un `code` estable junto al mensaje.
//
// El mensaje es para leerlo y puede cambiar de redacción; el código es para que
// la app decida qué hacer —ofrecer reenviar la verificación, o llevar al botón
// de Google— sin comparar textos en español, que se rompen al primer retoque.
func (h *UserHandler) respondWithErrorCode(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"message": message, "code": code})
}

func (h *UserHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.authService.VerifyEmail(r.Context(), req.Token); err != nil {
		h.respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Email verificado exitosamente"})
}

func (h *UserHandler) ResendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.authService.ResendVerificationEmail(r.Context(), req.Email); err != nil {
		// Aquí ya solo llegan fallos internos: el servicio devuelve nil cuando la
		// cuenta no existe o ya está verificada, precisamente para que la
		// respuesta no delate cuáles están registradas.
		log.Printf("ResendVerificationEmail: %v", err)
		h.respondWithError(w, http.StatusInternalServerError, "No se pudo reenviar el correo. Inténtalo de nuevo.")
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Correo de verificación reenviado"})
}

// DeleteMe atiende DELETE /api/me: la persona elimina su propia cuenta.
//
// El usuario sale SIEMPRE del token, nunca del cuerpo ni de la URL. Si se
// aceptara de fuera, cualquiera con sesión podría borrar la cuenta de otro, que
// es el mismo error que ya se corrigió en el registro a eventos y en el inicio
// de pagos.
//
// La cuenta se anonimiza, no se borra: ver AuthService.DeleteMyAccount.
func (h *UserHandler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		h.respondWithError(w, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	if err := h.authService.DeleteMyAccount(r.Context(), userID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// La cuenta no existe o ya estaba eliminada. Se responde 404 y no
			// 200 para no dar por hecho un borrado que no ha ocurrido.
			h.respondWithError(w, http.StatusNotFound, "La cuenta no existe o ya fue eliminada")
			return
		}
		h.respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 204: no hay nada que devolver, y el cliente debe cerrar sesión al recibirlo.
	w.WriteHeader(http.StatusNoContent)
}
