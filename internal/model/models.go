package model

import "time"

type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	Text      string `json:"text"`
	Chat      Chat   `json:"chat"`
	From      User   `json:"from"`
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
	ID           int    `gorm:"primary_key"`
	InstagramURL string `gorm:"not null"`
	TGFileID     string
	Caption      string
	CreatedAt    time.Time
}
