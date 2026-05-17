package bot

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"tg-movie-bot/internal/model"
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
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if u.Message != nil || u.CallbackQuery != nil {
		h.botService.PushUpdate(u)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *BotHandler) MetaReelHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ReelLink  string `json:"reel_link"`
		MovieCode string `json:"movie_code"`
		AuthToken string `json:"auth_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
	movieID, _, _, err := h.botService.pgRepo.GetMovieByCode(ctx, req.MovieCode)
	if err != nil {
		http.Error(w, "Movie code not found in system", http.StatusNotFound)
		return
	}

	err = h.botService.pgRepo.SaveReel(ctx, req.ReelLink, movieID)
	if err != nil {
		http.Error(w, "Database injection failure", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Reel successfully linked to movie"})
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
	usersCount, _ := h.botService.pgRepo.GetTotalUsersCount(ctx)
	moviesCount, _ := h.botService.pgRepo.GetTotalMoviesCount(ctx)
	channels, _ := h.botService.pgRepo.GetActiveChannels(ctx)

	resp := map[string]interface{}{
		"total_users":     usersCount,
		"total_movies":    moviesCount,
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
		channels, err := h.botService.pgRepo.GetActiveChannels(ctx)
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
	movies, err := h.botService.pgRepo.GetAllMovies(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(movies)
}
