package model

import "time"

type Movie struct {
	ID                int64     `json:"id"`
	MovieCode         string    `json:"code"`
	TelegramFileID    string    `json:"telegram_file_id"`
	ReelURL           string    `json:"reel_url,omitempty"`
	MessageID         int64     `json:"message_id"`
	Caption           string    `json:"caption,omitempty"`
	TelegramChannelID int64     `json:"telegram_channel_id,omitempty"`
	CreatedBy         int64     `json:"created_by,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}
