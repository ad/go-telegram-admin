package services

import (
	"fmt"

	"github.com/ad/go-telegram-admin/internal/db"
	"github.com/ad/go-telegram-admin/internal/models"
)

type PostTypeManager struct {
	repo *db.PostTypeRepository
}

func NewPostTypeManager(repo *db.PostTypeRepository) *PostTypeManager {
	return &PostTypeManager{repo: repo}
}

func (ptm *PostTypeManager) CreateType(name, photoID, template string) (*models.PostType, error) {
	postType := &models.PostType{
		Name:     name,
		PhotoID:  photoID,
		Template: template,
		IsActive: true,
	}

	if err := ptm.repo.Create(postType); err != nil {
		return nil, fmt.Errorf("failed to create post type: %w", err)
	}

	return postType, nil
}

func (ptm *PostTypeManager) GetType(id int64) (*models.PostType, error) {
	return ptm.repo.GetByID(id)
}

func (ptm *PostTypeManager) GetAllTypes() ([]*models.PostType, error) {
	return ptm.repo.GetAll()
}

func (ptm *PostTypeManager) GetActiveTypes() ([]*models.PostType, error) {
	return ptm.repo.GetActive()
}

func (ptm *PostTypeManager) UpdateTypeName(id int64, name string) error {
	postType, err := ptm.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("failed to get post type: %w", err)
	}

	postType.Name = name
	return ptm.repo.Update(postType)
}

func (ptm *PostTypeManager) UpdateTypeEmoji(id int64, emoji string) error {
	postType, err := ptm.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("failed to get post type: %w", err)
	}

	postType.Emoji = emoji
	return ptm.repo.Update(postType)
}

func (ptm *PostTypeManager) UpdateTypePhoto(id int64, photoID string) error {
	postType, err := ptm.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("failed to get post type: %w", err)
	}

	postType.PhotoID = photoID
	return ptm.repo.Update(postType)
}

func (ptm *PostTypeManager) UpdateTypeTemplate(id int64, template string) error {
	postType, err := ptm.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("failed to get post type: %w", err)
	}

	postType.Template = template
	return ptm.repo.Update(postType)
}

func (ptm *PostTypeManager) SetTypeActive(id int64, active bool) error {
	return ptm.repo.SetActive(id, active)
}

func (ptm *PostTypeManager) DeleteType(id int64) error {
	return ptm.repo.Delete(id)
}
