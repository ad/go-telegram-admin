package db

import (
	"database/sql"

	"github.com/ad/go-telegram-admin/internal/models"
)

type PublishedPostRepository struct {
	queue *DBQueue
}

func NewPublishedPostRepository(queue *DBQueue) *PublishedPostRepository {
	return &PublishedPostRepository{queue: queue}
}

func (r *PublishedPostRepository) Create(post *models.PublishedPost) error {
	result, err := r.queue.Execute(func(db *sql.DB) (interface{}, error) {
		res, err := db.Exec(`
			INSERT INTO published_posts (post_type_id, chat_id, topic_id, message_id, text, photo_id, entities, user_photo_id, user_photo_message_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, post.PostTypeID, post.ChatID, post.TopicID, post.MessageID, post.Text, post.PhotoID, post.Entities, post.UserPhotoID, post.UserPhotoMessageID)
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
	post.ID = result.(int64)
	return nil
}

func (r *PublishedPostRepository) GetByID(id int64) (*models.PublishedPost, error) {
	row := r.queue.DB().QueryRow(`
		SELECT id, post_type_id, chat_id, topic_id, message_id, text, photo_id, COALESCE(entities, ''), COALESCE(user_photo_id, ''), COALESCE(user_photo_message_id, 0), created_at
		FROM published_posts WHERE id = ?
	`, id)

	var post models.PublishedPost
	err := row.Scan(
		&post.ID,
		&post.PostTypeID,
		&post.ChatID,
		&post.TopicID,
		&post.MessageID,
		&post.Text,
		&post.PhotoID,
		&post.Entities,
		&post.UserPhotoID,
		&post.UserPhotoMessageID,
		&post.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *PublishedPostRepository) GetByMessageID(chatID, messageID int64) (*models.PublishedPost, error) {
	row := r.queue.DB().QueryRow(`
		SELECT id, post_type_id, chat_id, topic_id, message_id, text, photo_id, COALESCE(entities, ''), COALESCE(user_photo_id, ''), COALESCE(user_photo_message_id, 0), created_at
		FROM published_posts WHERE chat_id = ? AND message_id = ?
	`, chatID, messageID)

	var post models.PublishedPost
	err := row.Scan(
		&post.ID,
		&post.PostTypeID,
		&post.ChatID,
		&post.TopicID,
		&post.MessageID,
		&post.Text,
		&post.PhotoID,
		&post.Entities,
		&post.UserPhotoID,
		&post.UserPhotoMessageID,
		&post.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *PublishedPostRepository) GetAll() ([]*models.PublishedPost, error) {
	rows, err := r.queue.DB().Query(`
		SELECT id, post_type_id, chat_id, topic_id, message_id, text, photo_id, COALESCE(entities, ''), COALESCE(user_photo_id, ''), COALESCE(user_photo_message_id, 0), created_at
		FROM published_posts
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*models.PublishedPost
	for rows.Next() {
		var post models.PublishedPost
		if err := rows.Scan(
			&post.ID,
			&post.PostTypeID,
			&post.ChatID,
			&post.TopicID,
			&post.MessageID,
			&post.Text,
			&post.PhotoID,
			&post.Entities,
			&post.UserPhotoID,
			&post.UserPhotoMessageID,
			&post.CreatedAt,
		); err != nil {
			return nil, err
		}
		posts = append(posts, &post)
	}
	return posts, rows.Err()
}

func (r *PublishedPostRepository) Update(post *models.PublishedPost) error {
	_, err := r.queue.Execute(func(db *sql.DB) (interface{}, error) {
		_, err := db.Exec(`
			UPDATE published_posts SET
				post_type_id = ?,
				chat_id = ?,
				topic_id = ?,
				message_id = ?,
				text = ?,
				photo_id = ?,
				entities = ?,
				user_photo_id = ?,
				user_photo_message_id = ?
			WHERE id = ?
		`, post.PostTypeID, post.ChatID, post.TopicID, post.MessageID, post.Text, post.PhotoID, post.Entities, post.UserPhotoID, post.UserPhotoMessageID, post.ID)
		return nil, err
	})
	return err
}

func (r *PublishedPostRepository) Delete(id int64) error {
	_, err := r.queue.Execute(func(db *sql.DB) (interface{}, error) {
		_, err := db.Exec(`DELETE FROM published_posts WHERE id = ?`, id)
		return nil, err
	})
	return err
}

func (r *PublishedPostRepository) Count() (int64, error) {
	var count int64
	err := r.queue.DB().QueryRow(`SELECT COUNT(*) FROM published_posts`).Scan(&count)
	return count, err
}

func (r *PublishedPostRepository) GetPaginated(limit, offset int64) ([]*models.PublishedPost, error) {
	rows, err := r.queue.DB().Query(`
		SELECT id, post_type_id, chat_id, topic_id, message_id, text, photo_id, COALESCE(entities, ''), COALESCE(user_photo_id, ''), COALESCE(user_photo_message_id, 0), created_at
		FROM published_posts
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*models.PublishedPost
	for rows.Next() {
		var post models.PublishedPost
		if err := rows.Scan(
			&post.ID,
			&post.PostTypeID,
			&post.ChatID,
			&post.TopicID,
			&post.MessageID,
			&post.Text,
			&post.PhotoID,
			&post.Entities,
			&post.UserPhotoID,
			&post.UserPhotoMessageID,
			&post.CreatedAt,
		); err != nil {
			return nil, err
		}
		posts = append(posts, &post)
	}
	return posts, rows.Err()
}
