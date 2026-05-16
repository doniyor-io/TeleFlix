package bot

import (
	"context"
	"log"
	"net/url"
	"runtime"
	"strings"
	"tg-movie-bot/config"
	"tg-movie-bot/internal/model"
	"tg-movie-bot/internal/repository"
	"tg-movie-bot/pkg/telegram"
)

type BotService struct {
	cfg         *config.Config
	pgRepo      *repository.PostgresRepository
	redisRepo   *repository.RedisRepository
	tgClient    *telegram.TelegramClient
	updateQueue chan model.Update
}

func NewBotService(cfg *config.Config, pg *repository.PostgresRepository, rds *repository.RedisRepository, tg *telegram.TelegramClient) *BotService {
	queue := make(chan model.Update, 100000)

	botService := &BotService{
		cfg:         cfg,
		pgRepo:      pg,
		redisRepo:   rds,
		tgClient:    tg,
		updateQueue: queue,
	}

	botService.startWorkerPool()
	return botService
}

func (s *BotService) startWorkerPool() {
	numWorkers := runtime.NumCPU() * 2
	for i := 0; i < numWorkers; i++ {
		go s.worker(i)
	}
	log.Printf("[INFO] %d Go Worker threads initialized successfully", numWorkers)
}

func (s *BotService) worker(workerID int) {
	ctx := context.Background()
	for update := range s.updateQueue {
		if update.Message != nil {
			s.handleMessage(ctx, workerID, update.Message)
		} else if update.CallbackQuery != nil {
			s.handleCallbackQuery(ctx, workerID, update.CallbackQuery)
		}
	}
}

func normalizeInstagramURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	path := u.Path
	path = strings.Replace(path, "/reels/", "/reel/", 1)
	path = strings.TrimSuffix(path, "/")

	return "instagram.com" + path
}

func (s *BotService) handleMessage(ctx context.Context, workerID int, msg *model.Message) {
	log.Printf("[Worker %d] ChatID: %d | Text: %s", workerID, msg.Chat.ID, msg.Text)

	userID := msg.From.ID
	chatID := msg.Chat.ID

	userLang, err := s.redisRepo.GetUserLangCache(ctx, userID)
	if err != nil {
		userLang, _ = s.pgRepo.GetUserLang(ctx, userID)
		_ = s.redisRepo.SetUserLangCache(ctx, userID, userLang)
	}

	if s.isAdmin(msg.From.ID) {
		if strings.HasPrefix(msg.Text, "/add_movie") {
			s.handleAdminAddMovie(ctx, msg)
			return
		}
	}

	if msg.Text == "/start" {
		var welcomeTxt string

		if s.isAdmin(userID) {
			welcomeTxt = "STATUS: AUTH_SUCCESS\nRole: Administrator\n\nSelect system interface language:"
		} else {
			welcomeTxt = "Welcome to Cinema Bot API.\n\nSelect your language:"
		}

		s.tgClient.SendInlineKeyboard(ctx, chatID, welcomeTxt, [][]model.InlineButton{
			{
				{Text: "🇺🇿 UZ", Data: "adm_uz"},
				{Text: "🇷🇺 RU", Data: "adm_ru"},
				{Text: "🇺🇸 EN", Data: "adm_en"},
			},
		})
		return
	}

	if strings.Contains(msg.Text, "instagram.com/") {
		s.handleUserMovieRequest(ctx, msg, userLang)
		return
	}

	s.tgClient.SendMessage(ctx, chatID, T(userLang, "send_link"))
}

