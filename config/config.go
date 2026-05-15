package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Port             string  `mapstructure:"PORT"`
	Env              string  `mapstructure:"ENV"`
	TelegramBotToken string  `mapstructure:"TELEGRAM_BOT_TOKEN"`
	AdminIDs         []int64 `mapstructure:"ADMIN_IDS"`
	DatabaseURL      string  `mapstructure:"DATABASE_URL"`
	RedisURL         string  `mapstructure:"REDIS_URL"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("[WARN] .env not found, OS Env will be used")
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) IsAdmin(chatID int64) bool {
	for _, adminID := range c.AdminIDs {
		if adminID == chatID {
			return true
		}
	}
	return false
}
