package services

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/ad/go-telegram-admin/internal/db"
	"github.com/ad/go-telegram-admin/internal/models"
	_ "modernc.org/sqlite"
	"pgregory.net/rapid"
)

func TestParsePostLink_ValidLinks(t *testing.T) {
	pm := &PostManager{}

	tests := []struct {
		name           string
		link           string
		expectedChatID int64
		expectedMsgID  int64
		shouldError    bool
	}{
		{
			name:           "t.me/c format",
			link:           "https://t.me/c/1234567890/42",
			expectedChatID: -1001234567890,
			expectedMsgID:  42,
			shouldError:    false,
		},
		{
			name:           "telegram.me/c format",
			link:           "https://telegram.me/c/9876543210/100",
			expectedChatID: -1009876543210,
			expectedMsgID:  100,
			shouldError:    false,
		},
		{
			name:           "t.me with username",
			link:           "https://t.me/testchannel/123",
			expectedChatID: 0,
			expectedMsgID:  123,
			shouldError:    false,
		},
		{
			name:           "telegram.me with username",
			link:           "https://telegram.me/anotherchannel/456",
			expectedChatID: 0,
			expectedMsgID:  456,
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatID, msgID, err := pm.ParsePostLink(tt.link)

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.name == "t.me with username" || tt.name == "telegram.me with username" {
				if msgID != tt.expectedMsgID {
					t.Errorf("expected message ID %d, got %d", tt.expectedMsgID, msgID)
				}
			} else {
				if chatID != tt.expectedChatID {
					t.Errorf("expected chat ID %d, got %d", tt.expectedChatID, chatID)
				}
				if msgID != tt.expectedMsgID {
					t.Errorf("expected message ID %d, got %d", tt.expectedMsgID, msgID)
				}
			}
		})
	}
}

func TestParsePostLink_InvalidLinks(t *testing.T) {
	pm := &PostManager{}

	tests := []struct {
		name string
		link string
	}{
		{
			name: "completely invalid URL",
			link: "not a url at all",
		},
		{
			name: "wrong domain",
			link: "https://example.com/c/1234567890/42",
		},
		{
			name: "empty string",
			link: "",
		},
		{
			name: "malformed path",
			link: "https://t.me/invalid/path/structure/123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := pm.ParsePostLink(tt.link)
			if err == nil {
				t.Errorf("expected error for invalid link but got none")
			}
		})
	}
}

