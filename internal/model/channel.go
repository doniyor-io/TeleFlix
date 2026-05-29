package model

import "time"

type Channel struct {
	ID int64 `json:"id"`

	TelegramChannelID int64 `json:"telegram_channel_id"`

	InviteLink string `json:"invite_link"`

	IsActive bool `json:"is_active"`

	CreatedAt time.Time `json:"created_at"`
}
