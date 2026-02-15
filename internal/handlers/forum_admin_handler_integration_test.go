package handlers

import (
	"context"
	"database/sql"
	"testing"

	"github.com/ad/go-telegram-admin/internal/db"
	"github.com/ad/go-telegram-admin/internal/fsm"
	"github.com/ad/go-telegram-admin/internal/models"
	"github.com/ad/go-telegram-admin/internal/services"
	_ "modernc.org/sqlite"
)

func setupTestDBForumAdmin(t *testing.T) (*db.DBQueue, func()) {
	sqlDB, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}

	err = db.InitSchema(sqlDB)
	if err != nil {
		t.Fatal(err)
	}

	queue := db.NewDBQueueForTest(sqlDB)
	return queue, func() { sqlDB.Close() }
}

func TestEndToEndPostCreation(t *testing.T) {
	queue, cleanup := setupTestDBForumAdmin(t)
	defer cleanup()

	adminConfigRepo := db.NewAdminConfigRepository(queue)
	postTypeRepo := db.NewPostTypeRepository(queue)
	publishedPostRepo := db.NewPublishedPostRepository(queue)
	adminStateRepo := db.NewAdminStateRepository(queue)

	adminID := int64(12345)
	forumChatID := int64(-1001234567890)
	topicID := int64(42)

	err := adminConfigRepo.Save(&models.AdminConfig{
		AdminIDs:    []int64{adminID},
		ForumChatID: forumChatID,
		TopicID:     topicID,
	})
	if err != nil {
		t.Fatalf("Failed to save admin config: %v", err)
	}

	postType := &models.PostType{
		Name:     "Test Type",
		PhotoID:  "test_photo_id",
		Template: "Test template",
		IsActive: true,
	}
	err = postTypeRepo.Create(postType)
	if err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	t.Run("Complete post creation flow", func(t *testing.T) {
		postText := "This is a test post"

		state := &models.AdminState{
			UserID:         adminID,
			CurrentState:   fsm.StateNewPostConfirm,
			SelectedTypeID: postType.ID,
			DraftText:      postText,
			DraftPhotoID:   postType.PhotoID,
		}
		err = adminStateRepo.Save(state)
		if err != nil {
			t.Fatalf("Failed to save admin state: %v", err)
		}

		retrievedState, err := adminStateRepo.Get(adminID)
		if err != nil {
			t.Fatalf("Failed to get admin state: %v", err)
		}

		if retrievedState.CurrentState != fsm.StateNewPostConfirm {
			t.Errorf("Expected state %s, got %s", fsm.StateNewPostConfirm, retrievedState.CurrentState)
		}

		if retrievedState.SelectedTypeID != postType.ID {
			t.Errorf("Expected selected type ID %d, got %d", postType.ID, retrievedState.SelectedTypeID)
		}

		if retrievedState.DraftText != postText {
			t.Errorf("Expected draft text %s, got %s", postText, retrievedState.DraftText)
		}

		if retrievedState.DraftPhotoID != postType.PhotoID {
			t.Errorf("Expected draft photo ID %s, got %s", postType.PhotoID, retrievedState.DraftPhotoID)
		}
	})

	t.Run("Post data persistence after publication", func(t *testing.T) {
		messageID := int64(999)
		postText := "Published post text"

		publishedPost := &models.PublishedPost{
			PostTypeID: postType.ID,
			ChatID:     forumChatID,
			TopicID:    topicID,
			MessageID:  messageID,
			Text:       postText,
			PhotoID:    postType.PhotoID,
		}

		err := publishedPostRepo.Create(publishedPost)
		if err != nil {
			t.Fatalf("Failed to create published post: %v", err)
		}

		retrievedPost, err := publishedPostRepo.GetByID(publishedPost.ID)
		if err != nil {
			t.Fatalf("Failed to get published post: %v", err)
		}

		if retrievedPost.PostTypeID != postType.ID {
			t.Errorf("Expected post type ID %d, got %d", postType.ID, retrievedPost.PostTypeID)
		}

		if retrievedPost.ChatID != forumChatID {
			t.Errorf("Expected chat ID %d, got %d", forumChatID, retrievedPost.ChatID)
		}

		if retrievedPost.TopicID != topicID {
			t.Errorf("Expected topic ID %d, got %d", topicID, retrievedPost.TopicID)
		}

		if retrievedPost.MessageID != messageID {
			t.Errorf("Expected message ID %d, got %d", messageID, retrievedPost.MessageID)
		}

		if retrievedPost.Text != postText {
			t.Errorf("Expected text %s, got %s", postText, retrievedPost.Text)
		}

		if retrievedPost.PhotoID != postType.PhotoID {
			t.Errorf("Expected photo ID %s, got %s", postType.PhotoID, retrievedPost.PhotoID)
		}
	})

	t.Run("State cleared after publication", func(t *testing.T) {
		state := &models.AdminState{
			UserID:         adminID,
			CurrentState:   fsm.StateNewPostConfirm,
			SelectedTypeID: postType.ID,
			DraftText:      "Test",
			DraftPhotoID:   "test",
		}
		err := adminStateRepo.Save(state)
		if err != nil {
			t.Fatalf("Failed to save admin state: %v", err)
		}

		err = adminStateRepo.Clear(adminID)
		if err != nil {
			t.Fatalf("Failed to clear admin state: %v", err)
		}

		clearedState, err := adminStateRepo.Get(adminID)
		if err != nil && err.Error() != "sql: no rows in result set" {
			t.Fatalf("Failed to get admin state after clear: %v", err)
		}

		if clearedState != nil {
			t.Error("Admin state should be nil after clearing")
		}
	})

	t.Run("Post retrieval by message ID", func(t *testing.T) {
		messageID := int64(888)
		postText := "Another test post"

		publishedPost := &models.PublishedPost{
			PostTypeID: postType.ID,
			ChatID:     forumChatID,
			TopicID:    topicID,
			MessageID:  messageID,
			Text:       postText,
			PhotoID:    "",
		}

		err := publishedPostRepo.Create(publishedPost)
		if err != nil {
			t.Fatalf("Failed to create published post: %v", err)
		}

		retrievedPost, err := publishedPostRepo.GetByMessageID(forumChatID, messageID)
		if err != nil {
			t.Fatalf("Failed to get published post by message ID: %v", err)
		}

		if retrievedPost.MessageID != messageID {
			t.Errorf("Expected message ID %d, got %d", messageID, retrievedPost.MessageID)
		}

		if retrievedPost.Text != postText {
			t.Errorf("Expected text %s, got %s", postText, retrievedPost.Text)
		}
	})

	t.Run("Post manager integration", func(t *testing.T) {
		ctx := context.Background()
		postManager := services.NewPostManager(publishedPostRepo, postTypeRepo, adminConfigRepo)

		postText := "Post created via manager"
		post, err := postManager.CreatePost(ctx, postType.ID, postText)
		if err != nil {
			t.Fatalf("Failed to create post via manager: %v", err)
		}

		if post.PostTypeID != postType.ID {
			t.Errorf("Expected post type ID %d, got %d", postType.ID, post.PostTypeID)
		}

		if post.ChatID != forumChatID {
			t.Errorf("Expected chat ID %d, got %d", forumChatID, post.ChatID)
		}

		if post.TopicID != topicID {
			t.Errorf("Expected topic ID %d, got %d", topicID, post.TopicID)
		}

		if post.Text != postText {
			t.Errorf("Expected text %s, got %s", postText, post.Text)
		}

		if post.PhotoID != postType.PhotoID {
			t.Errorf("Expected photo ID %s, got %s", postType.PhotoID, post.PhotoID)
		}
	})

	t.Run("Post without photo", func(t *testing.T) {
		postTypeNoPhoto := &models.PostType{
			Name:     "Type Without Photo",
			PhotoID:  "",
			Template: "Template without photo",
			IsActive: true,
		}
		err := postTypeRepo.Create(postTypeNoPhoto)
		if err != nil {
			t.Fatalf("Failed to create post type without photo: %v", err)
		}

		messageID := int64(777)
		postText := "Post without photo"

		publishedPost := &models.PublishedPost{
			PostTypeID: postTypeNoPhoto.ID,
			ChatID:     forumChatID,
			TopicID:    topicID,
			MessageID:  messageID,
			Text:       postText,
			PhotoID:    "",
		}

		err = publishedPostRepo.Create(publishedPost)
		if err != nil {
			t.Fatalf("Failed to create published post without photo: %v", err)
		}

		retrievedPost, err := publishedPostRepo.GetByID(publishedPost.ID)
		if err != nil {
			t.Fatalf("Failed to get published post: %v", err)
		}

		if retrievedPost.PhotoID != "" {
			t.Errorf("Expected empty photo ID, got %s", retrievedPost.PhotoID)
		}

		if retrievedPost.Text != postText {
			t.Errorf("Expected text %s, got %s", postText, retrievedPost.Text)
		}
	})
}

