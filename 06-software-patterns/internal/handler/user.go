package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"learning-go/internal/service"
)

// UserIDKey is the exported context key for user ID
const UserIDKey string = "userID"

type UserHandler struct {
	svc *service.UserService
}

func New(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.svc.CreateUser(req.Name, req.Email)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrAlreadyExists) {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	idVal := r.Context().Value(UserIDKey)
	id, ok := idVal.(string)
	if !ok || id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	user, err := h.svc.GetUser(id)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	idVal := r.Context().Value(UserIDKey)
	id, ok := idVal.(string)
	if !ok || id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.svc.UpdateUser(id, req.Name)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idVal := r.Context().Value(UserIDKey)
	id, ok := idVal.(string)
	if !ok || id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	if err := h.svc.DeleteUser(id); err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users := h.svc.ListUsers()
	json.NewEncoder(w).Encode(users)
}
