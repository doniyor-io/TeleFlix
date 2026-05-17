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

	// ADMIN POLING REPLIES
	if s.isAdmin(userID) {
		switch msg.Text {
		case T(userLang, "btn_stats"):
			uCount, _ := s.pgRepo.GetTotalUsersCount(ctx)
			mCount, _ := s.pgRepo.GetTotalMoviesCount(ctx)
			ch, _ := s.pgRepo.GetActiveChannels(ctx)
			txt := T(userLang, "stats_text")
			txt = strings.NewReplacer("{users}", fmt.Sprintf("%d", uCount), "{movies}", fmt.Sprintf("%d", mCount), "{channels}", fmt.Sprintf("%d", len(ch))).Replace(txt)
			s.tgClient.SendMessage(ctx, chatID, txt)
			return

		case T(userLang, "btn_movies"):
			list, _ := s.pgRepo.GetLatestMoviesList(ctx, 10)
			if len(list) == 0 {
				s.tgClient.SendMessage(ctx, chatID, "🎬 Kinolar mavjud emas.")
				return
			}
			s.tgClient.SendMessage(ctx, chatID, strings.Join(list, "\n"))
			return

		case T(userLang, "btn_channels"):
			channels, _ := s.pgRepo.GetActiveChannels(ctx)
			if len(channels) == 0 {
				s.tgClient.SendMessage(ctx, chatID, "📢 Kanallar yo'q.")
				return
			}
			var sb strings.Builder
			sb.WriteString("📢 Faol kanallar:\n")
			for i, ch := range channels {
				sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, ch["link"].(string)))
			}
			s.tgClient.SendMessage(ctx, chatID, sb.String())
			return

		case T(userLang, "btn_add_movie"):
			s.tgClient.SendMessage(ctx, chatID, T(userLang, "add_movie_hint"))
			return

		case T(userLang, "btn_web_panel"):
			s.tgClient.SendInlineKeyboard(ctx, chatID, "🌐 Web Panelni ochish:", [][]model.InlineButton{{
				{Text: "💻 Open WebApp", URL: s.cfg.FrontendURL},
			}})
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

		s.tgClient.SendInlineKeyboard(ctx, chatID, welcomeTxt, [][]model.InlineButton{
			{
				{Text: "🇺🇿 UZ", Data: "lang_uz"},
				{Text: "🇷🇺 RU", Data: "lang_ru"},
				{Text: "🇬🇧 EN", Data: "lang_en"},
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
	chatID := callback.Message.Chat.ID
	userID := callback.From.ID
	data := callback.Data

	s.tgClient.AnswerCallbackQuery(ctx, callback.ID)

	if strings.HasPrefix(data, "lang_") {
		newLang := strings.TrimPrefix(data, "lang_")
		_ = s.pgRepo.SaveUserLang(ctx, userID, callback.From.Username, newLang)
		_ = s.redisRepo.SetUserLangCache(ctx, userID, newLang)

		s.tgClient.DeleteMessage(ctx, chatID, callback.Message.MessageID)

		if s.isAdmin(userID) {
			// Faqat shu admin chatID si uchun Menu Button (TMA Frontend URL) set qilinadi
			_ = s.tgClient.SetMenuButtonForChat(ctx, chatID, s.cfg.WebhookURL)

			buttons := [][]string{
				{T(newLang, "btn_stats"), T(newLang, "btn_movies")},
				{T(newLang, "btn_channels"), T(newLang, "btn_add_movie")},
				{T(newLang, "btn_web_panel")},
			}
			s.tgClient.SendReplyKeyboard(ctx, chatID, T(newLang, "lang_set_admin"), buttons)
		} else {
			s.tgClient.SendMessage(ctx, chatID, T(newLang, "lang_set_user"))
		}
		return
	}

	if data == "check_sub" {
		_ = s.redisRepo.InvalidateSubscriptionCache(ctx, userID)
		isSubbed := s.checkChannelsMembership(ctx, userID)
		if isSubbed {
			_ = s.redisRepo.SetSubscriptionCache(ctx, userID, isSubbed)
		}

		userLang, err := s.redisRepo.GetUserLangCache(ctx, userID)
		if err != nil || userLang == "" {
			userLang, _ = s.pgRepo.GetUserLang(ctx, userID)
			_ = s.redisRepo.SetUserLangCache(ctx, userID, userLang)
		}

		if isSubbed {
			s.tgClient.DeleteMessage(ctx, chatID, callback.Message.MessageID)
			s.tgClient.SendMessage(ctx, chatID, T(userLang, "sub_success"))
		} else {
			s.tgClient.SendMessage(ctx, chatID, T(userLang, "not_subbed_yet"))
		}
		return
	}
}

func (s *BotService) handleAdminAddMovieCommand(ctx context.Context, msg *model.Message) {
	parts := strings.SplitN(msg.Text, " ", 3)
	if len(parts) < 2 {
		s.tgClient.SendMessage(ctx, msg.Chat.ID, "❌ Xato format!\nIshlatish: /add_movie <file_id> [caption]")
		return
	}

	fileID := parts[1]
	caption := ""
	if len(parts) == 3 {
		caption = parts[2]
	}

	code, err := s.pgRepo.GenerateUniqueMovieCode(ctx)
	if err != nil {
		s.tgClient.SendMessage(ctx, msg.Chat.ID, "❌ Kod generatsiya qilishda xatolik.")
		return
	}

	fakeInstaURL := fmt.Sprintf("legacy.com/movie/%s", code)
	err = s.pgRepo.SaveMovie(ctx, fakeInstaURL, fileID, caption, code)
	if err != nil {
		s.tgClient.SendMessage(ctx, msg.Chat.ID, "❌ Bazaga yozishda muammo bo'ldi.")
		return
	}

	userLang, _ := s.redisRepo.GetUserLangCache(ctx, msg.From.ID)
	resTxt := T(userLang, "movie_added")
	resTxt = strings.NewReplacer("{code}", code, "{caption}", caption).Replace(resTxt)
	s.tgClient.SendMessage(ctx, msg.Chat.ID, resTxt)
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

		s.tgClient.SendInlineKeyboard(ctx, chatID, T(lang, "force_sub"), buttons)
		return
	}

	fileID, caption, err := s.pgRepo.GetMovieByReelLink(ctx, "https://"+instaURL)
	if err != nil {
		fileID, caption, err = s.pgRepo.GetMovieByInstagramURL(ctx, instaURL)
		if err != nil {
			s.tgClient.SendMessage(ctx, chatID, T(lang, "movie_not_found"))
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
		var chID int64

		switch v := ch["id"].(type) {
		case int64:
			chID = v
		case int:
			chID = int64(v)
		case int32:
			chID = int64(v)
		case float64:
			chID = int64(v)
		default:
			if strID, ok := ch["id"].(string); ok {
				importStr, err := strconv.ParseInt(strID, 10, 64)
				if err == nil {
					chID = importStr
				} else {
					log.Printf("[ERROR] Failed to convert string Channel ID to int64: %s", strID)
					return false
				}
			} else {
				log.Printf("[ERROR] Channel ID type is unknown for Go: %T (%v)", ch["id"], ch["id"])
				return false
			}
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
