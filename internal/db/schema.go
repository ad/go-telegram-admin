package db

import (
	"database/sql"
	"log"
	"os"
	"strings"
)

const schema = `
CREATE TABLE IF NOT EXISTS admin_state (
    user_id INTEGER PRIMARY KEY,
    current_state TEXT NOT NULL DEFAULT '',
    selected_type_id INTEGER DEFAULT 0,
    draft_text TEXT DEFAULT '',
    draft_photo_id TEXT DEFAULT '',
    draft_entities TEXT DEFAULT '',
    editing_post_id INTEGER DEFAULT 0,
    editing_type_id INTEGER DEFAULT 0,
    temp_name TEXT DEFAULT '',
    temp_emoji TEXT DEFAULT '',
    temp_photo_id TEXT DEFAULT '',
    temp_template TEXT DEFAULT '',
    last_bot_message_id INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS post_types (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    emoji TEXT DEFAULT '',
    photo_id TEXT DEFAULT '',
    template TEXT NOT NULL,
    template_entities TEXT DEFAULT '',
    is_active BOOLEAN DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS published_posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    post_type_id INTEGER NOT NULL REFERENCES post_types(id),
    chat_id INTEGER NOT NULL,
    topic_id INTEGER NOT NULL,
    message_id INTEGER NOT NULL,
    text TEXT NOT NULL,
    photo_id TEXT DEFAULT '',
    entities TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(chat_id, message_id)
);

CREATE TABLE IF NOT EXISTS admin_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS replies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id INTEGER NOT NULL,
    reply_to_message_id INTEGER NOT NULL,
    message_id INTEGER NOT NULL,
    text TEXT NOT NULL DEFAULT '',
    photo_id TEXT DEFAULT '',
    entities TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(chat_id, message_id)
);

CREATE INDEX IF NOT EXISTS idx_published_posts_message ON published_posts(chat_id, message_id);
CREATE INDEX IF NOT EXISTS idx_post_types_active ON post_types(is_active);
CREATE INDEX IF NOT EXISTS idx_replies_message ON replies(chat_id, message_id);
`

const migrations = `
ALTER TABLE admin_state ADD COLUMN reply_target_chat_id INTEGER DEFAULT 0;
ALTER TABLE admin_state ADD COLUMN reply_target_message_id INTEGER DEFAULT 0;
ALTER TABLE published_posts ADD COLUMN user_photo_id TEXT DEFAULT '';
ALTER TABLE published_posts ADD COLUMN user_photo_message_id INTEGER DEFAULT 0;
ALTER TABLE admin_state ADD COLUMN draft_user_photo_id TEXT DEFAULT ''
`

func InitSchema(db *sql.DB) error {
	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	migrationStatements := strings.Split(migrations, ";")
	for i, stmt := range migrationStatements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			// log.Printf("Migration %d failed: %s. Error: %v", i, stmt, err)
		} else {
			log.Printf("Migration %d executed: %s", i, stmt)
		}
	}

	if err := InitializeAdminConfig(db); err != nil {
		log.Printf("Failed to initialize admin config: %v", err)
		return err
	}

	return nil
}

func InitializeAdminConfig(db *sql.DB) error {
	adminIDs := strings.TrimSpace(getEnv("ADMIN_IDS", ""))
	forumChatID := strings.TrimSpace(getEnv("FORUM_CHAT_ID", "0"))
	topicID := strings.TrimSpace(getEnv("TOPIC_ID", "0"))

	if adminIDs != "" {
		_, err := db.Exec("INSERT OR IGNORE INTO admin_config (key, value) VALUES (?, ?)", "admin_ids", adminIDs)
		if err != nil {
			return err
		}
	}

	if forumChatID != "0" {
		_, err := db.Exec("INSERT OR IGNORE INTO admin_config (key, value) VALUES (?, ?)", "forum_chat_id", forumChatID)
		if err != nil {
			return err
		}
	}

	if topicID != "0" {
		_, err := db.Exec("INSERT OR IGNORE INTO admin_config (key, value) VALUES (?, ?)", "topic_id", topicID)
		if err != nil {
			return err
		}
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
