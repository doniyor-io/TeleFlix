package model

import "time"

type Update struct {
	UpdateID      int            `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	Text      string `json:"text"`
	Chat      Chat   `json:"chat"`
	From      User   `json:"from"`
	Video     *Video `json:"video"`
}

type CallbackQuery struct {
	ID              string   `json:"id"`
	From            *User    `json:"from"`
	Message         *Message `json:"message"`
	InlineMessageID string   `json:"inline_message_id"`
	ChatInstance    string   `json:"chat_instance"`
	Data            string   `json:"data"`
	GameShortName   string   `json:"game_short_name"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type DBUser struct {
	ID        int64
	Username  string
	CreatedAt time.Time
}

type Movie struct {
	ID           int       `json:"id"`
	InstagramURL string    `json:"instagram_url"`
	TGFileID     string    `json:"tg_file_id"`
	Code         string    `json:"code"`
	Caption      string    `json:"caption"`
	CreatedAt    time.Time `json:"created_at"`
}

type InlineButton struct {
	Text     string `json:"text"`
	URL      string `json:"url,omitempty"`
	Data     string `json:"callback_data,omitempty"`
	IsWebApp bool   `json:"is_web_app,omitempty"`
}

type Video struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}
