package db

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func setupSchemaTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	return db
}

func TestTableCreation(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	err := InitSchema(db)
	if err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	tables := []string{
		"post_types",
		"published_posts",
		"admin_config",
	}

	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("Table %s was not created: %v", table, err)
		}
	}
}

func TestAdminStateMigrations(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	err := InitSchema(db)
	if err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	columns := []string{
		"selected_type_id",
		"draft_text",
		"draft_photo_id",
		"editing_post_id",
		"editing_type_id",
		"temp_name",
		"temp_photo_id",
		"temp_template",
	}

	for _, column := range columns {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('admin_state') WHERE name=?", column).Scan(&count)
		if err != nil {
			t.Errorf("Failed to check column %s: %v", column, err)
		}
		if count == 0 {
			t.Errorf("Column %s was not added to admin_state table", column)
		}
	}
}

func TestAdminConfigInitialization(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	os.Setenv("ADMIN_IDS", "123456789,987654321")
	os.Setenv("FORUM_CHAT_ID", "-1001234567890")
	os.Setenv("TOPIC_ID", "42")
	defer func() {
		os.Unsetenv("ADMIN_IDS")
		os.Unsetenv("FORUM_CHAT_ID")
		os.Unsetenv("TOPIC_ID")
	}()

	err := InitSchema(db)
	if err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	tests := []struct {
		key      string
		expected string
	}{
		{"admin_ids", "123456789,987654321"},
		{"forum_chat_id", "-1001234567890"},
		{"topic_id", "42"},
	}

	for _, tt := range tests {
		var value string
		err := db.QueryRow("SELECT value FROM admin_config WHERE key=?", tt.key).Scan(&value)
		if err != nil {
			t.Errorf("Failed to retrieve %s from admin_config: %v", tt.key, err)
			continue
		}
		if value != tt.expected {
			t.Errorf("Expected %s to be %s, got %s", tt.key, tt.expected, value)
		}
	}
}

func TestAdminConfigInitializationWithoutEnvVars(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	os.Unsetenv("ADMIN_IDS")
	os.Unsetenv("FORUM_CHAT_ID")
	os.Unsetenv("TOPIC_ID")

	err := InitSchema(db)
	if err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM admin_config").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count admin_config rows: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 rows in admin_config when no env vars set, got %d", count)
	}
}

func TestIndexCreation(t *testing.T) {
	db := setupSchemaTestDB(t)
	defer db.Close()

	err := InitSchema(db)
	if err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	indexes := []string{
		"idx_published_posts_message",
		"idx_post_types_active",
	}

	for _, index := range indexes {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", index).Scan(&name)
		if err != nil {
			t.Errorf("Index %s was not created: %v", index, err)
		}
	}
}
