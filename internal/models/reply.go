package models

import "time"

type Reply struct {
	ID               int64
	ChatID           int64
	ReplyToMessageID int64
	MessageID        int64
	Text             string
	PhotoID          string
	Entities         string
	CreatedAt        time.Time
}
