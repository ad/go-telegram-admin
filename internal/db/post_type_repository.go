package db

import (
	"database/sql"

	"github.com/ad/go-telegram-admin/internal/models"
)

type PostTypeRepository struct {
	queue *DBQueue
}

func NewPostTypeRepository(queue *DBQueue) *PostTypeRepository {
	return &PostTypeRepository{queue: queue}
}

func (r *PostTypeRepository) Create(postType *models.PostType) error {
	result, err := r.queue.Execute(func(db *sql.DB) (interface{}, error) {
		res, err := db.Exec(`
			INSERT INTO post_types (name, emoji, photo_id, template, template_entities, is_active)
			VALUES (?, ?, ?, ?, ?, ?)
		`, postType.Name, postType.Emoji, postType.PhotoID, postType.Template, postType.TemplateEntities, postType.IsActive)
		if err != nil {
			return nil, err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, err
		}
		return id, nil
	})
	if err != nil {
		return err
	}
	postType.ID = result.(int64)
	return nil
}

func (r *PostTypeRepository) GetByID(id int64) (*models.PostType, error) {
	row := r.queue.DB().QueryRow(`
		SELECT id, name, COALESCE(emoji, ''), photo_id, template, COALESCE(template_entities, ''), is_active, created_at
		FROM post_types WHERE id = ?
	`, id)

	var postType models.PostType
	err := row.Scan(
		&postType.ID,
		&postType.Name,
		&postType.Emoji,
		&postType.PhotoID,
		&postType.Template,
		&postType.TemplateEntities,
		&postType.IsActive,
		&postType.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &postType, nil
}

func (r *PostTypeRepository) GetAll() ([]*models.PostType, error) {
	rows, err := r.queue.DB().Query(`
		SELECT id, name, COALESCE(emoji, ''), photo_id, template, COALESCE(template_entities, ''), is_active, created_at
		FROM post_types
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var postTypes []*models.PostType
	for rows.Next() {
		var pt models.PostType
		if err := rows.Scan(
			&pt.ID,
			&pt.Name,
			&pt.Emoji,
			&pt.PhotoID,
			&pt.Template,
			&pt.TemplateEntities,
			&pt.IsActive,
			&pt.CreatedAt,
		); err != nil {
			return nil, err
		}
		postTypes = append(postTypes, &pt)
	}
	return postTypes, rows.Err()
}

func (r *PostTypeRepository) GetActive() ([]*models.PostType, error) {
	rows, err := r.queue.DB().Query(`
		SELECT id, name, COALESCE(emoji, ''), photo_id, template, COALESCE(template_entities, ''), is_active, created_at
		FROM post_types
		WHERE is_active = TRUE
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var postTypes []*models.PostType
	for rows.Next() {
		var pt models.PostType
		if err := rows.Scan(
			&pt.ID,
			&pt.Name,
			&pt.Emoji,
			&pt.PhotoID,
			&pt.Template,
			&pt.TemplateEntities,
			&pt.IsActive,
			&pt.CreatedAt,
		); err != nil {
			return nil, err
		}
		postTypes = append(postTypes, &pt)
	}
	return postTypes, rows.Err()
}

func (r *PostTypeRepository) Update(postType *models.PostType) error {
	_, err := r.queue.Execute(func(db *sql.DB) (interface{}, error) {
		_, err := db.Exec(`
			UPDATE post_types SET
				name = ?,
				emoji = ?,
				photo_id = ?,
				template = ?,
				template_entities = ?,
				is_active = ?
			WHERE id = ?
		`, postType.Name, postType.Emoji, postType.PhotoID, postType.Template, postType.TemplateEntities, postType.IsActive, postType.ID)
		return nil, err
	})
	return err
}

func (r *PostTypeRepository) Delete(id int64) error {
	_, err := r.queue.Execute(func(db *sql.DB) (interface{}, error) {
		_, err := db.Exec(`DELETE FROM post_types WHERE id = ?`, id)
		return nil, err
	})
	return err
}

func (r *PostTypeRepository) SetActive(id int64, active bool) error {
	_, err := r.queue.Execute(func(db *sql.DB) (interface{}, error) {
		_, err := db.Exec(`UPDATE post_types SET is_active = ? WHERE id = ?`, active, id)
		return nil, err
	})
	return err
}
