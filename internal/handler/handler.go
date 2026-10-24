// Package handler maps HTTP requests to services and service errors to status codes.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/ariefusry/diecast-auction/internal/middleware"
	"github.com/ariefusry/diecast-auction/internal/service"
)

// ParseFunc turns a natural language query into a search filter (the LLM).
type ParseFunc func(ctx context.Context, query string) (service.Filter, error)

type handler struct {
	auth     *service.Auth
	wallets  *service.Wallets
	auctions *service.Auctions
	parse    ParseFunc
}

// New returns the API router. parse may be nil, then AI search is keyword search.
func New(auth *service.Auth, wallets *service.Wallets, auctions *service.Auctions, parse ParseFunc) http.Handler {
	h := &handler{auth: auth, wallets: wallets, auctions: auctions, parse: parse}
	authed := middleware.RequireAuth(auth.ParseToken)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /auth/register", h.register)
	mux.HandleFunc("POST /auth/login", h.login)
	mux.Handle("GET /wallet", authed(http.HandlerFunc(h.getWallet)))
	mux.Handle("POST /wallet/deposit", authed(http.HandlerFunc(h.deposit)))
	mux.Handle("POST /auctions", authed(http.HandlerFunc(h.createAuction)))
	mux.HandleFunc("GET /auctions", h.listAuctions)
	mux.HandleFunc("POST /auctions/search/ai", h.searchAI)
	mux.HandleFunc("GET /auctions/{id}", h.getAuction)
	mux.Handle("POST /auctions/{id}/publish", authed(http.HandlerFunc(h.auctionAction(h.auctions.Publish))))
	mux.Handle("POST /auctions/{id}/cancel", authed(http.HandlerFunc(h.auctionAction(h.auctions.Cancel))))
	mux.Handle("POST /auctions/{id}/buy-now", authed(http.HandlerFunc(h.auctionAction(h.auctions.BuyNow))))
	mux.Handle("POST /auctions/{id}/bids", authed(http.HandlerFunc(h.bid)))
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

func (h *handler) createAuction(w http.ResponseWriter, r *http.Request) {
	var req service.NewAuction
	if !decode(w, r, &req) {
		return
	}
	a, err := h.auctions.Create(r.Context(), middleware.UserID(r.Context()), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (h *handler) getAuction(w http.ResponseWriter, r *http.Request) {
	a, err := h.auctions.Get(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// listAuctions is the catalog: GET /auctions?q=&brand=&condition=&color=&tags=a,b&min_price=&max_price=&sort=
func (h *handler) listAuctions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	price := func(k string) int64 { n, _ := strconv.ParseInt(q.Get(k), 10, 64); return n }
	f := service.Filter{
		Text: q.Get("q"), Brand: q.Get("brand"), Series: q.Get("series"), Color: q.Get("color"),
		Condition: q.Get("condition"), MinPrice: price("min_price"), MaxPrice: price("max_price"), Sort: q.Get("sort"),
	}
	if tags := q.Get("tags"); tags != "" {
		f.Tags = strings.Split(tags, ",")
	}
	h.search(w, r, f, false)
}

// searchAI parses a free-text query with the LLM. If there is no LLM or it
// fails, the whole query becomes a keyword search so the endpoint stays up.
func (h *handler) searchAI(w http.ResponseWriter, r *http.Request) {
	var req struct{ Query string }
	if !decode(w, r, &req) {
		return
	}
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" || len(req.Query) > 200 {
		writeError(w, fmt.Errorf("%w: query must be 1-200 bytes", service.ErrInvalidInput))
		return
	}
	if h.parse != nil {
		f, err := h.parse(r.Context(), req.Query)
		if err == nil {
			h.search(w, r, f, true)
			return
		}
		log.Printf("ai search fell back to keywords: %v", err)
	}
	h.search(w, r, service.Filter{Text: req.Query}, false)
}

func (h *handler) search(w http.ResponseWriter, r *http.Request, f service.Filter, ai bool) {
	res, err := h.auctions.Search(r.Context(), f)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		AI bool `json:"ai"` // whether the LLM produced the filter
		service.SearchResult
	}{ai, res})
}

// auctionAction adapts a body-less (user, auction) service call such as publish or cancel.
func (h *handler) auctionAction(fn func(ctx context.Context, userID, id int64) (service.Auction, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, err := fn(r.Context(), middleware.UserID(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, a)
	}
}

func (h *handler) bid(w http.ResponseWriter, r *http.Request) {
	var req struct{ Amount int64 }
	if !decode(w, r, &req) {
		return
	}
	a, err := h.auctions.Bid(r.Context(), middleware.UserID(r.Context()), pathID(r), req.Amount)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// pathID parses {id}; a malformed id becomes 0, which matches no row and yields 404.
func pathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
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
	case errors.Is(err, service.ErrForbidden):
		status = http.StatusForbidden
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