func TestParsePostLink_DifferentFormats(t *testing.T) {
	pm := &PostManager{}

	tests := []struct {
		name        string
		link        string
		shouldError bool
	}{
		{
			name:        "with http protocol",
			link:        "http://t.me/c/1234567890/42",
			shouldError: false,
		},
		{
			name:        "with https protocol",
			link:        "https://t.me/c/1234567890/42",
			shouldError: false,
		},
		{
			name:        "without protocol",
			link:        "t.me/c/1234567890/42",
			shouldError: false,
		},
		{
			name:        "with trailing slash",
			link:        "https://t.me/c/1234567890/42/",
			shouldError: false,
		},
		{
			name:        "with query parameters",
			link:        "https://t.me/c/1234567890/42?comment=123",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := pm.ParsePostLink(tt.link)
			if tt.shouldError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestProperty10_PostLinkValidation(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		pm := &PostManager{}

		chatIDNum := rapid.Int64Range(1000000000, 9999999999).Draw(rt, "chatIDNum")
		messageID := rapid.Int64Range(1, 999999).Draw(rt, "messageID")

		linkFormat := rapid.IntRange(0, 1).Draw(rt, "linkFormat")
		var link string
		if linkFormat == 0 {
			link = fmt.Sprintf("https://t.me/c/%d/%d", chatIDNum, messageID)
		} else {
			link = fmt.Sprintf("https://telegram.me/c/%d/%d", chatIDNum, messageID)
		}

		parsedChatID, parsedMessageID, err := pm.ParsePostLink(link)
		if err != nil {
			rt.Fatalf("Failed to parse valid link %s: %v", link, err)
		}

		expectedChatID := -1000000000000 - chatIDNum
		if parsedChatID != expectedChatID {
			rt.Fatalf("Expected chat ID %d, got %d", expectedChatID, parsedChatID)
		}

		if parsedMessageID != messageID {
			rt.Fatalf("Expected message ID %d, got %d", messageID, parsedMessageID)
		}
	})
}

func TestProperty11_PostEditPreservation(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		testDB, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			rt.Fatal(err)
		}
		defer testDB.Close()

		if err := db.InitSchema(testDB); err != nil {
			rt.Fatal(err)
		}

		queue := db.NewDBQueueForTest(testDB)
		postRepo := db.NewPublishedPostRepository(queue)
		postTypeRepo := db.NewPostTypeRepository(queue)
		configRepo := db.NewAdminConfigRepository(queue)

		pm := NewPostManager(postRepo, postTypeRepo, configRepo)

		postType := &models.PostType{
			Name:     rapid.StringMatching(`[a-zA-Zа-яА-Я ]{3,30}`).Draw(rt, "name"),
			PhotoID:  rapid.StringMatching(`[a-zA-Z0-9_-]{1,50}`).Draw(rt, "photoID"),
			Template: rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s\.,!?]{10,200}`).Draw(rt, "template"),
			IsActive: true,
		}

		if err := postTypeRepo.Create(postType); err != nil {
			rt.Fatal(err)
		}

		originalText := rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s\.,!?]{10,200}`).Draw(rt, "originalText")
		post := &models.PublishedPost{
			PostTypeID: postType.ID,
			ChatID:     rapid.Int64Range(-1000000000000, -1).Draw(rt, "chatID"),
			TopicID:    rapid.Int64Range(1, 999999).Draw(rt, "topicID"),
			MessageID:  rapid.Int64Range(1, 999999).Draw(rt, "messageID"),
			Text:       originalText,
			PhotoID:    postType.PhotoID,
		}

		if err := postRepo.Create(post); err != nil {
			rt.Fatal(err)
		}

		originalPhotoID := post.PhotoID
		newText := rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s\.,!?]{10,200}`).Draw(rt, "newText")

		if err := pm.EditPost(context.Background(), post.ID, newText); err != nil {
			rt.Fatal(err)
		}

		retrieved, err := postRepo.GetByID(post.ID)
		if err != nil {
			rt.Fatal(err)
		}

		if retrieved.Text != newText {
			rt.Fatalf("Expected text to be updated to %s, got %s", newText, retrieved.Text)
		}

		if retrieved.PhotoID != originalPhotoID {
			rt.Fatalf("Expected photo ID to be preserved as %s, got %s", originalPhotoID, retrieved.PhotoID)
		}
	})
}

func TestProperty12_PostDeletionCleanup(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatal(err)
	}

	queue := db.NewDBQueueForTest(testDB)
	postRepo := db.NewPublishedPostRepository(queue)
	postTypeRepo := db.NewPostTypeRepository(queue)
	configRepo := db.NewAdminConfigRepository(queue)

	pm := NewPostManager(postRepo, postTypeRepo, configRepo)

	rapid.Check(t, func(rt *rapid.T) {
		postType := &models.PostType{
			Name:     rapid.StringMatching(`[a-zA-Zа-яА-Я ]{3,30}`).Draw(rt, "name"),
			PhotoID:  rapid.StringMatching(`[a-zA-Z0-9_-]{0,50}`).Draw(rt, "photoID"),
			Template: rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s\.,!?]{10,200}`).Draw(rt, "template"),
			IsActive: true,
		}

		if err := postTypeRepo.Create(postType); err != nil {
			rt.Fatal(err)
		}

		post := &models.PublishedPost{
			PostTypeID: postType.ID,
			ChatID:     rapid.Int64Range(-1000000000000, -1).Draw(rt, "chatID"),
			TopicID:    rapid.Int64Range(1, 999999).Draw(rt, "topicID"),
			MessageID:  rapid.Int64Range(1, 999999).Draw(rt, "messageID"),
			Text:       rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s\.,!?]{10,200}`).Draw(rt, "text"),
			PhotoID:    postType.PhotoID,
		}

		if err := postRepo.Create(post); err != nil {
			rt.Fatal(err)
		}

		if err := pm.DeletePost(context.Background(), post.ID); err != nil {
			rt.Fatal(err)
		}

		_, err = postRepo.GetByID(post.ID)
		if err == nil {
			rt.Fatalf("Expected post to be deleted, but it still exists")
		}
		if err != sql.ErrNoRows {
			rt.Fatalf("Expected sql.ErrNoRows, got %v", err)
		}
	})
}

func TestValidLinkIsProcessed(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	postRepo := db.NewPublishedPostRepository(queue)
	postTypeRepo := db.NewPostTypeRepository(queue)
	configRepo := db.NewAdminConfigRepository(queue)

	pm := NewPostManager(postRepo, postTypeRepo, configRepo)

	postType := &models.PostType{
		Name:     "Test Type",
		PhotoID:  "photo123",
		Template: "Template",
		IsActive: true,
	}
	if err := postTypeRepo.Create(postType); err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	post := &models.PublishedPost{
		PostTypeID: postType.ID,
		ChatID:     -1001234567890,
		TopicID:    42,
		MessageID:  100,
		Text:       "Original text",
		PhotoID:    "photo123",
	}
	if err := postRepo.Create(post); err != nil {
		t.Fatalf("Failed to create post: %v", err)
	}

	link := "https://t.me/c/1234567890/100"
	retrievedPost, err := pm.GetPostByLink(link)
	if err != nil {
		t.Fatalf("Valid link should be processed: %v", err)
	}

	if retrievedPost == nil {
		t.Fatal("Expected post to be retrieved")
	}

	if retrievedPost.ID != post.ID {
		t.Errorf("Expected post ID %d, got %d", post.ID, retrievedPost.ID)
	}

	if retrievedPost.Text != post.Text {
		t.Errorf("Expected text %q, got %q", post.Text, retrievedPost.Text)
	}
}

func TestInvalidLinkReturnsError(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	postRepo := db.NewPublishedPostRepository(queue)
	postTypeRepo := db.NewPostTypeRepository(queue)
	configRepo := db.NewAdminConfigRepository(queue)

	pm := NewPostManager(postRepo, postTypeRepo, configRepo)

	invalidLinks := []string{
		"not a link",
		"https://example.com/invalid",
		"",
		"https://t.me/invalid/format/123",
	}

	for _, link := range invalidLinks {
		_, err := pm.GetPostByLink(link)
		if err == nil {
			t.Errorf("Invalid link %q should return error", link)
		}
	}
}

func TestNonExistentPostReturnsError(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	postRepo := db.NewPublishedPostRepository(queue)
	postTypeRepo := db.NewPostTypeRepository(queue)
	configRepo := db.NewAdminConfigRepository(queue)

	pm := NewPostManager(postRepo, postTypeRepo, configRepo)

	link := "https://t.me/c/1234567890/999"
	_, err = pm.GetPostByLink(link)
	if err == nil {
		t.Error("Non-existent post should return error")
	}

	if err != sql.ErrNoRows {
		t.Errorf("Expected sql.ErrNoRows, got %v", err)
	}
}
