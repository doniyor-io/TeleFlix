package bot

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"runtime"
	"strconv"
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
			s.handleCallbackQuery(ctx, update.CallbackQuery)
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

	// ADMIN POLING REPLIES
	if s.isAdmin(userID) {
		switch msg.Text {
		case T(userLang, "btn_stats"):
			uCount, _ := s.pgRepo.GetTotalUsersCount(ctx)
			mCount, _ := s.pgRepo.GetTotalMoviesCount(ctx)
			ch, _ := s.pgRepo.GetActiveChannels(ctx)
			txt := T(userLang, "stats_text")
			txt = strings.NewReplacer("{users}", fmt.Sprintf("%d", uCount), "{movies}", fmt.Sprintf("%d", mCount), "{channels}", fmt.Sprintf("%d", len(ch))).Replace(txt)
			err := s.tgClient.SendMessage(ctx, chatID, txt)
			if err != nil {
				return
			}
			return

		case T(userLang, "btn_movies"):
			list, _ := s.pgRepo.GetLatestMoviesList(ctx, 10)
			if len(list) == 0 {
				err = s.tgClient.SendMessage(ctx, chatID, T(userLang, "no_movie"))
				if err != nil {
					return
				}
				return
			}
			err := s.tgClient.SendMessage(ctx, chatID, strings.Join(list, "\n"))
			if err != nil {
				return
			}
			return

		case T(userLang, "btn_channels"):
			channels, _ := s.pgRepo.GetActiveChannels(ctx)
			if len(channels) == 0 {
				err := s.tgClient.SendMessage(ctx, chatID, T(userLang, "no_channel"))
				if err != nil {
					return
				}
				return
			}
			var sb strings.Builder
			sb.WriteString("📢 Faol kanallar:\n")
			for i, ch := range channels {
				sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, ch["link"].(string)))
			}
			err := s.tgClient.SendMessage(ctx, chatID, sb.String())
			if err != nil {
				return
			}
			return

		case T(userLang, "btn_add_movie"):
			err := s.tgClient.SendMessage(ctx, chatID, T(userLang, "add_movie_hint"))
			if err != nil {
				return
			}
			return

		case T(userLang, "btn_web_panel"):
			err := s.tgClient.SendInlineKeyboard(ctx, chatID, "🌐 Web Panelni ochish:", [][]model.InlineButton{{
				{Text: "💻 Open WebApp", URL: s.cfg.WebhookURL},
			}})
			if err != nil {
				return
			}
			return
		}

		if strings.HasPrefix(msg.Text, "/add_movie") {
			s.handleAdminAddMovieCommand(ctx, msg)
			return
		}
	}

	if msg.Text == "/start" {
		var welcomeTxt string
		if s.isAdmin(userID) {
			welcomeTxt = T(userLang, "welcome_admin")
		} else {
			welcomeTxt = T(userLang, "welcome_user")
		}

		err := s.tgClient.SendInlineKeyboard(ctx, chatID, welcomeTxt, [][]model.InlineButton{
			{
				{Text: "🇺🇿 UZ", Data: "lang_uz"},
				{Text: "🇷🇺 RU", Data: "lang_ru"},
				{Text: "🇬🇧 EN", Data: "lang_en"},
			},
		})
		if err != nil {
			return
		}
		return
	}

	if strings.Contains(msg.Text, "instagram.com/") {
		s.handleUserMovieRequest(ctx, msg, userLang)
		return
	}

	err = s.tgClient.SendMessage(ctx, chatID, T(userLang, "send_link"))
	if err != nil {
		return
	}
}

func (s *BotService) handleCallbackQuery(ctx context.Context, callback *model.CallbackQuery) {
	chatID := callback.Message.Chat.ID
	userID := callback.From.ID
	data := callback.Data

	err := s.tgClient.AnswerCallbackQuery(ctx, callback.ID)
	if err != nil {
		return
	}

	if strings.HasPrefix(data, "lang_") {
		newLang := strings.TrimPrefix(data, "lang_")
		_ = s.pgRepo.SaveUserLang(ctx, userID, callback.From.Username, newLang)
		_ = s.redisRepo.SetUserLangCache(ctx, userID, newLang)

		err := s.tgClient.DeleteMessage(ctx, chatID, callback.Message.MessageID)
		if err != nil {
			return
		}

		if s.isAdmin(userID) {
			_ = s.tgClient.SetMenuButtonForChat(ctx, chatID, s.cfg.WebhookURL)

			buttons := [][]string{
				{T(newLang, "btn_stats"), T(newLang, "btn_movies")},
				{T(newLang, "btn_channels"), T(newLang, "btn_add_movie")},
				{T(newLang, "btn_web_panel")},
			}
			err := s.tgClient.SendReplyKeyboard(ctx, chatID, T(newLang, "lang_set_admin"), buttons)
			if err != nil {
				return
			}
		} else {
			err := s.tgClient.SendMessage(ctx, chatID, T(newLang, "lang_set_user"))
			if err != nil {
				return
			}
		}
		return
	}

	if data == "check_sub" {
		_ = s.redisRepo.InvalidateSubscriptionCache(ctx, userID)

		log.Printf("[DEBUG] Checking subscription for user %d", userID)
		channels, err := s.pgRepo.GetActiveChannels(ctx)
		if err != nil {
			log.Printf("[ERROR] Failed to get active channels: %v", err)
		} else {
			log.Printf("[DEBUG] Active channels: %+v", channels)
		}

		isSubbed := s.checkChannelsMembership(ctx, userID)
		log.Printf("[DEBUG] Subscription result for user %d: %v", userID, isSubbed)

		if isSubbed {
			_ = s.redisRepo.SetSubscriptionCache(ctx, userID, isSubbed)
		}

		userLang, err := s.redisRepo.GetUserLangCache(ctx, userID)
		if err != nil || userLang == "" {
			userLang, _ = s.pgRepo.GetUserLang(ctx, userID)
			_ = s.redisRepo.SetUserLangCache(ctx, userID, userLang)
		}

		if isSubbed {
			err := s.tgClient.DeleteMessage(ctx, chatID, callback.Message.MessageID)
			if err != nil {
				return
			}
			err = s.tgClient.SendMessage(ctx, chatID, T(userLang, "sub_success"))
			if err != nil {
				return
			}
		} else {
			err := s.tgClient.SendMessage(ctx, chatID, T(userLang, "not_subbed_yet"))
			if err != nil {
				return
			}
		}
		return
	}
}

