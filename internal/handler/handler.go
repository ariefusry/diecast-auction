// Package handler maps HTTP requests to services and service errors to status codes.
//
// Swagger docs are generated from the annotations below into internal/apidocs:
//
//	swag init -g handler.go -d internal/handler,internal/service -o internal/apidocs --ot go,json
//
// @title                      Diecast Auction API
// @version                    1.0
// @description                Escrow auctions for 1:64 diecast (Hot Wheels, Mini GT, Inno64, Matchbox, Tomica, Tarmac Works, M2 Machines). Money is whole rupiah.
// @BasePath                   /
// @securityDefinitions.apikey BearerAuth
// @in                         header
// @name                       Authorization
// @description                "Bearer <token>" from POST /auth/login
package handler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	httpSwagger "github.com/swaggo/http-swagger/v2"
	"golang.org/x/time/rate"

	_ "github.com/ariefusry/diecast-auction/internal/apidocs" // registers the generated spec
	"github.com/ariefusry/diecast-auction/internal/middleware"
	"github.com/ariefusry/diecast-auction/internal/service"
)

// Per-IP limits against bots: auth refills one request per 6s (10/min), bids 2/s,
// AI search one per 3s (20/min) since each may cost an LLM call.
const (
	authBurst = 10
	bidBurst  = 5
	aiBurst   = 5
)

// maxUpload caps one item photo at 5 MiB.
const maxUpload = 5 << 20

// imageTypes maps the sniffed content type of an accepted upload to its file extension.
var imageTypes = map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}

// ParseFunc turns a natural language query into a search filter (the LLM).
type ParseFunc func(ctx context.Context, query string) (service.Filter, error)

type handler struct {
	auth     *service.Auth
	wallets  *service.Wallets
	auctions *service.Auctions
	parse    ParseFunc
	uploads  string
}

type errorResponse struct {
	Error string `json:"error" example:"conflict: bid must be at least 110000"`
}

type registerRequest struct {
	Email    string `json:"email" example:"budi@example.com"`
	Username string `json:"username" example:"budi_64"`
	Password string `json:"password" example:"correct-horse"`
}

type loginRequest struct {
	Email    string `json:"email" example:"demo_bidder1@demo.local"`
	Password string `json:"password" example:"demo-password"`
}

type tokenResponse struct {
	Token string `json:"token"`
}

type amountRequest struct {
	Amount int64 `json:"amount" example:"110000"`
}

type searchRequest struct {
	Query string `json:"query" example:"jdm merah mint di bawah 100k"`
}

type uploadResponse struct {
	URL string `json:"url" example:"/uploads/3f9c2a1b7e4d8c60.jpg"`
}

type searchResponse struct {
	AI bool `json:"ai"` // whether the LLM produced the filter
	service.SearchResult
}

// New returns the API router. parse may be nil, then AI search is keyword
// search. Uploaded photos are stored in and served from uploadDir.
func New(auth *service.Auth, wallets *service.Wallets, auctions *service.Auctions, parse ParseFunc, uploadDir string) http.Handler {
	h := &handler{auth: auth, wallets: wallets, auctions: auctions, parse: parse, uploads: uploadDir}
	authed := func(f http.HandlerFunc) http.Handler { return middleware.RequireAuth(auth.ParseToken)(f) }
	authLimit := middleware.RateLimit(rate.Every(6*time.Second), authBurst)
	bidLimit := middleware.RateLimit(2, bidBurst)
	aiLimit := middleware.RateLimit(rate.Every(3*time.Second), aiBurst)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, fmt.Errorf("%w: no route for %s %s", service.ErrNotFound, r.Method, r.URL.Path))
	})
	mux.Handle("GET /swagger/", httpSwagger.WrapHandler)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("POST /auth/register", authLimit(http.HandlerFunc(h.register)))
	mux.Handle("POST /auth/login", authLimit(http.HandlerFunc(h.login)))
	mux.Handle("GET /wallet", authed(h.getWallet))
	mux.Handle("POST /wallet/deposit", authed(h.deposit))
	mux.Handle("POST /auctions", authed(h.createAuction))
	mux.HandleFunc("GET /auctions", h.listAuctions)
	mux.Handle("POST /auctions/search/ai", aiLimit(http.HandlerFunc(h.searchAI)))
	mux.HandleFunc("GET /auctions/{id}", h.getAuction)
	mux.HandleFunc("GET /auctions/{id}/bids", h.listBids)
	mux.Handle("POST /auctions/{id}/publish", authed(h.publish))
	mux.Handle("POST /auctions/{id}/cancel", authed(h.cancel))
	mux.Handle("POST /auctions/{id}/buy-now", authed(h.buyNow))
	mux.Handle("POST /auctions/{id}/bids", bidLimit(authed(h.bid)))
	mux.Handle("POST /uploads", bidLimit(authed(h.upload)))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", files(uploadDir)))
	return mux
}

