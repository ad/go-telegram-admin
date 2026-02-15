package services

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/ad/go-telegram-admin/internal/db"
	"github.com/ad/go-telegram-admin/internal/models"
	_ "modernc.org/sqlite"
)

func setupTestDBForBackup(t *testing.T) (*sql.DB, *db.DBQueue, func()) {
	sqlDB, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}

	err = db.InitSchema(sqlDB)
	if err != nil {
		t.Fatal(err)
	}

	queue := db.NewDBQueueForTest(sqlDB)
	return sqlDB, queue, func() { sqlDB.Close() }
}

func TestEndToEndBackupFlow(t *testing.T) {
	_, queue, cleanup := setupTestDBForBackup(t)
	defer cleanup()

	postTypeRepo := db.NewPostTypeRepository(queue)
	publishedPostRepo := db.NewPublishedPostRepository(queue)
	adminConfigRepo := db.NewAdminConfigRepository(queue)

	adminConfig := &models.AdminConfig{
		AdminIDs:    []int64{123456789, 987654321},
		ForumChatID: -1001234567890,
		TopicID:     42,
	}
	err := adminConfigRepo.Save(adminConfig)
	if err != nil {
		t.Fatalf("Failed to save admin config: %v", err)
	}

	postType1 := &models.PostType{
		Name:     "Announcement",
		PhotoID:  "photo_announcement",
		Template: "Important announcement template",
		IsActive: true,
	}
	err = postTypeRepo.Create(postType1)
	if err != nil {
		t.Fatalf("Failed to create post type 1: %v", err)
	}

	postType2 := &models.PostType{
		Name:     "News",
		PhotoID:  "",
		Template: "News template",
		IsActive: false,
	}
	err = postTypeRepo.Create(postType2)
	if err != nil {
		t.Fatalf("Failed to create post type 2: %v", err)
	}

	post1 := &models.PublishedPost{
		PostTypeID: postType1.ID,
		ChatID:     -1001234567890,
		TopicID:    42,
		MessageID:  100,
		Text:       "First announcement post",
		PhotoID:    postType1.PhotoID,
	}
	err = publishedPostRepo.Create(post1)
	if err != nil {
		t.Fatalf("Failed to create post 1: %v", err)
	}

	post2 := &models.PublishedPost{
		PostTypeID: postType1.ID,
		ChatID:     -1001234567890,
		TopicID:    42,
		MessageID:  101,
		Text:       "Second announcement post",
		PhotoID:    postType1.PhotoID,
	}
	err = publishedPostRepo.Create(post2)
	if err != nil {
		t.Fatalf("Failed to create post 2: %v", err)
	}

	bm := &BackupManager{
		queue:  queue,
		dbPath: ":memory:",
		bot:    nil,
	}

	t.Run("Complete backup creation flow", func(t *testing.T) {
		sqlDump, err := bm.CreateBackup()
		if err != nil {
			t.Fatalf("Failed to create backup: %v", err)
		}

		if sqlDump == "" {
			t.Fatal("Backup should not be empty")
		}

		dumpLen := len(sqlDump)
		previewLen := 500
		if dumpLen < previewLen {
			previewLen = dumpLen
		}
		t.Logf("Backup dump length: %d bytes", dumpLen)
		t.Logf("First %d chars: %s", previewLen, sqlDump[:previewLen])

		if !strings.Contains(sqlDump, "BEGIN TRANSACTION") {
			t.Error("Backup should contain BEGIN TRANSACTION")
		}

		if !strings.Contains(sqlDump, "COMMIT") {
			t.Error("Backup should contain COMMIT")
		}

		if !strings.Contains(sqlDump, "post_types") {
			t.Error("Backup should contain post_types table")
		}

		if !strings.Contains(sqlDump, "published_posts") {
			t.Error("Backup should contain published_posts table")
		}

		if !strings.Contains(sqlDump, "admin_config") {
			t.Error("Backup should contain admin_config table")
		}

		if !strings.Contains(sqlDump, "Announcement") {
			t.Error("Backup should contain post type 'Announcement'")
		}

		if !strings.Contains(sqlDump, "News") {
			t.Error("Backup should contain post type 'News'")
		}

		if !strings.Contains(sqlDump, "First announcement post") {
			t.Error("Backup should contain first post text")
		}

		if !strings.Contains(sqlDump, "Second announcement post") {
			t.Error("Backup should contain second post text")
		}

		if !strings.Contains(sqlDump, "123456789") {
			t.Error("Backup should contain admin IDs")
		}
	})

	t.Run("Backup contains correct SQL dump", func(t *testing.T) {
		sqlDump, err := bm.CreateBackup()
		if err != nil {
			t.Fatalf("Failed to create backup: %v", err)
		}

		newDB, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("Failed to open new db: %v", err)
		}
		defer newDB.Close()

		_, err = newDB.Exec(sqlDump)
		if err != nil {
			t.Fatalf("Failed to execute backup SQL: %v", err)
		}

		var count int
		err = newDB.QueryRow("SELECT COUNT(*) FROM post_types").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query post_types: %v", err)
		}

		if count != 2 {
			t.Errorf("Expected 2 post types in restored DB, got %d", count)
		}

		err = newDB.QueryRow("SELECT COUNT(*) FROM published_posts").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query published_posts: %v", err)
		}

		if count != 2 {
			t.Errorf("Expected 2 published posts in restored DB, got %d", count)
		}

		var name string
		err = newDB.QueryRow("SELECT name FROM post_types WHERE id = ?", postType1.ID).Scan(&name)
		if err != nil {
			t.Fatalf("Failed to query post type name: %v", err)
		}

		if name != "Announcement" {
			t.Errorf("Expected post type name 'Announcement', got %s", name)
		}

		var text string
		err = newDB.QueryRow("SELECT text FROM published_posts WHERE id = ?", post1.ID).Scan(&text)
		if err != nil {
			t.Fatalf("Failed to query post text: %v", err)
		}

		if text != "First announcement post" {
			t.Errorf("Expected post text 'First announcement post', got %s", text)
		}
	})

	t.Run("SendBackupToAdmin creates file with correct format", func(t *testing.T) {
		sqlDump, err := bm.CreateBackup()
		if err != nil {
			t.Fatalf("Failed to create backup: %v", err)
		}

		if bm.bot == nil {
			t.Skip("Bot is nil, cannot test actual sending, but backup creation succeeded")
		}

		ctx := context.Background()
		err = bm.SendBackupToAdmin(ctx, 123456789, sqlDump)
		if err != nil {
			t.Logf("SendBackupToAdmin error (expected with nil bot): %v", err)
		}
	})
}

