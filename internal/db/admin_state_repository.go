package db

import (
	"database/sql"

	"github.com/ad/go-telegram-admin/internal/models"
)

type AdminStateRepository struct {
	queue *DBQueue
}

func NewAdminStateRepository(queue *DBQueue) *AdminStateRepository {
	return &AdminStateRepository{queue: queue}
}

func (r *AdminStateRepository) Save(state *models.AdminState) error {
	_, err := r.queue.Execute(func(db *sql.DB) (interface{}, error) {
		_, err := db.Exec(`
			INSERT INTO admin_state (user_id, current_state, selected_type_id, draft_text, draft_photo_id, draft_entities, editing_post_id, editing_type_id, temp_name, temp_emoji, temp_photo_id, temp_template, last_bot_message_id, reply_target_chat_id, reply_target_message_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(user_id) DO UPDATE SET
				current_state = excluded.current_state,
				selected_type_id = excluded.selected_type_id,
				draft_text = excluded.draft_text,
				draft_photo_id = excluded.draft_photo_id,
				draft_entities = excluded.draft_entities,
				editing_post_id = excluded.editing_post_id,
				editing_type_id = excluded.editing_type_id,
				temp_name = excluded.temp_name,
				temp_emoji = excluded.temp_emoji,
				temp_photo_id = excluded.temp_photo_id,
				temp_template = excluded.temp_template,
				last_bot_message_id = excluded.last_bot_message_id,
				reply_target_chat_id = excluded.reply_target_chat_id,
				reply_target_message_id = excluded.reply_target_message_id
		`, state.UserID, state.CurrentState, state.SelectedTypeID, state.DraftText, state.DraftPhotoID, state.DraftEntities, state.EditingPostID, state.EditingTypeID, state.TempName, state.TempEmoji, state.TempPhotoID, state.TempTemplate, state.LastBotMessageID, state.ReplyTargetChatID, state.ReplyTargetMessageID)
		return nil, err
	})
	return err
}

func (r *AdminStateRepository) Get(userID int64) (*models.AdminState, error) {
	row := r.queue.DB().QueryRow(`
		SELECT user_id, current_state, COALESCE(selected_type_id, 0), COALESCE(draft_text, ''), COALESCE(draft_photo_id, ''), COALESCE(draft_entities, ''), COALESCE(editing_post_id, 0), COALESCE(editing_type_id, 0), COALESCE(temp_name, ''), COALESCE(temp_emoji, ''), COALESCE(temp_photo_id, ''), COALESCE(temp_template, ''), COALESCE(last_bot_message_id, 0), COALESCE(reply_target_chat_id, 0), COALESCE(reply_target_message_id, 0)
		FROM admin_state WHERE user_id = ?
	`, userID)

	var state models.AdminState
	err := row.Scan(&state.UserID, &state.CurrentState, &state.SelectedTypeID, &state.DraftText, &state.DraftPhotoID, &state.DraftEntities, &state.EditingPostID, &state.EditingTypeID, &state.TempName, &state.TempEmoji, &state.TempPhotoID, &state.TempTemplate, &state.LastBotMessageID, &state.ReplyTargetChatID, &state.ReplyTargetMessageID)
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *AdminStateRepository) Clear(userID int64) error {
	_, err := r.queue.Execute(func(db *sql.DB) (interface{}, error) {
		_, err := db.Exec(`DELETE FROM admin_state WHERE user_id = ?`, userID)
		return nil, err
	})
	return err
}
