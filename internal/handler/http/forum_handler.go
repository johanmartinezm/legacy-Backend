package http

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// No ImageStorageService needed here anymore since we receive image_url directly


type ForumHandler struct {
	forumService ports.ForumService
	imageStorage interface{} // Optional: inject the actual storage service here if needed, or we might use the image_handler's service
}

// Since imageStorage is not defined cleanly here without viewing image_handler, let's keep it simple.
// We will skip actual image upload in this file and assume imageURL comes from somewhere, OR
// let's define the methods according to plan.
// The plan states we accept multipart/form-data.

func NewForumHandler(forumService ports.ForumService, imageStorage interface{}) *ForumHandler {
	return &ForumHandler{
		forumService: forumService,
		imageStorage: imageStorage,
	}
}

// =======================
// PUBLIC USER ENDPOINTS
// =======================

func (h *ForumHandler) ListPublicForums(w http.ResponseWriter, r *http.Request) {
	forums, err := h.forumService.ListPublicForums(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(forums)
}

func (h *ForumHandler) CreateUserForum(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var forum domain.Forum
	if err := json.NewDecoder(r.Body).Decode(&forum); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.forumService.CreateUserForum(r.Context(), userID, &forum); err != nil {
		if err.Error() == "alias_required" {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		// Added logging to track down 500 error
		importLog := "log"
		_ = importLog // Just a mental note to import log, wait I'll use standard print
		println("Error in ProposeForum:", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(forum)
}

func (h *ForumHandler) ListPosts(w http.ResponseWriter, r *http.Request) {
	forumID := chi.URLParam(r, "forumID")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	
	limit, _ := strconv.Atoi(limitStr)
	if limit == 0 {
		limit = 20
	}
	offset, _ := strconv.Atoi(offsetStr)

	posts, err := h.forumService.GetForumThread(r.Context(), forumID, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(posts)
}

func (h *ForumHandler) PublishPost(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	forumID := chi.URLParam(r, "forumID")

	var payload struct {
		Content  string  `json:"content"`
		ParentID *string `json:"parent_id"`
		ImageURL string  `json:"image_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if payload.Content == "" && payload.ImageURL == "" {
		http.Error(w, "content or image is required", http.StatusBadRequest)
		return
	}

	post, err := h.forumService.PublishPost(r.Context(), userID, forumID, payload.Content, payload.ImageURL, payload.ParentID)
	if err != nil {
		if err.Error() == "alias_required" {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(post)
}

func (h *ForumHandler) ReportPost(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	postID := chi.URLParam(r, "postID")

	var payload struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.forumService.ReportPost(r.Context(), userID, postID, payload.Reason); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// =======================
// ADMIN ENDPOINTS
// =======================

func (h *ForumHandler) AdminListForums(w http.ResponseWriter, r *http.Request) {
	forums, err := h.forumService.ListAllForums(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(forums)
}


func (h *ForumHandler) AdminCreateForum(w http.ResponseWriter, r *http.Request) {
	var forum domain.Forum
	if err := json.NewDecoder(r.Body).Decode(&forum); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.forumService.CreateForum(r.Context(), &forum); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(forum)
}

func (h *ForumHandler) AdminUpdateForum(w http.ResponseWriter, r *http.Request) {
	forumID := chi.URLParam(r, "forumID")
	var forum domain.Forum
	if err := json.NewDecoder(r.Body).Decode(&forum); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	forum.ID = forumID

	if err := h.forumService.UpdateForum(r.Context(), &forum); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ForumHandler) AdminLockForum(w http.ResponseWriter, r *http.Request) {
	forumID := chi.URLParam(r, "forumID")
	if err := h.forumService.LockForum(r.Context(), forumID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ForumHandler) AdminUnlockForum(w http.ResponseWriter, r *http.Request) {
	forumID := chi.URLParam(r, "forumID")
	if err := h.forumService.UnlockForum(r.Context(), forumID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ForumHandler) AdminDeleteForum(w http.ResponseWriter, r *http.Request) {
	forumID := chi.URLParam(r, "forumID")
	if err := h.forumService.DeleteForum(r.Context(), forumID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ForumHandler) AdminDeletePost(w http.ResponseWriter, r *http.Request) {
	postID := chi.URLParam(r, "postID")
	if err := h.forumService.DeletePost(r.Context(), postID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ForumHandler) AdminListFlaggedPosts(w http.ResponseWriter, r *http.Request) {
	posts, err := h.forumService.ListFlaggedPosts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(posts)
}

func (h *ForumHandler) AdminGetForumTree(w http.ResponseWriter, r *http.Request) {
	forumID := chi.URLParam(r, "forumID")
	posts, err := h.forumService.GetForumTree(r.Context(), forumID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(posts)
}
