package db

import (
	"database/sql"
	"testing"

	"github.com/ad/go-telegram-admin/internal/models"
	_ "modernc.org/sqlite"
	"pgregory.net/rapid"
)

func TestProperty15_AdminListManagement(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		testDB, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			rt.Fatal(err)
		}
		defer testDB.Close()

		if err := InitSchema(testDB); err != nil {
			rt.Fatal(err)
		}

		queue := NewDBQueueForTest(testDB)
		repo := NewAdminConfigRepository(queue)

		adminID := rapid.Int64Range(1, 1000000).Draw(rt, "adminID")

		if err := repo.AddAdmin(adminID); err != nil {
			rt.Fatal(err)
		}

		isAdmin, err := repo.IsAdmin(adminID)
		if err != nil {
			rt.Fatal(err)
		}

		if !isAdmin {
			rt.Fatalf("Expected admin ID %d to be in admin list after adding", adminID)
		}

		if err := repo.RemoveAdmin(adminID); err != nil {
			rt.Fatal(err)
		}

		isAdmin, err = repo.IsAdmin(adminID)
		if err != nil {
			rt.Fatal(err)
		}

		if isAdmin {
			rt.Fatalf("Expected admin ID %d to not be in admin list after removing", adminID)
		}
	})
}

func TestProperty18_DataPersistenceRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		testDB, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			rt.Fatal(err)
		}
		defer testDB.Close()

		if err := InitSchema(testDB); err != nil {
			rt.Fatal(err)
		}

		queue := NewDBQueueForTest(testDB)
		adminConfigRepo := NewAdminConfigRepository(queue)
		postTypeRepo := NewPostTypeRepository(queue)
		publishedPostRepo := NewPublishedPostRepository(queue)

		numAdmins := rapid.IntRange(1, 10).Draw(rt, "numAdmins")
		adminIDs := make([]int64, numAdmins)
		for i := 0; i < numAdmins; i++ {
			adminIDs[i] = rapid.Int64Range(1, 1000000).Draw(rt, "adminID")
		}

		forumChatID := rapid.Int64Range(-1000000000000, -1).Draw(rt, "forumChatID")
		topicID := rapid.Int64Range(1, 1000000).Draw(rt, "topicID")

		config := &models.AdminConfig{
			AdminIDs:    adminIDs,
			ForumChatID: forumChatID,
			TopicID:     topicID,
		}

		if err := adminConfigRepo.Save(config); err != nil {
			rt.Fatal(err)
		}

		retrieved, err := adminConfigRepo.Get()
		if err != nil {
			rt.Fatal(err)
		}

		if len(retrieved.AdminIDs) != len(config.AdminIDs) {
			rt.Fatalf("Expected %d admin IDs, got %d", len(config.AdminIDs), len(retrieved.AdminIDs))
		}

		for i, id := range config.AdminIDs {
			if retrieved.AdminIDs[i] != id {
				rt.Fatalf("Expected admin ID %d at position %d, got %d", id, i, retrieved.AdminIDs[i])
			}
		}

		if retrieved.ForumChatID != config.ForumChatID {
			rt.Fatalf("Expected forum chat ID %d, got %d", config.ForumChatID, retrieved.ForumChatID)
		}

		if retrieved.TopicID != config.TopicID {
			rt.Fatalf("Expected topic ID %d, got %d", config.TopicID, retrieved.TopicID)
		}

		postType := &models.PostType{
			Name:     rapid.StringMatching(`[a-zA-Zа-яА-Я ]{3,30}`).Draw(rt, "name"),
			PhotoID:  rapid.StringMatching(`[a-zA-Z0-9_-]{0,50}`).Draw(rt, "photoID"),
			Template: rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s\.,!?]{10,200}`).Draw(rt, "template"),
			IsActive: rapid.Bool().Draw(rt, "isActive"),
		}

		if err := postTypeRepo.Create(postType); err != nil {
			rt.Fatal(err)
		}

		retrievedPostType, err := postTypeRepo.GetByID(postType.ID)
		if err != nil {
			rt.Fatal(err)
		}

		if retrievedPostType.Name != postType.Name {
			rt.Fatalf("Expected post type name %s, got %s", postType.Name, retrievedPostType.Name)
		}
		if retrievedPostType.PhotoID != postType.PhotoID {
			rt.Fatalf("Expected post type photo ID %s, got %s", postType.PhotoID, retrievedPostType.PhotoID)
		}
		if retrievedPostType.Template != postType.Template {
			rt.Fatalf("Expected post type template %s, got %s", postType.Template, retrievedPostType.Template)
		}
		if retrievedPostType.IsActive != postType.IsActive {
			rt.Fatalf("Expected post type is_active %v, got %v", postType.IsActive, retrievedPostType.IsActive)
		}

		publishedPost := &models.PublishedPost{
			PostTypeID: postType.ID,
			ChatID:     rapid.Int64Range(-1000000000000, -1).Draw(rt, "chatID"),
			TopicID:    rapid.Int64Range(1, 1000000).Draw(rt, "topicID"),
			MessageID:  rapid.Int64Range(1, 1000000).Draw(rt, "messageID"),
			Text:       rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s\.,!?]{10,200}`).Draw(rt, "text"),
			PhotoID:    rapid.StringMatching(`[a-zA-Z0-9_-]{0,50}`).Draw(rt, "photoID"),
		}

		if err := publishedPostRepo.Create(publishedPost); err != nil {
			rt.Fatal(err)
		}

		retrievedPost, err := publishedPostRepo.GetByID(publishedPost.ID)
		if err != nil {
			rt.Fatal(err)
		}

		if retrievedPost.PostTypeID != publishedPost.PostTypeID {
			rt.Fatalf("Expected post type ID %d, got %d", publishedPost.PostTypeID, retrievedPost.PostTypeID)
		}
		if retrievedPost.ChatID != publishedPost.ChatID {
			rt.Fatalf("Expected chat ID %d, got %d", publishedPost.ChatID, retrievedPost.ChatID)
		}
		if retrievedPost.TopicID != publishedPost.TopicID {
			rt.Fatalf("Expected topic ID %d, got %d", publishedPost.TopicID, retrievedPost.TopicID)
		}
		if retrievedPost.MessageID != publishedPost.MessageID {
			rt.Fatalf("Expected message ID %d, got %d", publishedPost.MessageID, retrievedPost.MessageID)
		}
		if retrievedPost.Text != publishedPost.Text {
			rt.Fatalf("Expected text %s, got %s", publishedPost.Text, retrievedPost.Text)
		}
		if retrievedPost.PhotoID != publishedPost.PhotoID {
			rt.Fatalf("Expected photo ID %s, got %s", publishedPost.PhotoID, retrievedPost.PhotoID)
		}
	})
}
