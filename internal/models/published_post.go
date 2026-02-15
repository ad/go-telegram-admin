package models

import "time"

type PublishedPost struct {
	ID         int64
	PostTypeID int64
	ChatID     int64
	TopicID    int64
	MessageID  int64
	Text       string
	PhotoID    string
	Entities   string
	CreatedAt  time.Time
}