func (s *BotService) handleAdminAddMovieCommand(ctx context.Context, msg *model.Message) {

	userLang, err := s.redisRepo.GetUserLangCache(ctx, msg.From.ID)
	parts := strings.SplitN(msg.Text, " ", 3)
	if len(parts) < 2 {
		err := s.tgClient.SendMessage(ctx, msg.Chat.ID, T(userLang, "movie_add_format_error"))
		if err != nil {
			return
		}
		return
	}

	fileID := parts[1]
	caption := ""
	if len(parts) == 3 {
		caption = parts[2]
	}

	code, err := s.pgRepo.GenerateUniqueMovieCode(ctx)
	if err != nil {
		err := s.tgClient.SendMessage(ctx, msg.Chat.ID, T(userLang, "movie_code_generation_error"))
		if err != nil {
			return
		}
		return
	}

	fakeInstaURL := fmt.Sprintf("legacy.com/movie/%s", code)
	err = s.pgRepo.SaveMovie(ctx, fakeInstaURL, fileID, caption, code)
	if err != nil {
		err := s.tgClient.SendMessage(ctx, msg.Chat.ID, T(userLang, "movie_db_write_error"))
		if err != nil {
			return
		}
		return
	}

	resTxt := T(userLang, "movie_added")
	resTxt = strings.NewReplacer("{code}", code, "{caption}", caption).Replace(resTxt)
	err = s.tgClient.SendMessage(ctx, msg.Chat.ID, resTxt)
	if err != nil {
		return
	}
}

func (s *BotService) handleUserMovieRequest(ctx context.Context, msg *model.Message, lang string) {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	instaURL := normalizeInstagramURL(msg.Text)

	isSubbed, err := s.redisRepo.GetSubscriptionCache(ctx, userID)
	if err != nil {
		isSubbed = s.checkChannelsMembership(ctx, userID)
		if isSubbed {
			_ = s.redisRepo.SetSubscriptionCache(ctx, userID, isSubbed)
		}
	}

	if !isSubbed {
		channels, _ := s.pgRepo.GetActiveChannels(ctx)
		var buttons [][]model.InlineButton
		for _, ch := range channels {
			buttons = append(buttons, []model.InlineButton{{
				Text: "📢 Kanalga o'tish",
				URL:  ch["link"].(string),
			}})
		}
		buttons = append(buttons, []model.InlineButton{{
			Text: T(lang, "check_sub_btn"),
			Data: "check_sub",
		}})

		err := s.tgClient.SendInlineKeyboard(ctx, chatID, T(lang, "force_sub"), buttons)
		if err != nil {
			return
		}
		return
	}

	fileID, caption, err := s.pgRepo.GetMovieByReelLink(ctx, "https://"+instaURL)
	if err != nil {
		fileID, caption, err = s.pgRepo.GetMovieByInstagramURL(ctx, instaURL)
		if err != nil {
			err := s.tgClient.SendMessage(ctx, chatID, T(lang, "movie_not_found"))
			if err != nil {
				return
			}
			return
		}
	}

	_ = s.tgClient.SendVideo(ctx, chatID, fileID, caption)
}

func (s *BotService) checkChannelsMembership(ctx context.Context, userID int64) bool {
	channels, err := s.pgRepo.GetActiveChannels(ctx)
	if err != nil || len(channels) == 0 {
		return true
	}

	for _, ch := range channels {
		chID, err := parseChannelID(ch["id"])
		if err != nil {
			log.Printf("[ERROR] Invalid channel ID format: %v", ch["id"])
			continue
		}

		subbed, err := s.tgClient.IsChatMember(ctx, chID, userID)
		if err != nil {
			log.Printf("[CHECK-SUB ERROR] Channel: %d, User: %d, Cause: %v", chID, userID, err)
			return false
		}

		if !subbed {
			return false
		}
	}
	return true
}

func parseChannelID(id interface{}) (int64, error) {
	switch v := id.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("unsupported channel ID type: %T", id)
	}
}

func (s *BotService) isAdmin(userID int64) bool {
	return s.cfg.IsAdmin(userID)
}

func (s *BotService) PushUpdate(u model.Update) {
	select {
	case s.updateQueue <- u:
	default:
		log.Printf("[WARN] Core overflow: Update queue is full. Frame dropped.")
	}
}