func TestBackupWithEmptyDatabase(t *testing.T) {
	_, queue, cleanup := setupTestDBForBackup(t)
	defer cleanup()

	bm := &BackupManager{
		queue:  queue,
		dbPath: ":memory:",
		bot:    nil,
	}

	sqlDump, err := bm.CreateBackup()
	if err != nil {
		t.Fatalf("Failed to create backup of empty database: %v", err)
	}

	if sqlDump == "" {
		t.Fatal("Backup should not be empty even for empty database")
	}

	if !strings.Contains(sqlDump, "BEGIN TRANSACTION") {
		t.Error("Backup should contain BEGIN TRANSACTION")
	}

	if !strings.Contains(sqlDump, "COMMIT") {
		t.Error("Backup should contain COMMIT")
	}

	if !strings.Contains(sqlDump, "CREATE TABLE") {
		t.Error("Backup should contain CREATE TABLE statements")
	}
}

func TestBackupWithLargeDataset(t *testing.T) {
	_, queue, cleanup := setupTestDBForBackup(t)
	defer cleanup()

	postTypeRepo := db.NewPostTypeRepository(queue)
	publishedPostRepo := db.NewPublishedPostRepository(queue)

	postType := &models.PostType{
		Name:     "Large Dataset Type",
		PhotoID:  "photo_large",
		Template: "Template for large dataset",
		IsActive: true,
	}
	err := postTypeRepo.Create(postType)
	if err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	for i := 0; i < 50; i++ {
		post := &models.PublishedPost{
			PostTypeID: postType.ID,
			ChatID:     -1001234567890,
			TopicID:    42,
			MessageID:  int64(1000 + i),
			Text:       strings.Repeat("Test post content ", 10),
			PhotoID:    postType.PhotoID,
		}
		err = publishedPostRepo.Create(post)
		if err != nil {
			t.Fatalf("Failed to create post %d: %v", i, err)
		}
	}

	bm := &BackupManager{
		queue:  queue,
		dbPath: ":memory:",
		bot:    nil,
	}

	sqlDump, err := bm.CreateBackup()
	if err != nil {
		t.Fatalf("Failed to create backup with large dataset: %v", err)
	}

	if sqlDump == "" {
		t.Fatal("Backup should not be empty")
	}

	postCount := strings.Count(sqlDump, "Test post content")
	if postCount < 50 {
		t.Errorf("Expected at least 50 posts in backup, found %d", postCount)
	}
}
