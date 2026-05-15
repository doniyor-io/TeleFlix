package bot

import (
	"context"
	"log"
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
	log.Printf("[INFO] %d ta Go Worker Pool muvaffaqiyatli ishga tushirildi", numWorkers)
}

func (s *BotService) worker(workerID int) {
	ctx := context.Background()
	for update := range s.updateQueue {
		if update.Message == nil {
			continue
		}
		s.handleMessage(ctx, workerID, update.Message)
	}
}

func (s *BotService) handleMessage(ctx context.Context, workerID int, msg *model.Message) {
	log.Printf("[Worker %d] ChatID: %d | Text: %s", workerID, msg.Chat.ID, msg.Text)

	if s.isAdmin(msg.From.ID) {
		if strings.HasPrefix(msg.Text, "/add_movie") {
			s.handleAdminAddMovie(ctx, msg)
			return
		}
	}

	if msg.Text == "/start" {
		s.tgClient.SendMessage(ctx, msg.Chat.ID, "Assalomu alaykum! Kinolarni olish uchun Instagram Reels linkini yuboring. 🎬")
		return
	}

	if strings.Contains(msg.Text, "instagram.com/") {
		s.handleUserMovieRequest(ctx, msg)
		return
	}

	s.tgClient.SendMessage(ctx, msg.Chat.ID, "Iltimos, faqat haqiqiy Instagram Reels linkini yuboring. 🤔")
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
		s.tgClient.SendMessage(ctx, msg.Chat.ID, "❌ Xato format!\nFormat: `/add_movie <insta_link> <file_id> <izoh>`")
		return
	}

	instaURL := parts[1]
	fileID := parts[2]
	caption := ""
	if len(parts) == 4 {
		caption = parts[3]
	}

	err := s.pgRepo.SaveMovie(ctx, instaURL, fileID, caption)
	if err != nil {
		log.Printf("[ERROR] Kinoni saqlashda xato: %v", err)
		s.tgClient.SendMessage(ctx, msg.Chat.ID, "❌ Bazaga saqlashda xatolik yuz berdi.")
		return
	}

	s.tgClient.SendMessage(ctx, msg.Chat.ID, "✅ Kino muvaffaqiyatli saqlandi/yangilandi!")
}

func (s *BotService) handleUserMovieRequest(ctx context.Context, msg *model.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	instaURL := msg.Text

	isSubbed, err := s.redisRepo.GetSubscriptionCache(ctx, userID)
	if err != nil {
		isSubbed = s.checkChannelsMembership(ctx, userID)
		s.redisRepo.SetSubscriptionCache(ctx, userID, isSubbed)
	}

	if !isSubbed {
		channels, _ := s.pgRepo.GetActiveChannels(ctx)
		text := "🚨 Botdan foydalanish uchun homiy kanallarga a'zo bo'lishingiz shart:\n\n"
		for i, ch := range channels {
			text += string(rune(i+49)) + ") " + ch["link"].(string) + "\n"
		}
		text += "\nObuna bo'lib, qaytadan linkni yuboring! 🔄"
		s.tgClient.SendMessage(ctx, chatID, text)
		return
	}

	fileID, caption, err := s.pgRepo.GetMovieByInstagramURL(ctx, instaURL)
	if err != nil {
		s.tgClient.SendMessage(ctx, chatID, "😔 Kechirasiz, bu link bo'yicha hali kino yuklanmagan.")
		return
	}

	err = s.tgClient.SendVideo(ctx, chatID, fileID, caption)
	if err != nil {
		log.Printf("[ERROR] Videoni userga yuborishda xato: %v", err)
		s.tgClient.SendMessage(ctx, chatID, "❌ Videoni yuborishda xatolik yuz berdi.")
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
	s.updateQueue <- u
}
