package db

import (
	"database/sql"

	"github.com/ad/go-telegram-admin/internal/models"
)

type ReplyRepository struct {
	queue *DBQueue
}

func NewReplyRepository(queue *DBQueue) *ReplyRepository {
	return &ReplyRepository{queue: queue}
}

func (r *ReplyRepository) Create(reply *models.Reply) error {
	result, err := r.queue.Execute(func(db *sql.DB) (interface{}, error) {
		res, err := db.Exec(`
			INSERT INTO replies (chat_id, reply_to_message_id, message_id, text, photo_id, entities)
			VALUES (?, ?, ?, ?, ?, ?)
		`, reply.ChatID, reply.ReplyToMessageID, reply.MessageID, reply.Text, reply.PhotoID, reply.Entities)
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
	reply.ID = result.(int64)
	return nil
}

func (r *ReplyRepository) GetByID(id int64) (*models.Reply, error) {
	row := r.queue.DB().QueryRow(`
		SELECT id, chat_id, reply_to_message_id, message_id, text, COALESCE(photo_id, ''), COALESCE(entities, ''), created_at
		FROM replies WHERE id = ?
	`, id)

	var reply models.Reply
	err := row.Scan(
		&reply.ID,
		&reply.ChatID,
		&reply.ReplyToMessageID,
		&reply.MessageID,
		&reply.Text,
		&reply.PhotoID,
		&reply.Entities,
		&reply.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &reply, nil
}

func (r *ReplyRepository) GetAll() ([]*models.Reply, error) {
	rows, err := r.queue.DB().Query(`
		SELECT id, chat_id, reply_to_message_id, message_id, text, COALESCE(photo_id, ''), COALESCE(entities, ''), created_at
		FROM replies
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var replies []*models.Reply
	for rows.Next() {
		var reply models.Reply
		if err := rows.Scan(
			&reply.ID,
			&reply.ChatID,
			&reply.ReplyToMessageID,
			&reply.MessageID,
			&reply.Text,
			&reply.PhotoID,
			&reply.Entities,
			&reply.CreatedAt,
		); err != nil {
			return nil, err
		}
		replies = append(replies, &reply)
	}
	return replies, rows.Err()
}

func (r *ReplyRepository) Count() (int64, error) {
	var count int64
	err := r.queue.DB().QueryRow(`SELECT COUNT(*) FROM replies`).Scan(&count)
	return count, err
}

func (r *ReplyRepository) GetPaginated(limit, offset int64) ([]*models.Reply, error) {
	rows, err := r.queue.DB().Query(`
		SELECT id, chat_id, reply_to_message_id, message_id, text, COALESCE(photo_id, ''), COALESCE(entities, ''), created_at
		FROM replies
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var replies []*models.Reply
	for rows.Next() {
		var reply models.Reply
		if err := rows.Scan(
			&reply.ID,
			&reply.ChatID,
			&reply.ReplyToMessageID,
			&reply.MessageID,
			&reply.Text,
			&reply.PhotoID,
			&reply.Entities,
			&reply.CreatedAt,
		); err != nil {
			return nil, err
		}
		replies = append(replies, &reply)
	}
	return replies, rows.Err()
}

func (r *ReplyRepository) Update(reply *models.Reply) error {
	_, err := r.queue.Execute(func(db *sql.DB) (interface{}, error) {
		_, err := db.Exec(`
			UPDATE replies SET
				text = ?,
				photo_id = ?,
				entities = ?
			WHERE id = ?
		`, reply.Text, reply.PhotoID, reply.Entities, reply.ID)
		return nil, err
	})
	return err
}

func (r *ReplyRepository) Delete(id int64) error {
	_, err := r.queue.Execute(func(db *sql.DB) (interface{}, error) {
		_, err := db.Exec(`DELETE FROM replies WHERE id = ?`, id)
		return nil, err
	})
	return err
}