func TestSettingsPersistence(t *testing.T) {
	queue, cleanup := setupTestDBForumAdmin(t)
	defer cleanup()

	adminConfigRepo := db.NewAdminConfigRepository(queue)
	settingsManager := services.NewSettingsManager(adminConfigRepo)

	t.Run("Settings changes persist across operations", func(t *testing.T) {
		admin1 := int64(12345)
		admin2 := int64(67890)
		forumID := int64(-1001234567890)
		topicID := int64(42)

		if err := settingsManager.AddAdmin(admin1); err != nil {
			t.Fatalf("Failed to add admin 1: %v", err)
		}
		if err := settingsManager.AddAdmin(admin2); err != nil {
			t.Fatalf("Failed to add admin 2: %v", err)
		}

		if err := settingsManager.SetForumConfig(forumID, topicID); err != nil {
			t.Fatalf("Failed to set forum config: %v", err)
		}

		admins, err := settingsManager.GetAdmins()
		if err != nil {
			t.Fatalf("Failed to get admins: %v", err)
		}
		if len(admins) != 2 {
			t.Errorf("Expected 2 admins, got %d", len(admins))
		}

		retrievedForumID, retrievedTopicID, err := settingsManager.GetForumConfig()
		if err != nil {
			t.Fatalf("Failed to get forum config: %v", err)
		}
		if retrievedForumID != forumID {
			t.Errorf("Expected forum ID %d, got %d", forumID, retrievedForumID)
		}
		if retrievedTopicID != topicID {
			t.Errorf("Expected topic ID %d, got %d", topicID, retrievedTopicID)
		}

		if err := settingsManager.RemoveAdmin(admin2); err != nil {
			t.Fatalf("Failed to remove admin 2: %v", err)
		}

		admins, err = settingsManager.GetAdmins()
		if err != nil {
			t.Fatalf("Failed to get admins after removal: %v", err)
		}
		if len(admins) != 1 {
			t.Errorf("Expected 1 admin after removal, got %d", len(admins))
		}
		if admins[0] != admin1 {
			t.Errorf("Expected remaining admin %d, got %d", admin1, admins[0])
		}

		retrievedForumID, retrievedTopicID, err = settingsManager.GetForumConfig()
		if err != nil {
			t.Fatalf("Failed to get forum config after admin removal: %v", err)
		}
		if retrievedForumID != forumID {
			t.Errorf("Forum config should persist after admin removal, expected %d, got %d", forumID, retrievedForumID)
		}
		if retrievedTopicID != topicID {
			t.Errorf("Topic config should persist after admin removal, expected %d, got %d", topicID, retrievedTopicID)
		}
	})

	t.Run("Settings apply to subsequent operations", func(t *testing.T) {
		queue2, cleanup2 := setupTestDBForumAdmin(t)
		defer cleanup2()

		adminConfigRepo2 := db.NewAdminConfigRepository(queue2)
		settingsManager2 := services.NewSettingsManager(adminConfigRepo2)
		postTypeRepo := db.NewPostTypeRepository(queue2)
		publishedPostRepo := db.NewPublishedPostRepository(queue2)

		adminID := int64(11111)
		forumID := int64(-1009876543210)
		topicID := int64(99)

		if err := settingsManager2.AddAdmin(adminID); err != nil {
			t.Fatalf("Failed to add admin: %v", err)
		}
		if err := settingsManager2.SetForumConfig(forumID, topicID); err != nil {
			t.Fatalf("Failed to set forum config: %v", err)
		}

		postType := &models.PostType{
			Name:     "Integration Test Type",
			PhotoID:  "",
			Template: "Integration test template",
			IsActive: true,
		}
		if err := postTypeRepo.Create(postType); err != nil {
			t.Fatalf("Failed to create post type: %v", err)
		}

		config, err := adminConfigRepo2.Get()
		if err != nil {
			t.Fatalf("Failed to get config: %v", err)
		}

		publishedPost := &models.PublishedPost{
			PostTypeID: postType.ID,
			ChatID:     config.ForumChatID,
			TopicID:    config.TopicID,
			MessageID:  12345,
			Text:       "Test post using configured settings",
			PhotoID:    "",
		}
		if err := publishedPostRepo.Create(publishedPost); err != nil {
			t.Fatalf("Failed to create published post: %v", err)
		}

		retrievedPost, err := publishedPostRepo.GetByID(publishedPost.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve published post: %v", err)
		}

		if retrievedPost.ChatID != forumID {
			t.Errorf("Post should use configured forum ID, expected %d, got %d", forumID, retrievedPost.ChatID)
		}
		if retrievedPost.TopicID != topicID {
			t.Errorf("Post should use configured topic ID, expected %d, got %d", topicID, retrievedPost.TopicID)
		}

		isAdmin, err := settingsManager2.IsAdmin(adminID)
		if err != nil {
			t.Fatalf("Failed to check admin status: %v", err)
		}
		if !isAdmin {
			t.Error("Configured admin should be recognized as admin")
		}

		isAdmin, err = settingsManager2.IsAdmin(99999)
		if err != nil {
			t.Fatalf("Failed to check non-admin status: %v", err)
		}
		if isAdmin {
			t.Error("Non-configured user should not be recognized as admin")
		}
	})
}
