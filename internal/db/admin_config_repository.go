package db

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/ad/go-telegram-admin/internal/models"
)

type AdminConfigRepository struct {
	queue *DBQueue
}

func NewAdminConfigRepository(queue *DBQueue) *AdminConfigRepository {
	return &AdminConfigRepository{queue: queue}
}

func (r *AdminConfigRepository) Get() (*models.AdminConfig, error) {
	db := r.queue.DB()
	config := &models.AdminConfig{
		AdminIDs:    []int64{},
		ForumChatID: 0,
		TopicID:     0,
	}

	var adminIDsStr string
	err := db.QueryRow(`SELECT value FROM admin_config WHERE key = ?`, "admin_ids").Scan(&adminIDsStr)
	if err == nil && adminIDsStr != "" {
		parts := strings.Split(adminIDsStr, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				if id, err := strconv.ParseInt(part, 10, 64); err == nil {
					config.AdminIDs = append(config.AdminIDs, id)
				}
			}
		}
	}

	var forumChatIDStr string
	err = db.QueryRow(`SELECT value FROM admin_config WHERE key = ?`, "forum_chat_id").Scan(&forumChatIDStr)
	if err == nil && forumChatIDStr != "" {
		if id, err := strconv.ParseInt(forumChatIDStr, 10, 64); err == nil {
			config.ForumChatID = id
		}
	}

	var topicIDStr string
	err = db.QueryRow(`SELECT value FROM admin_config WHERE key = ?`, "topic_id").Scan(&topicIDStr)
	if err == nil && topicIDStr != "" {
		if id, err := strconv.ParseInt(topicIDStr, 10, 64); err == nil {
			config.TopicID = id
		}
	}

	return config, nil
}

func (r *AdminConfigRepository) Save(config *models.AdminConfig) error {
	_, err := r.queue.Execute(func(db *sql.DB) (interface{}, error) {
		adminIDsStrs := make([]string, len(config.AdminIDs))
		for i, id := range config.AdminIDs {
			adminIDsStrs[i] = strconv.FormatInt(id, 10)
		}
		adminIDsStr := strings.Join(adminIDsStrs, ",")

		_, err := db.Exec(`
			INSERT OR REPLACE INTO admin_config (key, value) VALUES (?, ?)
		`, "admin_ids", adminIDsStr)
		if err != nil {
			return nil, err
		}

		_, err = db.Exec(`
			INSERT OR REPLACE INTO admin_config (key, value) VALUES (?, ?)
		`, "forum_chat_id", strconv.FormatInt(config.ForumChatID, 10))
		if err != nil {
			return nil, err
		}

		_, err = db.Exec(`
			INSERT OR REPLACE INTO admin_config (key, value) VALUES (?, ?)
		`, "topic_id", strconv.FormatInt(config.TopicID, 10))
		return nil, err
	})
	return err
}

func (r *AdminConfigRepository) AddAdmin(adminID int64) error {
	config, err := r.Get()
	if err != nil {
		config = &models.AdminConfig{AdminIDs: []int64{}}
	}

	for _, id := range config.AdminIDs {
		if id == adminID {
			return nil
		}
	}

	config.AdminIDs = append(config.AdminIDs, adminID)
	return r.Save(config)
}

func (r *AdminConfigRepository) RemoveAdmin(adminID int64) error {
	config, err := r.Get()
	if err != nil {
		return err
	}

	newAdminIDs := []int64{}
	for _, id := range config.AdminIDs {
		if id != adminID {
			newAdminIDs = append(newAdminIDs, id)
		}
	}

	config.AdminIDs = newAdminIDs
	return r.Save(config)
}

func (r *AdminConfigRepository) IsAdmin(userID int64) (bool, error) {
	config, err := r.Get()
	if err != nil {
		return false, err
	}

	for _, id := range config.AdminIDs {
		if id == userID {
			return true, nil
		}
	}
	return false, nil
}

func (r *AdminConfigRepository) SetForumConfig(chatID, topicID int64) error {
	config, err := r.Get()
	if err != nil {
		config = &models.AdminConfig{AdminIDs: []int64{}}
	}

	config.ForumChatID = chatID
	config.TopicID = topicID
	return r.Save(config)
}