// register godoc
// @Summary  Create an account with an empty wallet
// @Tags     auth
// @Accept   json
// @Produce  json
// @Param    body body     registerRequest true "New account"
// @Success  201  {object} service.User
// @Failure  400  {object} errorResponse
// @Failure  409  {object} errorResponse "email or username taken"
// @Failure  429  {object} errorResponse
// @Router   /auth/register [post]
func (h *handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
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

// login godoc
// @Summary  Get a JWT valid for 24 hours
// @Tags     auth
// @Accept   json
// @Produce  json
// @Param    body body     loginRequest true "Credentials"
// @Success  200  {object} tokenResponse
// @Failure  401  {object} errorResponse
// @Failure  429  {object} errorResponse
// @Router   /auth/login [post]
func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decode(w, r, &req) {
		return
	}
	token, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokenResponse{Token: token})
}

// getWallet godoc
// @Summary  Available and escrow-locked balance
// @Tags     wallet
// @Produce  json
// @Security BearerAuth
// @Success  200 {object} service.Wallet
// @Failure  401 {object} errorResponse
// @Router   /wallet [get]
func (h *handler) getWallet(w http.ResponseWriter, r *http.Request) {
	wallet, err := h.wallets.Get(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wallet)
}

// deposit godoc
// @Summary  Mock top-up, at most Rp10.000.000 per call
// @Tags     wallet
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body     amountRequest true "Amount in rupiah"
// @Success  200  {object} service.Wallet
// @Failure  400  {object} errorResponse
// @Failure  401  {object} errorResponse
// @Router   /wallet/deposit [post]
func (h *handler) deposit(w http.ResponseWriter, r *http.Request) {
	var req amountRequest
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

// createAuction godoc
// @Summary  List an item as a draft auction
// @Tags     auctions
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body body     service.NewAuction true "Item and auction terms"
// @Success  201  {object} service.Auction
// @Failure  400  {object} errorResponse
// @Failure  401  {object} errorResponse
// @Router   /auctions [post]
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

// getAuction godoc
// @Summary  One auction, including its current end_time (anti-sniping may move it)
// @Tags     auctions
// @Produce  json
// @Param    id  path     int true "Auction ID"
// @Success  200 {object} service.Auction
// @Failure  404 {object} errorResponse
// @Router   /auctions/{id} [get]
func (h *handler) getAuction(w http.ResponseWriter, r *http.Request) {
	a, err := h.auctions.Get(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// listBids godoc
// @Summary  Latest bids on an auction, newest first, bidder names masked
// @Tags     auctions
// @Produce  json
// @Param    id  path     int true "Auction ID"
// @Success  200 {array}  service.BidEntry
// @Failure  404 {object} errorResponse
// @Router   /auctions/{id}/bids [get]
func (h *handler) listBids(w http.ResponseWriter, r *http.Request) {
	bids, err := h.auctions.Bids(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bids)
}

// listAuctions godoc
// @Summary  Catalog of active auctions
// @Tags     auctions
// @Produce  json
// @Param    q         query    string false "Keywords"
// @Param    brand     query    string false "Hot Wheels, Mini GT, Inno64, Matchbox, Tomica, Tarmac Works or M2 Machines"
// @Param    condition query    string false "mint_on_card, sealed_box, acrylic_case, loose_mint, damaged_packaging"
// @Param    color     query    string false "Color substring"
// @Param    series    query    string false "Series substring"
// @Param    tags      query    string false "Comma separated, all must match (e.g. jdm,nissan)"
// @Param    min_price query    int    false "Current price at least (rupiah)"
// @Param    max_price query    int    false "Current price at most (rupiah)"
// @Param    sort      query    string false "ending_soon (default), newest, price_asc, price_desc"
// @Success  200       {object} searchResponse
// @Router   /auctions [get]
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

// searchAI godoc
// @Summary      Natural language search
// @Description  An LLM turns the query into a filter. Without an LLM, or when it fails, the query is used as keywords (ai=false).
// @Description  When nothing matches, filters are relaxed step by step (relaxed=true).
// @Tags         auctions
// @Accept       json
// @Produce      json
// @Param        body body     searchRequest true "Query, 1-200 bytes"
// @Success      200  {object} searchResponse
// @Failure      400  {object} errorResponse
// @Failure      429  {object} errorResponse
// @Router       /auctions/search/ai [post]
func (h *handler) searchAI(w http.ResponseWriter, r *http.Request) {
	var req searchRequest
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
		slog.Warn("ai search fell back to keywords", "err", err)
	}
	h.search(w, r, service.Filter{Text: req.Query}, false)
}

// search runs f. Only free-text searches (AI or keywords) relax when nothing
// matches; the catalog's explicit filters are applied as given.
func (h *handler) search(w http.ResponseWriter, r *http.Request, f service.Filter, ai bool) {
	res, err := h.auctions.Search(r.Context(), f, r.URL.Path == "/auctions/search/ai")
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, searchResponse{AI: ai, SearchResult: res})
}

// publish godoc
// @Summary  Open a draft for bidding (end_time = now + duration)
// @Tags     auctions
// @Produce  json
// @Security BearerAuth
// @Param    id  path     int true "Auction ID"
// @Success  200 {object} service.Auction
// @Failure  403 {object} errorResponse "not the seller"
// @Failure  404 {object} errorResponse
// @Failure  409 {object} errorResponse "not a draft"
// @Router   /auctions/{id}/publish [post]
func (h *handler) publish(w http.ResponseWriter, r *http.Request) {
	h.respond(w, r, h.auctions.Publish)
}

// cancel godoc
// @Summary  Cancel a draft or active auction that has no bids
// @Tags     auctions
// @Produce  json
// @Security BearerAuth
// @Param    id  path     int true "Auction ID"
// @Success  200 {object} service.Auction
// @Failure  403 {object} errorResponse "not the seller"
// @Failure  404 {object} errorResponse
// @Failure  409 {object} errorResponse "already has bids or closed"
// @Router   /auctions/{id}/cancel [post]
func (h *handler) cancel(w http.ResponseWriter, r *http.Request) {
	h.respond(w, r, h.auctions.Cancel)
}

// buyNow godoc
// @Summary  Buy at buy_now_price, only before the first bid
// @Tags     auctions
// @Produce  json
// @Security BearerAuth
// @Param    id  path     int true "Auction ID"
// @Success  200 {object} service.Auction
// @Failure  403 {object} errorResponse "own auction"
// @Failure  404 {object} errorResponse
// @Failure  409 {object} errorResponse "closed, no buy-now price, has bids, or insufficient balance"
// @Router   /auctions/{id}/buy-now [post]
func (h *handler) buyNow(w http.ResponseWriter, r *http.Request) {
	h.respond(w, r, h.auctions.BuyNow)
}

// bid godoc
// @Summary      Place a bid
// @Description  Escrows the amount, releases the previous leader, and extends end_time by 2 minutes when placed in the final 2 minutes.
// @Tags         auctions
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path     int           true "Auction ID"
// @Param        body body     amountRequest true "Bid in rupiah"
// @Success      200  {object} service.Auction
// @Failure      400  {object} errorResponse
// @Failure      403  {object} errorResponse "own auction"
// @Failure      404  {object} errorResponse
// @Failure      409  {object} errorResponse "closed, too low, already leading, or insufficient balance"
// @Failure      429  {object} errorResponse
// @Router       /auctions/{id}/bids [post]
func (h *handler) bid(w http.ResponseWriter, r *http.Request) {
	var req amountRequest
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

// upload godoc
// @Summary  Upload an item photo (JPEG, PNG or WebP, at most 5 MiB)
// @Tags     auctions
// @Accept   multipart/form-data
// @Produce  json
// @Security BearerAuth
// @Param    image formData file true "Photo"
// @Success  201   {object} uploadResponse "use url as item.image_url"
// @Failure  400   {object} errorResponse
// @Failure  401   {object} errorResponse
// @Failure  429   {object} errorResponse
// @Router   /uploads [post]
func (h *handler) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+64<<10) // room for multipart headers
	f, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, fmt.Errorf("%w: send one image of at most 5 MiB in the \"image\" field", service.ErrInvalidInput))
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxUpload+1))
	if err != nil || header.Size > maxUpload || len(data) > maxUpload {
		writeError(w, fmt.Errorf("%w: image must be at most 5 MiB", service.ErrInvalidInput))
		return
	}
	// Trust the bytes, not the file name or client Content-Type.
	ext, ok := imageTypes[http.DetectContentType(data)]
	if !ok {
		writeError(w, fmt.Errorf("%w: image must be JPEG, PNG or WebP", service.ErrInvalidInput))
		return
	}
	name := rand.Text() + ext
	if err := os.WriteFile(filepath.Join(h.uploads, name), data, 0o644); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, uploadResponse{URL: "/uploads/" + name})
}

// files serves uploaded photos without directory listings.
func files(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			writeError(w, service.ErrNotFound)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fs.ServeHTTP(w, r)
	})
}

// respond runs a body-less (user, auction) service call such as publish or cancel.
func (h *handler) respond(w http.ResponseWriter, r *http.Request, fn func(ctx context.Context, userID, id int64) (service.Auction, error)) {
	a, err := fn(r.Context(), middleware.UserID(r.Context()), pathID(r))
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
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
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
		slog.Error("internal error", "err", err)
		msg = "internal server error"
	}
	writeJSON(w, status, errorResponse{Error: msg})
}
