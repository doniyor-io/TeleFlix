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

	if msg.Video != nil {
		log.Printf("[VIDEO UTILITY] Worker %d | ChatID: %d | FILE ID: %s", workerID, msg.Chat.ID, msg.Video.FileID)

		if s.isAdmin(msg.From.ID) {
			userLang, err := s.redisRepo.GetUserLanguageCache(ctx, msg.From.ID)
			if err != nil || userLang == "" {
				userLang, _ = s.pgRepo.GetUserLanguage(ctx, msg.From.ID)
			}
			_ = s.redisRepo.SetPendingMovieFileID(ctx, msg.From.ID, msg.Video.FileID)
			_ = s.tgClient.SendMessage(ctx, msg.Chat.ID, T(userLang, "movie_wizard_meta_prompt"))
		}
		return
	}

	userID := msg.From.ID
	chatID := msg.Chat.ID

	userLang, err := s.redisRepo.GetUserLanguageCache(ctx, userID)
	if err != nil {
		userLang, _ = s.pgRepo.GetUserLanguage(ctx, userID)
		_ = s.redisRepo.SetUserLanguageCache(ctx, userID, userLang)
	}

	if !s.isAdmin(userID) && msg.Contact != nil {
		s.handleUserContact(ctx, msg, userLang)
		return
	}

	if s.isAdmin(userID) {
		if s.handlePendingMovieMeta(ctx, msg, userLang) {
			return
		}

		switch msg.Text {
		case T(userLang, "btn_stats"):
			stats, _ := s.pgRepo.GetStatistics(ctx)
			ch, _ := s.pgRepo.GetChannels(ctx)
			txt := T(userLang, "stats_text")
			txt = strings.NewReplacer("{users}", fmt.Sprintf("%d", stats["users"]), "{movies}", fmt.Sprintf("%d", stats["movies"]), "{channels}", fmt.Sprintf("%d", len(ch))).Replace(txt)
			err := s.tgClient.SendMessage(ctx, chatID, txt)
			if err != nil {
				log.Printf("[TG SEND ERROR] stats message failed for chat %d: %v", chatID, err)
				return
			}
			return

		case T(userLang, "btn_movies"):
			movies, _ := s.pgRepo.GetMovies(ctx)
			if len(movies) == 0 {
				err = s.tgClient.SendMessage(ctx, chatID, T(userLang, "no_movie"))
				if err != nil {
					return
				}
				return
			}

			var list []string
			for i, m := range movies {
				if i >= 30 {
					break
				}
				list = append(list, fmt.Sprintf("%d. %s — %s", i+1, m.Title, m.MovieCode))
			}
			if len(movies) > 30 {
				list = append(list, fmt.Sprintf("\n... yana %d ta kino bor. To'liq ro'yxat Web Panelda.", len(movies)-30))
			}

			err := s.tgClient.SendMessage(ctx, chatID, strings.Join(list, "\n"))
			if err != nil {
				return
			}
			return

		case T(userLang, "btn_channels"):
			channels, _ := s.pgRepo.GetChannels(ctx)
			if len(channels) == 0 {
				err := s.tgClient.SendMessage(ctx, chatID, T(userLang, "no_channel"))
				if err != nil {
					return
				}
				return
			}
			var sb strings.Builder
			sb.WriteString("Faol kanallar:\n")
			for i, ch := range channels {
				sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, ch.InviteLink))
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
			err := s.tgClient.SendInlineKeyboard(ctx, chatID, "Web Panelni ochish:", [][]model.InlineButton{{
				{Text: "Open WebApp", URL: s.frontendURL()},
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

		if strings.HasPrefix(msg.Text, "/link_reel") {
			s.handleAdminLinkReelCommand(ctx, msg, userLang)
			return
		}
	}

	if msg.Text == "/start" {
		var welcomeTxt string
		if s.isAdmin(userID) {
			welcomeTxt = T(userLang, "welcome_admin")
		} else {
			welcomeTxt = T(userLang, "welcome_user")
			if err := s.tgClient.ResetMenuButtonForChat(ctx, chatID); err != nil {
				log.Printf("[TG SEND ERROR] reset menu button failed for chat %d: %v", chatID, err)
			}
		}

		s.sendLanguageSelection(ctx, chatID, welcomeTxt)
		return
	}

	if !s.isAdmin(userID) && !s.userHasContact(ctx, userID) {
		_ = s.sendContactRequest(ctx, chatID, userLang)
		return
	}

	if msg.Text == "/top" || msg.Text == T(userLang, "btn_top_movies") {
		s.sendTopMovies(ctx, chatID, userLang)
		return
	}

	if isMovieCode(msg.Text) {
		s.sendMovieByCode(ctx, userID, chatID, strings.TrimSpace(msg.Text), userLang)
		return
	}

	if strings.Contains(msg.Text, "instagram.com/") {
		s.handleUserMovieRequest(ctx, msg, userLang)
		return
	}

	err = s.tgClient.SendMessage(ctx, chatID, T(userLang, "send_link"))
	if err != nil {
		log.Printf("[TG SEND ERROR] fallback prompt failed for chat %d: %v", chatID, err)
		return
	}
}

func (s *BotService) handleCallbackQuery(ctx context.Context, callback *model.CallbackQuery) {
	if callback == nil || callback.From == nil || callback.Message == nil {
		log.Printf("[CALLBACK ERROR] malformed callback payload: %+v", callback)
		return
	}

	chatID := callback.Message.Chat.ID
	userID := callback.From.ID
	data := callback.Data

	err := s.tgClient.AnswerCallbackQuery(ctx, callback.ID)
	if err != nil {
		return
	}

	if strings.HasPrefix(data, "lang_") {
		newLang := strings.TrimPrefix(data, "lang_")
		_ = s.pgRepo.SaveUserLanguage(ctx, userID, callback.From.Username, newLang)
		_ = s.redisRepo.SetUserLanguageCache(ctx, userID, newLang)

		err := s.tgClient.DeleteMessage(ctx, chatID, callback.Message.MessageID)
		if err != nil {
			return
		}

		if s.isAdmin(userID) {
			if err := s.tgClient.SetMenuButtonForChat(ctx, chatID, s.frontendURL()); err != nil {
				log.Printf("[TG SEND ERROR] set menu button failed for chat %d: %v", chatID, err)
			}

			buttons := [][]string{
				{T(newLang, "btn_stats"), T(newLang, "btn_movies")},
				{T(newLang, "btn_channels"), T(newLang, "btn_add_movie")},
				{T(newLang, "btn_web_panel")},
			}
			err := s.tgClient.SendReplyKeyboard(ctx, chatID, T(newLang, "lang_set_admin"), buttons)
			if err != nil {
				log.Printf("[TG SEND ERROR] admin keyboard failed for chat %d: %v", chatID, err)
				return
			}
		} else {
			if err := s.tgClient.ResetMenuButtonForChat(ctx, chatID); err != nil {
				log.Printf("[TG SEND ERROR] reset menu button failed for chat %d: %v", chatID, err)
			}

			messageID, err := s.tgClient.SendMessageWithResult(ctx, chatID, T(newLang, "lang_set_user"))
			if err != nil {
				log.Printf("[TG SEND ERROR] lang confirmation failed for chat %d: %v", chatID, err)
				return
			}
			if err := s.tgClient.PinChatMessage(ctx, chatID, messageID); err != nil {
				log.Printf("[TG SEND ERROR] pin language confirmation failed for chat %d: %v", chatID, err)
			}
			if !s.userHasContact(ctx, userID) {
				_ = s.sendContactRequest(ctx, chatID, newLang)
			}
		}
		return
	}

	if data == "check_sub" {
		_ = s.redisRepo.DeleteSubscriptionCache(ctx, userID)

		log.Printf("[DEBUG] Checking subscription for user %d", userID)
		isSubbed := s.checkChannelsMembership(ctx, userID)
		log.Printf("[DEBUG] Subscription result for user %d: %v", userID, isSubbed)

		if isSubbed {
			_ = s.redisRepo.SetSubscriptionCache(ctx, userID, isSubbed)
		}

		userLang, err := s.redisRepo.GetUserLanguageCache(ctx, userID)
		if err != nil || userLang == "" {
			userLang, _ = s.pgRepo.GetUserLanguage(ctx, userID)
			_ = s.redisRepo.SetUserLanguageCache(ctx, userID, userLang)
		}

		if isSubbed {
			err := s.tgClient.DeleteMessage(ctx, chatID, callback.Message.MessageID)
			if err != nil {
				return
			}
			err = s.tgClient.SendMessage(ctx, chatID, T(userLang, "sub_success"))
			if err != nil {
				log.Printf("[TG SEND ERROR] sub success message failed for chat %d: %v", chatID, err)
				return
			}
		} else {
			_ = s.sendSubscriptionPrompt(ctx, chatID, userLang, T(userLang, "not_subbed_yet"))
		}
		return
	}
}

func (s *BotService) handleUserContact(ctx context.Context, msg *model.Message, lang string) {
	contact := msg.Contact
	userID := msg.From.ID
	chatID := msg.Chat.ID

	if contact.UserID != 0 && contact.UserID != userID {
		_ = s.tgClient.SendMessage(ctx, chatID, T(lang, "contact_invalid"))
		_ = s.sendContactRequest(ctx, chatID, lang)
		return
	}

	if strings.TrimSpace(contact.PhoneNumber) == "" {
		_ = s.tgClient.SendMessage(ctx, chatID, T(lang, "contact_invalid"))
		_ = s.sendContactRequest(ctx, chatID, lang)
		return
	}

	firstName := contact.FirstName
	if firstName == "" {
		firstName = msg.From.FirstName
	}
	lastName := contact.LastName
	if lastName == "" {
		lastName = msg.From.LastName
	}

	if err := s.pgRepo.SaveUserContact(ctx, userID, msg.From.Username, contact.PhoneNumber, firstName, lastName); err != nil {
		log.Printf("[USER ERROR] failed to save contact for user %d: %v", userID, err)
		_ = s.tgClient.SendMessage(ctx, chatID, T(lang, "contact_save_error"))
		return
	}

	if err := s.tgClient.SendRemoveKeyboardMessage(ctx, chatID, T(lang, "contact_saved")); err != nil {
		log.Printf("[TG SEND ERROR] remove contact keyboard failed for chat %d: %v", chatID, err)
	}
}

func (s *BotService) sendContactRequest(ctx context.Context, chatID int64, lang string) error {
	return s.tgClient.SendContactRequestKeyboard(ctx, chatID, T(lang, "contact_required"), T(lang, "contact_button"))
}

func (s *BotService) sendLanguageSelection(ctx context.Context, chatID int64, text string) {
	err := s.tgClient.SendInlineKeyboard(ctx, chatID, text, [][]model.InlineButton{
		{
			{Text: "🇺🇿 UZ", Data: "lang_uz"},
			{Text: "🇷🇺 RU", Data: "lang_ru"},
			{Text: "🇬🇧 EN", Data: "lang_en"},
		},
	})
	if err != nil {
		log.Printf("[TG SEND ERROR] language keyboard failed for chat %d: %v", chatID, err)
	}
}

func (s *BotService) userExists(ctx context.Context, userID int64) bool {
	exists, err := s.pgRepo.UserExists(ctx, userID)
	if err != nil {
		log.Printf("[USER ERROR] failed to check user existence for %d: %v", userID, err)
		return false
	}
	return exists
}

func (s *BotService) userHasContact(ctx context.Context, userID int64) bool {
	exists, err := s.pgRepo.UserHasContact(ctx, userID)
	if err != nil {
		log.Printf("[USER ERROR] failed to check user contact for %d: %v", userID, err)
		return false
	}
	return exists
}

func (s *BotService) handleAdminAddMovieCommand(ctx context.Context, msg *model.Message) {

	userLang, err := s.redisRepo.GetUserLanguageCache(ctx, msg.From.ID)
	if err != nil {
		userLang = "uz"
	}
	parts := strings.SplitN(msg.Text, " ", 6)
	if len(parts) < 4 {
		err := s.tgClient.SendMessage(ctx, msg.Chat.ID, T(userLang, "movie_add_format_error"))
		if err != nil {
			return
		}
		return
	}

	input := repository.CreateMovieInput{
		FileID: parts[1],
		Title:  parts[2],
		Rating: parts[3],
	}
	if len(parts) >= 5 {
		input.Language = parts[4]
	}
	if len(parts) == 6 {
		input.Caption = parts[5]
	}

	movie, err := s.pgRepo.CreateMovie(ctx, input)
	if err != nil {
		err := s.tgClient.SendMessage(ctx, msg.Chat.ID, T(userLang, "movie_db_write_error"))
		if err != nil {
			return
		}
		return
	}

	resTxt := T(userLang, "movie_added")
	resTxt = strings.NewReplacer(
		"{code}", movie.MovieCode,
		"{title}", movie.Title,
		"{rating}", movie.Rating,
		"{language}", movie.Language,
		"{caption}", movie.Caption,
	).Replace(resTxt)
	err = s.tgClient.SendMessage(ctx, msg.Chat.ID, resTxt)
	if err != nil {
		return
	}
}

func (s *BotService) handlePendingMovieMeta(ctx context.Context, msg *model.Message, lang string) bool {
	if strings.HasPrefix(msg.Text, "/") {
		return false
	}

	fileID, err := s.redisRepo.GetPendingMovieFileID(ctx, msg.From.ID)
	if err != nil || strings.TrimSpace(fileID) == "" {
		return false
	}

	input, err := parseMovieMetaInput(msg.Text)
	if err != nil {
		_ = s.tgClient.SendMessage(ctx, msg.Chat.ID, T(lang, "movie_wizard_format_error"))
		return true
	}
	input.FileID = fileID

	movie, err := s.pgRepo.CreateMovie(ctx, input)
	if err != nil {
		log.Printf("[MOVIE ERROR] failed to create movie from wizard: %v", err)
		_ = s.tgClient.SendMessage(ctx, msg.Chat.ID, T(lang, "movie_db_write_error"))
		return true
	}

	_ = s.redisRepo.DeletePendingMovieFileID(ctx, msg.From.ID)
	resTxt := strings.NewReplacer(
		"{code}", movie.MovieCode,
		"{title}", movie.Title,
		"{rating}", movie.Rating,
		"{language}", movie.Language,
		"{caption}", movie.Caption,
	).Replace(T(lang, "movie_added"))
	_ = s.tgClient.SendMessage(ctx, msg.Chat.ID, resTxt)
	return true
}

func parseMovieMetaInput(text string) (repository.CreateMovieInput, error) {
	parts := strings.SplitN(text, "|", 4)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) < 1 || parts[0] == "" {
		return repository.CreateMovieInput{}, fmt.Errorf("movie title is required")
	}

	input := repository.CreateMovieInput{Title: parts[0]}
	if len(parts) > 1 {
		input.Rating = parts[1]
	}
	if len(parts) > 2 {
		input.Language = parts[2]
	}
	if len(parts) > 3 {
		input.Caption = parts[3]
	}
	return input, nil
}

func (s *BotService) handleAdminLinkReelCommand(ctx context.Context, msg *model.Message, lang string) {
	parts := strings.Fields(msg.Text)
	if len(parts) != 3 {
		_ = s.tgClient.SendMessage(ctx, msg.Chat.ID, T(lang, "link_reel_format_error"))
		return
	}
	if err := s.linkReelToMovie(ctx, parts[1], parts[2]); err != nil {
		_ = s.tgClient.SendMessage(ctx, msg.Chat.ID, T(lang, "link_reel_error"))
		return
	}
	_ = s.tgClient.SendMessage(ctx, msg.Chat.ID, T(lang, "link_reel_success"))
}

func (s *BotService) linkReelToMovie(ctx context.Context, code string, reelURL string) error {
	movie, err := s.pgRepo.GetMovieByCode(ctx, strings.Trim(strings.TrimSpace(code), "#"))
	if err != nil {
		return err
	}
	shortcode := repository.ExtractShortcode(reelURL)
	if shortcode == "" {
		return fmt.Errorf("invalid reel link")
	}
	return s.pgRepo.CreateReel(ctx, shortcode, reelURL, movie.ID)
}

func (s *BotService) handleUserMovieRequest(ctx context.Context, msg *model.Message, lang string) {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	instaURL := normalizeInstagramURL(msg.Text)

	isSubbed := s.checkChannelsMembership(ctx, userID)
	if isSubbed {
		_ = s.redisRepo.SetSubscriptionCache(ctx, userID, true)
	} else {
		_ = s.redisRepo.DeleteSubscriptionCache(ctx, userID)
	}

	if !isSubbed {
		_ = s.sendSubscriptionPrompt(ctx, chatID, lang, T(lang, "force_sub"))
		return
	}

	shortcode := repository.ExtractShortcode("https://" + instaURL)
	movie, err := s.pgRepo.GetMovieByShortcode(ctx, shortcode)

	if err != nil {
		err := s.tgClient.SendMessage(ctx, chatID, T(lang, "movie_not_found"))
		if err != nil {
			return
		}
		return
	}

	if strings.Contains(movie.TelegramFileID, "instagram.com") || strings.HasPrefix(movie.TelegramFileID, "http://") || strings.HasPrefix(movie.TelegramFileID, "https://") {
		log.Printf("[SECURITY ERROR] movie %s has invalid telegram_file_id value", movie.MovieCode)
		_ = s.tgClient.SendMessage(ctx, chatID, T(lang, "movie_delivery_error"))
		return
	}

	err = s.tgClient.SendVideo(ctx, chatID, movie.TelegramFileID, formatMovieCaption(movie))

	if err != nil {
		log.Printf("[TG SEND VIDEO ERROR] failed to send movie %s to chat %d: %v", movie.MovieCode, chatID, err)
		_ = s.tgClient.SendMessage(ctx, chatID, T(lang, "movie_delivery_error"))
		return
	}
	_ = s.pgRepo.IncrementMovieRequest(ctx, movie.ID)
}

func (s *BotService) sendMovieByCode(ctx context.Context, userID int64, chatID int64, code string, lang string) {
	isSubbed := s.checkChannelsMembership(ctx, userID)
	if isSubbed {
		_ = s.redisRepo.SetSubscriptionCache(ctx, userID, true)
	} else {
		_ = s.redisRepo.DeleteSubscriptionCache(ctx, userID)
	}
	if !isSubbed {
		_ = s.sendSubscriptionPrompt(ctx, chatID, lang, T(lang, "force_sub"))
		return
	}

	movie, err := s.pgRepo.GetMovieByCode(ctx, code)
	if err != nil {
		_ = s.tgClient.SendMessage(ctx, chatID, T(lang, "movie_not_found"))
		return
	}
	if strings.Contains(movie.TelegramFileID, "instagram.com") || strings.HasPrefix(movie.TelegramFileID, "http://") || strings.HasPrefix(movie.TelegramFileID, "https://") {
		log.Printf("[SECURITY ERROR] movie %s has invalid telegram_file_id value", movie.MovieCode)
		_ = s.tgClient.SendMessage(ctx, chatID, T(lang, "movie_delivery_error"))
		return
	}
	if err := s.tgClient.SendVideo(ctx, chatID, movie.TelegramFileID, formatMovieCaption(movie)); err != nil {
		log.Printf("[TG SEND VIDEO ERROR] failed to send movie %s to chat %d: %v", movie.MovieCode, chatID, err)
		_ = s.tgClient.SendMessage(ctx, chatID, T(lang, "movie_delivery_error"))
		return
	}
	_ = s.pgRepo.IncrementMovieRequest(ctx, movie.ID)
}

func (s *BotService) sendTopMovies(ctx context.Context, chatID int64, lang string) {
	movies, err := s.pgRepo.GetTopMovies(ctx, 10)
	if err != nil || len(movies) == 0 {
		_ = s.tgClient.SendMessage(ctx, chatID, T(lang, "top_movies_empty"))
		return
	}

	var sb strings.Builder
	sb.WriteString(T(lang, "top_movies_title"))
	sb.WriteString("\n\n")
	for i, movie := range movies {
		sb.WriteString(fmt.Sprintf("%d. %s — %s (%d)\n", i+1, movie.Title, movie.MovieCode, movie.RequestCount))
	}
	sb.WriteString("\n")
	sb.WriteString(T(lang, "top_movies_hint"))
	_ = s.tgClient.SendMessage(ctx, chatID, sb.String())
}

func formatMovieCaption(movie *model.Movie) string {
	var lines []string
	if strings.TrimSpace(movie.Title) != "" {
		lines = append(lines, "🎬 "+movie.Title)
	}
	lines = append(lines, "🔑 Kod: "+movie.MovieCode)
	if strings.TrimSpace(movie.Rating) != "" {
		lines = append(lines, "⭐ Reyting: "+movie.Rating)
	}
	if strings.TrimSpace(movie.Language) != "" {
		lines = append(lines, "🌐 Til: "+movie.Language)
	}
	if strings.TrimSpace(movie.Caption) != "" {
		lines = append(lines, "", movie.Caption)
	}
	return strings.Join(lines, "\n")
}

func isMovieCode(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for _, r := range text {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (s *BotService) checkChannelsMembership(ctx context.Context, userID int64) bool {
	channels, err := s.pgRepo.GetChannels(ctx)
	if err != nil {
		log.Printf("[CHECK-SUB ERROR] failed to load channels for user %d: %v", userID, err)
		return false
	}

	if len(channels) == 0 {
		return true
	}

	for _, ch := range channels {
		chID := ch.TelegramChannelID

		for _, candidateID := range membershipCheckChannelIDs(chID) {
			log.Printf("[DEBUG] Checking channel ID: %d for user %d", candidateID, userID)

			subbed, checkErr := s.tgClient.GetChatMember(ctx, candidateID, userID)
			if checkErr != nil {
				log.Printf("[CHECK-SUB ERROR] Channel: %d, User: %d, Cause: %v", candidateID, userID, checkErr)
				continue
			}

			if !subbed {
				return false
			}

			goto nextChannel
		}
		return false

	nextChannel:
	}
	return true
}

func (s *BotService) sendSubscriptionPrompt(ctx context.Context, chatID int64, lang string, text string) error {
	channels, err := s.pgRepo.GetChannels(ctx)
	if err != nil {
		log.Printf("[CHECK-SUB ERROR] failed to load channels for prompt: %v", err)
		return s.tgClient.SendMessage(ctx, chatID, T(lang, "subscription_check_error"))
	}

	var buttons [][]model.InlineButton
	for _, ch := range channels {
		buttons = append(buttons, []model.InlineButton{{
			Text: "📢 Kanalga o'tish",
			URL:  ch.InviteLink,
		}})
	}

	buttons = append(buttons, []model.InlineButton{{
		Text: T(lang, "check_sub_btn"),
		Data: "check_sub",
	}})

	if len(channels) == 0 {
		return s.tgClient.SendMessage(ctx, chatID, T(lang, "send_link"))
	}

	if err := s.tgClient.SendInlineKeyboard(ctx, chatID, text, buttons); err != nil {
		log.Printf("[TG SEND ERROR] force_sub keyboard failed for chat %d: %v", chatID, err)
		return err
	}

	return nil
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

func membershipCheckChannelIDs(channelID int64) []int64 {
	if channelID <= 0 || strings.HasPrefix(strconv.FormatInt(channelID, 10), "-100") {
		return []int64{channelID}
	}

	fallbackID, err := strconv.ParseInt("-100"+strconv.FormatInt(channelID, 10), 10, 64)
	if err != nil {
		return []int64{channelID}
	}

	return []int64{channelID, fallbackID}
}

func (s *BotService) isAdmin(userID int64) bool {
	return s.cfg.IsAdmin(userID)
}

func (s *BotService) frontendURL() string {
	if strings.TrimSpace(s.cfg.FrontendURL) != "" {
		return s.cfg.FrontendURL
	}
	return s.cfg.WebhookURL
}

func (s *BotService) PushUpdate(u model.Update) {
	select {
	case s.updateQueue <- u:
	default:
		log.Printf("[WARN] Core overflow: Update queue is full. Frame dropped.")
	}
}
