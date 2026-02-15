package services

import (
	"fmt"

	"github.com/ad/go-telegram-admin/internal/db"
)

type SettingsManager struct {
	configRepo *db.AdminConfigRepository
}

func NewSettingsManager(configRepo *db.AdminConfigRepository) *SettingsManager {
	return &SettingsManager{configRepo: configRepo}
}

func (sm *SettingsManager) AddAdmin(adminID int64) error {
	return sm.configRepo.AddAdmin(adminID)
}

func (sm *SettingsManager) RemoveAdmin(adminID int64) error {
	return sm.configRepo.RemoveAdmin(adminID)
}

func (sm *SettingsManager) GetAdmins() ([]int64, error) {
	config, err := sm.configRepo.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}
	return config.AdminIDs, nil
}

func (sm *SettingsManager) IsAdmin(userID int64) (bool, error) {
	return sm.configRepo.IsAdmin(userID)
}

func (sm *SettingsManager) SetForumConfig(chatID, topicID int64) error {
	return sm.configRepo.SetForumConfig(chatID, topicID)
}

func (sm *SettingsManager) GetForumConfig() (chatID, topicID int64, err error) {
	config, err := sm.configRepo.Get()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get config: %w", err)
	}
	return config.ForumChatID, config.TopicID, nil
}
