package bot

import (
	"encoding/json"
	"net/http"
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
