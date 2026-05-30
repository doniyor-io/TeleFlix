package model

import "time"

type DBUser struct {
	ID int64 `json:"id"`

	Username string `json:"username,omitempty"`

	PhoneNumber string `json:"phone_number,omitempty"`

	FirstName string `json:"first_name,omitempty"`

	LastName string `json:"last_name,omitempty"`

	LanguageCode string `json:"language_code"`

	CreatedAt time.Time `json:"created_at"`
}