func (s *BotService) handleCallbackQuery(ctx context.Context, workerID int, callback *model.CallbackQuery) {
	log.Printf("[Worker %d] Callback signal received | Data: %s", workerID, callback.Data)

	chatID := callback.Message.Chat.ID
	userID := callback.From.ID
	data := callback.Data

	// Stop loading animation on the button immediately
	s.tgClient.AnswerCallbackQuery(ctx, callback.ID)

	// Process language selection signals
	if strings.HasPrefix(data, "adm_") {
		var responseText, yesBtn, noBtn string
		newLang := "uz"

		switch data {
		case "adm_uz":
			newLang = "uz"
			if s.isAdmin(userID) {
				responseText = "Access granted. Root privileges enabled.\nInitialize Telegram Mini App (TMA) session?"
				yesBtn = "$ exec tma_init --open"
				noBtn = "$ exit"
			} else {
				responseText = "Language set to: UZ\nSystem ready. Send Instagram URL."
			}
		case "adm_ru":
			newLang = "ru"
			if s.isAdmin(userID) {
				responseText = "Доступ разрешен. Права администратора активны.\nИнициализировать сессию TMA?"
				yesBtn = "$ exec tma_init --open"
				noBtn = "$ exit"
			} else {
				responseText = "Язык изменен на: RU\nСистема готова. Отправьте ссылку Instagram."
			}
		case "adm_en":
			newLang = "en"
			if s.isAdmin(userID) {
				responseText = "Access granted. Root privileges enabled.\nInitialize Telegram Mini App (TMA) session?"
				yesBtn = "$ exec tma_init --open"
				noBtn = "$ exit"
			} else {
				responseText = "Language set to: EN\nSystem ready. Send Instagram URL."
			}
		}

		_ = s.pgRepo.SaveUserLang(ctx, userID, callback.From.Username, newLang)
		_ = s.redisRepo.SetUserLangCache(ctx, userID, newLang)

		s.tgClient.DeleteMessage(ctx, chatID, callback.Message.MessageID)

		if s.isAdmin(userID) {
			tmaURL := s.cfg.WebhookURL
			s.tgClient.SendWebAppButton(ctx, chatID, responseText, yesBtn, tmaURL, noBtn, "close_panel")
		} else {
			s.tgClient.SendMessage(ctx, chatID, responseText)
		}
		return
	}

	// Terminate session if admin cancels
	if data == "close_panel" {
		s.tgClient.DeleteMessage(ctx, chatID, callback.Message.MessageID)
		s.tgClient.SendMessage(ctx, chatID, "Session terminated. TMA initialization aborted.")
		return
	}
}

func (s *BotService) isAdmin(userID int64) bool {
	for _, adminID := range s.cfg.AdminIDs {
		if adminID == userID {
			return true
		}
	}
	return false
}

func (s *BotService) handleAdminAddMovie(ctx context.Context, msg *model.Message) {
	parts := strings.SplitN(msg.Text, " ", 4)
	if len(parts) < 3 {
		s.tgClient.SendMessage(ctx, msg.Chat.ID, "❌ Format error!\nUsage: `/add_movie <insta_link> <file_id> <comment>`")
		return
	}

	instaURL := normalizeInstagramURL(parts[1])
	fileID := parts[2]
	caption := ""
	if len(parts) == 4 {
		caption = parts[3]
	}

	err := s.pgRepo.SaveMovie(ctx, instaURL, fileID, caption)
	if err != nil {
		log.Printf("[ERROR] Database operation failed (SaveMovie): %v", err)
		s.tgClient.SendMessage(ctx, msg.Chat.ID, "❌ Database write failure.")
		return
	}

	s.tgClient.SendMessage(ctx, msg.Chat.ID, "✅ Record updated successfully.")
}

func (s *BotService) handleUserMovieRequest(ctx context.Context, msg *model.Message, lang string) {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	instaURL := normalizeInstagramURL(msg.Text)

	isSubbed, err := s.redisRepo.GetSubscriptionCache(ctx, userID)
	if err != nil {
		isSubbed = s.checkChannelsMembership(ctx, userID)
		_ = s.redisRepo.SetSubscriptionCache(ctx, userID, isSubbed)
	}

	if !isSubbed {
		channels, _ := s.pgRepo.GetActiveChannels(ctx)
		text := T(lang, "force_sub") + "\n\n"
		for i, ch := range channels {
			text += string(rune(i+49)) + ") " + ch["link"].(string) + "\n"
		}
		s.tgClient.SendMessage(ctx, chatID, text)
		return
	}

	fileID, caption, err := s.pgRepo.GetMovieByInstagramURL(ctx, instaURL)
	if err != nil {
		s.tgClient.SendMessage(ctx, chatID, T(lang, "movie_not_found"))
		return
	}

	err = s.tgClient.SendVideo(ctx, chatID, fileID, caption)
	if err != nil {
		log.Printf("[ERROR] Video transmission failure: %v", err)
	}
}

func (s *BotService) checkChannelsMembership(ctx context.Context, userID int64) bool {
	channels, err := s.pgRepo.GetActiveChannels(ctx)
	if err != nil || len(channels) == 0 {
		return true
	}

	for _, ch := range channels {
		chID := ch["id"].(int64)
		subbed, err := s.tgClient.IsChatMember(ctx, chID, userID)
		if err != nil || !subbed {
			return false
		}
	}
	return true
}

func (s *BotService) PushUpdate(u model.Update) {
	select {
	case s.updateQueue <- u:
	default:
		log.Printf("[WARN] Core overflow: Update queue is full. Frame dropped.")
	}
}
