package instagramwebhook

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Handler struct {
	verifyToken string
	secret      string
	store       *Store
	logger      *log.Logger
}

func NewHandler(verifyToken string, secret string, store *Store, logger *log.Logger) *Handler {
	return &Handler{
		verifyToken: verifyToken,
		secret:      secret,
		store:       store,
		logger:      logger,
	}
}

func (h *Handler) InstagramWebhook(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.verify(w, r)
	case http.MethodPost:
		h.ingest(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	mode := query.Get("hub.mode")
	token := query.Get("hub.verify_token")
	challenge := query.Get("hub.challenge")

	if mode != "subscribe" || token == "" || challenge == "" {
		http.Error(w, "invalid verification request", http.StatusBadRequest)
		return
	}

	if token != h.verifyToken {
		http.Error(w, "verification token mismatch", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(challenge))
}

func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if !authorizedBearer(r, h.secret) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload MetaWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json payload", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	processed := 0
	skipped := 0

	for _, event := range payload.ReelEvents() {
		code := ExtractMovieCode(event.Caption)
		shortcode := ExtractShortcode(event.Permalink)

		if code == "" || shortcode == "" {
			skipped++
			h.logger.Printf("[WARN] skipped instagram webhook event: missing code or shortcode permalink=%q", event.Permalink)
			continue
		}

		movieID, err := h.store.BindReelToMovieCode(ctx, code, shortcode, event.Permalink)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				skipped++
				h.logger.Printf("[WARN] skipped instagram webhook event: movie code %q was not found", code)
				continue
			}

			h.logger.Printf("[ERROR] failed to bind instagram reel shortcode=%q code=%q: %v", shortcode, code, err)
			http.Error(w, "failed to process webhook", http.StatusInternalServerError)
			return
		}

		processed++
		h.logger.Printf("[INFO] bound instagram reel shortcode=%q code=%q movie_id=%d", shortcode, code, movieID)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(struct {
		Status    string `json:"status"`
		Processed int    `json:"processed"`
		Skipped   int    `json:"skipped"`
	}{
		Status:    "ok",
		Processed: processed,
		Skipped:   skipped,
	})
}

func authorizedBearer(r *http.Request, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return false
	}

	token := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(token, "Bearer ") {
		return false
	}

	token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
	if token == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}
