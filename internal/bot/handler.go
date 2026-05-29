package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"tg-movie-bot/internal/model"
	"tg-movie-bot/internal/repository"
)

type BotHandler struct {
	botService *BotService
}

func NewBotHandler(service *BotService) *BotHandler {
	return &BotHandler{botService: service}
}

func (h *BotHandler) WebhookHTTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var u model.Update
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		log.Printf("[WEBHOOK ERROR] failed to decode update: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if u.Message != nil || u.CallbackQuery != nil {
		h.botService.PushUpdate(u)
	}

	w.WriteHeader(http.StatusOK)
}

type MetaWebhookPayload struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Time    int64  `json:"time"`
		Changes []struct {
			Field string `json:"field"`
			Value struct {
				MediaID   string `json:"media_id"`
				ID        string `json:"id"`
				Text      string `json:"text"`
				PostID    string `json:"post_id"`
				Permalink string `json:"permalink"`
				Caption   string `json:"caption"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

// Fallback struct for the simplified prompt.txt payload
type SimpleReelPayload struct {
	ReelLink  string `json:"reel_link"`
	MovieCode string `json:"movie_code"`
	AuthToken string `json:"auth_token"`
}

func (h *BotHandler) MetaReelHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		mode := r.URL.Query().Get("hub.mode")
		token := r.URL.Query().Get("hub.verify_token")
		challenge := r.URL.Query().Get("hub.challenge")

		if mode == "subscribe" && token == h.botService.cfg.MetaWebhookVerifyToken {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(challenge))
			return
		}
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Try the raw Meta Webhook Payload first
	var metaPayload MetaWebhookPayload
	err = json.Unmarshal(bodyBytes, &metaPayload)

	if err == nil && metaPayload.Object == "instagram" && len(metaPayload.Entry) > 0 {
		// Valid Meta Webhook
		h.processMetaWebhook(metaPayload)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("EVENT_RECEIVED"))
		return
	}

	// Try simplified payload from prompt.txt
	var req SimpleReelPayload
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		log.Printf("[META REEL ERROR] failed to decode request: %v", err)
		http.Error(w, "Bad request body", http.StatusBadRequest)
		return
	}

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		token = req.AuthToken
	}

	if token != h.botService.cfg.MetaWebhookSecret {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	movie, err := h.botService.pgRepo.GetMovieByCode(ctx, req.MovieCode)
	if err != nil {
		http.Error(w, "Movie code not found in system", http.StatusNotFound)
		return
	}

	shortcode := repository.ExtractShortcode(req.ReelLink)
	if shortcode == "" {
		http.Error(w, "Invalid reel link", http.StatusBadRequest)
		return
	}

	err = h.botService.pgRepo.CreateReel(ctx, shortcode, req.ReelLink, int64(movie.ID))
	if err != nil {
		http.Error(w, "Database injection failure", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Reel successfully linked to movie"})
}

func (h *BotHandler) processMetaWebhook(payload MetaWebhookPayload) {
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			caption := change.Value.Text
			if caption == "" {
				caption = change.Value.Caption
			}

			// Look for #MOVxxxx in caption
			words := strings.Fields(caption)
			var movieCode string
			for _, w := range words {
				if strings.HasPrefix(w, "#MOV") {
					movieCode = strings.TrimPrefix(w, "#")
					break
				}
			}

			if movieCode == "" {
				continue // No movie code in this webhook
			}

			permalink := change.Value.Permalink
			mediaID := change.Value.MediaID
			if mediaID == "" {
				mediaID = change.Value.ID
			}

			if permalink == "" && mediaID != "" {
				// We need to fetch the permalink from Graph API
				fetchedLink := h.fetchMediaPermalink(mediaID)
				if fetchedLink != "" {
					permalink = fetchedLink
				}
			}

			if permalink != "" {
				ctx := context.Background()
				movie, err := h.botService.pgRepo.GetMovieByCode(ctx, movieCode)
				if err == nil {
					shortcode := repository.ExtractShortcode(permalink)
					if shortcode != "" {
						_ = h.botService.pgRepo.CreateReel(ctx, shortcode, permalink, int64(movie.ID))
						log.Printf("[META WEBHOOK] Successfully linked %s to %s", permalink, movieCode)
					}
				}
			}
		}
	}
}

func (h *BotHandler) fetchMediaPermalink(mediaID string) string {
	token := h.botService.cfg.InstagramAccessToken
	if token == "" {
		return ""
	}

	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s?fields=permalink&access_token=%s", mediaID, token)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("[GRAPH API ERROR] %v", err)
		return ""
	}
	defer resp.Body.Close()

	var res struct {
		Permalink string `json:"permalink"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err == nil {
		return res.Permalink
	}
	return ""
}

func (h *BotHandler) CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, ngrok-skip-browser-warning")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *BotHandler) GetStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	stats, _ := h.botService.pgRepo.GetStatistics(ctx)
	channels, _ := h.botService.pgRepo.GetChannels(ctx)

	resp := map[string]interface{}{
		"total_users":     stats["users"],
		"total_movies":    stats["movies"],
		"total_reels":     stats["reels"],
		"active_channels": len(channels),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *BotHandler) ChannelsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		channels, err := h.botService.pgRepo.GetChannels(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(channels)

	case http.MethodPost:
		var req struct {
			ChannelID  string `json:"tg_channel_id"`
			InviteLink string `json:"invite_link"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[ADMIN CHANNELS ERROR] invalid add channel payload: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		chID, err := strconv.ParseInt(req.ChannelID, 10, 64)
		if err != nil {
			http.Error(w, "Invalid Channel ID format", http.StatusBadRequest)
			return
		}

		if err := h.botService.pgRepo.AddChannel(ctx, chID, req.InviteLink); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Channel added successfully"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *BotHandler) DeleteChannelHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	chID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid Channel ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if err := h.botService.pgRepo.DeleteChannel(ctx, chID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Channel deleted successfully"})
}

func (h *BotHandler) GetMoviesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	movies, err := h.botService.pgRepo.GetMovies(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(movies)
}
