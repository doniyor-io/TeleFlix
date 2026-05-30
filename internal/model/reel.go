package model

import "time"

type Reel struct {
	ID int64 `json:"id"`

	Shortcode string `json:"shortcode"`

	ReelURL string `json:"reel_url"`

	MovieID int64 `json:"movie_id"`

	CreatedAt time.Time `json:"created_at"`
}
