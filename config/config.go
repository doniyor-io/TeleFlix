package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Port string
	Env  string

	TelegramBotToken string

	AdminIDs []int64

	DatabaseURL string
	RedisURL    string

	WebhookURL  string
	FrontendURL string

	BotName string

	BridgeSecret string

	InstagramAccessToken   string
	InstagramBusinessID    string
	MetaWebhookVerifyToken string
	MetaWebhookSecret      string
}

func Load() (*Config, error) {
	viper.SetConfigFile(".env")

	viper.AutomaticEnv()

	_ = viper.ReadInConfig()

	cfg := &Config{
		Port:                   viper.GetString("PORT"),
		Env:                    viper.GetString("ENV"),
		TelegramBotToken:       viper.GetString("TELEGRAM_BOT_TOKEN"),
		DatabaseURL:            viper.GetString("DATABASE_URL"),
		RedisURL:               viper.GetString("REDIS_URL"),
		WebhookURL:             viper.GetString("WEBHOOK_URL"),
		FrontendURL:            viper.GetString("FRONTEND_URL"),
		BotName:                viper.GetString("BOT_NAME"),
		BridgeSecret:           viper.GetString("BRIDGE_SECRET"),
		InstagramAccessToken:   viper.GetString("INSTAGRAM_ACCESS_TOKEN"),
		InstagramBusinessID:    viper.GetString("INSTAGRAM_BUSINESS_ID"),
		MetaWebhookVerifyToken: viper.GetString("META_WEBHOOK_VERIFY_TOKEN"),
		MetaWebhookSecret:      viper.GetString("META_WEBHOOK_SECRET"),
	}

	admins := viper.GetString("ADMIN_IDS")

	if admins != "" {
		for _, raw := range strings.Split(admins, ",") {
			raw = strings.TrimSpace(raw)

			var id int64

			_, err := fmt.Sscan(raw, &id)
			if err == nil {
				cfg.AdminIDs = append(cfg.AdminIDs, id)
			}
		}
	}

	if cfg.Port == "" {
		cfg.Port = "9090"
	}

	return cfg, nil
}

func (c *Config) IsAdmin(userID int64) bool {
	for _, id := range c.AdminIDs {
		if id == userID {
			return true
		}
	}

	return false
}
