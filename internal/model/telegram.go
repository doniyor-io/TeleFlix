package model

type Update struct {
	UpdateID      int            `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

type Message struct {
	MessageID int `json:"message_id"`

	Text string `json:"text,omitempty"`

	Caption string `json:"caption,omitempty"`

	Chat Chat `json:"chat"`

	From User `json:"from"`

	Video *Video `json:"video,omitempty"`

	Contact *Contact `json:"contact,omitempty"`
}

type CallbackQuery struct {
	ID string `json:"id"`

	From *User `json:"from,omitempty"`

	Message *Message `json:"message,omitempty"`

	Data string `json:"data,omitempty"`
}

type Chat struct {
	ID int64 `json:"id"`

	Type string `json:"type"`
}

type User struct {
	ID int64 `json:"id"`

	Username string `json:"username,omitempty"`

	FirstName string `json:"first_name,omitempty"`

	LastName string `json:"last_name,omitempty"`
}

type Contact struct {
	PhoneNumber string `json:"phone_number"`

	FirstName string `json:"first_name,omitempty"`

	LastName string `json:"last_name,omitempty"`

	UserID int64 `json:"user_id,omitempty"`
}

type Video struct {
	FileID string `json:"file_id"`

	FileUniqueID string `json:"file_unique_id"`

	Duration int `json:"duration"`

	Width int `json:"width"`

	Height int `json:"height"`

	FileSize int64 `json:"file_size,omitempty"`
}

type InlineButton struct {
	Text string `json:"text"`

	URL string `json:"url,omitempty"`

	Data string `json:"callback_data,omitempty"`
}
