// Package handler maps HTTP requests to services and service errors to status codes.
package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/ariefusry/diecast-auction/internal/middleware"
	"github.com/ariefusry/diecast-auction/internal/service"
)

type handler struct {
	auth    *service.Auth
	wallets *service.Wallets
}

// New returns the API router.
func New(auth *service.Auth, wallets *service.Wallets) http.Handler {
	h := &handler{auth: auth, wallets: wallets}
	authed := middleware.RequireAuth(auth.ParseToken)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /auth/register", h.register)
	mux.HandleFunc("POST /auth/login", h.login)
	mux.Handle("GET /wallet", authed(http.HandlerFunc(h.getWallet)))
	mux.Handle("POST /wallet/deposit", authed(http.HandlerFunc(h.deposit)))
	return mux
}

func (h *handler) register(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Username, Password string }
	if !decode(w, r, &req) {
		return
	}
	user, err := h.auth.Register(r.Context(), req.Email, req.Username, req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password string }
	if !decode(w, r, &req) {
		return
	}
	token, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *handler) getWallet(w http.ResponseWriter, r *http.Request) {
	wallet, err := h.wallets.Get(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wallet)
}

func (h *handler) deposit(w http.ResponseWriter, r *http.Request) {
	var req struct{ Amount int64 }
	if !decode(w, r, &req) {
		return
	}
	wallet, err := h.wallets.Deposit(r.Context(), middleware.UserID(r.Context()), req.Amount)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wallet)
}

// decode reads a JSON body capped at 1 MiB; on failure it writes 400 and returns false.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError maps service sentinels to status codes. Unknown errors are
// logged and hidden behind a generic 500 so internals never leak to clients.
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, service.ErrInvalidCredentials):
		status = http.StatusUnauthorized
	case errors.Is(err, service.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, service.ErrConflict):
		status = http.StatusConflict
	}
	msg := err.Error()
	if status == http.StatusInternalServerError {
		log.Printf("internal error: %v", err)
		msg = "internal server error"
	}
	writeJSON(w, status, map[string]string{"error": msg})
}
