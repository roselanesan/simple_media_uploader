package models

import "time"

type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

type Media struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Filename  string    `json:"filename"`
	R2Key     string    `json:"r2_key"`
	MimeType  string    `json:"mime_type"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}
