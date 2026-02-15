package services

import (
	"github.com/ad/go-telegram-admin/internal/db"
)

type AdminAuthMiddleware struct {
	configRepo *db.AdminConfigRepository
}

func NewAdminAuthMiddleware(configRepo *db.AdminConfigRepository) *AdminAuthMiddleware {
	return &AdminAuthMiddleware{
		configRepo: configRepo,
	}
}

func (m *AdminAuthMiddleware) IsAuthorized(userID int64) bool {
	isAdmin, err := m.configRepo.IsAdmin(userID)
	if err != nil {
		return false
	}
	return isAdmin
}

func (m *AdminAuthMiddleware) ShouldIgnore(userID int64) bool {
	return !m.IsAuthorized(userID)
}
