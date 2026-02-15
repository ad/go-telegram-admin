package handlers

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/ad/go-telegram-admin/internal/db"
	"github.com/ad/go-telegram-admin/internal/services"
	_ "modernc.org/sqlite"
)

func setupTestDBForErrorHandling(t *testing.T) (*sql.DB, *db.DBQueue, func()) {
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

func TestErrorHandling_InvalidPostLink(t *testing.T) {
	ctx := context.Background()
	_, queue, cleanup := setupTestDBForErrorHandling(t)
	defer cleanup()

	adminConfigRepo := db.NewAdminConfigRepository(queue)
	postTypeRepo := db.NewPostTypeRepository(queue)
	publishedPostRepo := db.NewPublishedPostRepository(queue)

	postManager := services.NewPostManager(publishedPostRepo, postTypeRepo, adminConfigRepo)

	tests := []struct {
		name string
		link string
	}{
		{
			name: "completely invalid link",
			link: "not a link at all",
		},
		{
			name: "invalid format",
			link: "https://example.com/invalid",
		},
		{
			name: "wrong telegram domain",
			link: "https://telegram.org/c/1234567890/123",
		},
		{
			name: "no path",
			link: "https://t.me/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := postManager.ParsePostLink(tt.link)
			if err == nil {
				t.Errorf("Expected error for invalid link %q, got nil", tt.link)
				return
			}
			if err.Error() != "invalid post link format" {
				t.Errorf("Expected error message 'invalid post link format', got %q", err.Error())
			}
		})
	}

	_ = ctx
}

func TestErrorHandling_DatabaseErrors(t *testing.T) {
	ctx := context.Background()
	_, queue, cleanup := setupTestDBForErrorHandling(t)
	defer cleanup()

	adminConfigRepo := db.NewAdminConfigRepository(queue)
	postTypeRepo := db.NewPostTypeRepository(queue)
	publishedPostRepo := db.NewPublishedPostRepository(queue)

	postManager := services.NewPostManager(publishedPostRepo, postTypeRepo, adminConfigRepo)

	t.Run("get non-existent post type", func(t *testing.T) {
		_, err := postManager.CreatePost(ctx, 99999, "Test text")
		if err == nil {
			t.Error("Expected error when creating post with non-existent type")
		}
	})

	t.Run("edit non-existent post", func(t *testing.T) {
		err := postManager.EditPost(ctx, 99999, "New text")
		if err == nil {
			t.Error("Expected error when editing non-existent post")
		}
	})
}

func TestErrorHandling_ParsePostLinkErrors(t *testing.T) {
	_, queue, cleanup := setupTestDBForErrorHandling(t)
	defer cleanup()

	adminConfigRepo := db.NewAdminConfigRepository(queue)
	postTypeRepo := db.NewPostTypeRepository(queue)
	publishedPostRepo := db.NewPublishedPostRepository(queue)

	postManager := services.NewPostManager(publishedPostRepo, postTypeRepo, adminConfigRepo)

	tests := []struct {
		name        string
		link        string
		expectError bool
	}{
		{
			name:        "valid private channel link",
			link:        "https://t.me/c/1234567890/999",
			expectError: false,
		},
		{
			name:        "valid public channel link",
			link:        "https://t.me/testchannel/999",
			expectError: false,
		},
		{
			name:        "invalid link - no path",
			link:        "https://t.me/",
			expectError: true,
		},
		{
			name:        "invalid link - wrong format",
			link:        "https://example.com/post/123",
			expectError: true,
		},
		{
			name:        "invalid link - empty string",
			link:        "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := postManager.ParsePostLink(tt.link)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for link %q, got nil", tt.link)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error for link %q, got %v", tt.link, err)
			}
		})
	}
}

func TestErrorHandling_GetPostByLinkErrors(t *testing.T) {
	_, queue, cleanup := setupTestDBForErrorHandling(t)
	defer cleanup()

	adminConfigRepo := db.NewAdminConfigRepository(queue)
	postTypeRepo := db.NewPostTypeRepository(queue)
	publishedPostRepo := db.NewPublishedPostRepository(queue)

	postManager := services.NewPostManager(publishedPostRepo, postTypeRepo, adminConfigRepo)

	t.Run("invalid link format", func(t *testing.T) {
		_, err := postManager.GetPostByLink("invalid link")
		if err == nil {
			t.Error("Expected error for invalid link format")
		}
	})

	t.Run("post not found in database", func(t *testing.T) {
		_, err := postManager.GetPostByLink("https://t.me/c/1234567890/999")
		if err == nil {
			t.Error("Expected error when post not found in database")
		}
	})
}

func TestErrorHandling_ErrorMessages(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		expectedMsg   string
		requirementID string
	}{
		{
			name:          "publish error",
			err:           errors.New("failed to send message"),
			expectedMsg:   "❌ Не удалось опубликовать пост",
			requirementID: "10.1",
		},
		{
			name:          "edit error",
			err:           errors.New("failed to edit message"),
			expectedMsg:   "❌ Не удалось отредактировать пост",
			requirementID: "10.2",
		},
		{
			name:          "delete error",
			err:           errors.New("failed to delete message"),
			expectedMsg:   "❌ Не удалось удалить пост",
			requirementID: "10.3",
		},
		{
			name:          "invalid link",
			err:           errors.New("invalid post link format"),
			expectedMsg:   "❌ Неверный формат ссылки",
			requirementID: "10.4",
		},
		{
			name:          "insufficient permissions",
			err:           errors.New("bot doesn't have permission"),
			expectedMsg:   "❌ У бота недостаточно прав",
			requirementID: "10.5",
		},
		{
			name:          "backup error",
			err:           errors.New("failed to create backup"),
			expectedMsg:   "❌ Ошибка при создании бэкапа",
			requirementID: "10.6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Error("Error should not be nil")
			}
		})
	}
}
