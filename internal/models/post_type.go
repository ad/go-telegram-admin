package models

import "time"

type PostType struct {
	ID               int64
	Name             string
	Emoji            string
	PhotoID          string
	Template         string
	TemplateEntities string
	IsActive         bool
	CreatedAt        time.Time
}
