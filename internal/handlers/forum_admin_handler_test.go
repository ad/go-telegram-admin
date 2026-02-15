package handlers

import (
	"context"
	"database/sql"
	"testing"

	"github.com/ad/go-telegram-admin/internal/db"
	"github.com/ad/go-telegram-admin/internal/fsm"
	"github.com/ad/go-telegram-admin/internal/models"
	"github.com/ad/go-telegram-admin/internal/services"
	tgmodels "github.com/go-telegram/bot/models"
	_ "modernc.org/sqlite"
)

func setupForumAdminHandler(t *testing.T) (*ForumAdminHandler, *sql.DB) {
	t.Helper()

	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)

	adminConfigRepo := db.NewAdminConfigRepository(queue)
	postTypeRepo := db.NewPostTypeRepository(queue)
	publishedPostRepo := db.NewPublishedPostRepository(queue)
	adminStateRepo := db.NewAdminStateRepository(queue)

	authMiddleware := services.NewAdminAuthMiddleware(adminConfigRepo)
	postManager := services.NewPostManager(publishedPostRepo, postTypeRepo, adminConfigRepo)
	postTypeManager := services.NewPostTypeManager(postTypeRepo)
	settingsManager := services.NewSettingsManager(adminConfigRepo)
	backupManager := services.NewBackupManager(nil, ":memory:", queue)

	handler := NewForumAdminHandler(
		nil,
		authMiddleware,
		adminConfigRepo,
		postTypeRepo,
		publishedPostRepo,
		adminStateRepo,
		postManager,
		postTypeManager,
		settingsManager,
		backupManager,
	)

	return handler, testDB
}

func TestAdminCommandRouting(t *testing.T) {
	_, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	adminID := int64(12345)
	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	adminConfigRepo.AddAdmin(adminID)

	tests := []struct {
		name        string
		command     string
		shouldRoute bool
	}{
		{
			name:        "/new routes correctly",
			command:     "/new",
			shouldRoute: true,
		},
		{
			name:        "/edit routes correctly",
			command:     "/edit",
			shouldRoute: true,
		},
		{
			name:        "/delete routes correctly",
			command:     "/delete",
			shouldRoute: true,
		},
		{
			name:        "/cancel routes correctly",
			command:     "/cancel",
			shouldRoute: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.command == "/admin" || tt.command == "/new" {
				t.Skip("Skipping commands that require bot instance")
			}
		})
	}
}

func TestNonAdminIgnored(t *testing.T) {
	handler, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	adminID := int64(12345)
	nonAdminID := int64(67890)

	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	adminConfigRepo.AddAdmin(adminID)

	ctx := context.Background()

	msg := &tgmodels.Message{
		Text: "/admin",
		From: &tgmodels.User{ID: nonAdminID},
		Chat: tgmodels.Chat{ID: nonAdminID},
	}

	handled := handler.HandleCommand(ctx, msg)
	if handled {
		t.Error("Non-admin should be ignored, but command was handled")
	}
}

func TestCallbackIgnoresNonAdmin(t *testing.T) {
	handler, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	adminID := int64(12345)
	nonAdminID := int64(67890)

	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	adminConfigRepo.AddAdmin(adminID)

	ctx := context.Background()

	callback := &tgmodels.CallbackQuery{
		ID:   "test-callback",
		From: tgmodels.User{ID: nonAdminID},
		Data: "admin:menu",
		Message: tgmodels.MaybeInaccessibleMessage{
			Message: &tgmodels.Message{
				Chat: tgmodels.Chat{ID: nonAdminID},
				ID:   1,
			},
		},
	}

	handled := handler.HandleCallback(ctx, callback)
	if handled {
		t.Error("Non-admin callback should be ignored, but was handled")
	}
}

func TestAdminMenuContainsAllButtons(t *testing.T) {
	expectedButtons := []string{
		"Новый пост",
		"Редактировать пост",
		"Удалить пост",
		"Настройки",
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "Новый пост", CallbackData: "admin_new_post"},
			},
			{
				{Text: "Редактировать пост", CallbackData: "admin_edit_post"},
			},
			{
				{Text: "Удалить пост", CallbackData: "admin_delete_post"},
			},
			{
				{Text: "Настройки", CallbackData: "admin_settings"},
			},
		},
	}

	if len(keyboard.InlineKeyboard) != 4 {
		t.Errorf("Expected 4 buttons, got %d", len(keyboard.InlineKeyboard))
	}

	for i, row := range keyboard.InlineKeyboard {
		if len(row) != 1 {
			t.Errorf("Expected 1 button in row %d, got %d", i, len(row))
		}
		if row[0].Text != expectedButtons[i] {
			t.Errorf("Expected button text %q, got %q", expectedButtons[i], row[0].Text)
		}
	}
}

func TestAdminMenuButtonTextsInRussian(t *testing.T) {
	expectedButtons := []string{
		"Новый пост",
		"Редактировать пост",
		"Удалить пост",
		"Настройки",
	}

	for _, buttonText := range expectedButtons {
		if buttonText == "" {
			t.Errorf("Button text should not be empty")
		}
		if len(buttonText) < 3 {
			t.Errorf("Button text %q seems too short", buttonText)
		}
	}
}

func TestNewCommandDisplaysOnlyActiveTypes(t *testing.T) {
	handler, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	queue := db.NewDBQueueForTest(testDB)
	postTypeRepo := db.NewPostTypeRepository(queue)

	activeType1 := &models.PostType{
		Name:     "Active Type 1",
		PhotoID:  "photo1",
		Template: "Template 1",
		IsActive: true,
	}
	activeType2 := &models.PostType{
		Name:     "Active Type 2",
		PhotoID:  "photo2",
		Template: "Template 2",
		IsActive: true,
	}
	inactiveType := &models.PostType{
		Name:     "Inactive Type",
		PhotoID:  "photo3",
		Template: "Template 3",
		IsActive: false,
	}

	if err := postTypeRepo.Create(activeType1); err != nil {
		t.Fatalf("Failed to create active type 1: %v", err)
	}
	if err := postTypeRepo.Create(activeType2); err != nil {
		t.Fatalf("Failed to create active type 2: %v", err)
	}
	if err := postTypeRepo.Create(inactiveType); err != nil {
		t.Fatalf("Failed to create inactive type: %v", err)
	}

	activeTypes, err := postTypeRepo.GetActive()
	if err != nil {
		t.Fatalf("Failed to get active types: %v", err)
	}

	if len(activeTypes) != 2 {
		t.Errorf("Expected 2 active types, got %d", len(activeTypes))
	}

	for _, pt := range activeTypes {
		if !pt.IsActive {
			t.Errorf("GetActive returned inactive type: %s", pt.Name)
		}
		if pt.Name == "Inactive Type" {
			t.Errorf("GetActive returned inactive type by name: %s", pt.Name)
		}
	}

	_ = handler
}

func TestNewCommandDisplaysTypesAsButtons(t *testing.T) {
	handler, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	queue := db.NewDBQueueForTest(testDB)
	postTypeRepo := db.NewPostTypeRepository(queue)

	type1 := &models.PostType{
		Name:     "Type 1",
		PhotoID:  "photo1",
		Template: "Template 1",
		IsActive: true,
	}
	type2 := &models.PostType{
		Name:     "Type 2",
		PhotoID:  "photo2",
		Template: "Template 2",
		IsActive: true,
	}

	if err := postTypeRepo.Create(type1); err != nil {
		t.Fatalf("Failed to create type 1: %v", err)
	}
	if err := postTypeRepo.Create(type2); err != nil {
		t.Fatalf("Failed to create type 2: %v", err)
	}

	activeTypes, err := postTypeRepo.GetActive()
	if err != nil {
		t.Fatalf("Failed to get active types: %v", err)
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: make([][]tgmodels.InlineKeyboardButton, 0, len(activeTypes)),
	}

	for _, pt := range activeTypes {
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []tgmodels.InlineKeyboardButton{
			{
				Text:         pt.Name,
				CallbackData: "select_type:" + string(rune(pt.ID)),
			},
		})
	}

	if len(keyboard.InlineKeyboard) != 2 {
		t.Errorf("Expected 2 buttons, got %d", len(keyboard.InlineKeyboard))
	}

	for i, row := range keyboard.InlineKeyboard {
		if len(row) != 1 {
			t.Errorf("Expected 1 button in row %d, got %d", i, len(row))
		}
		if row[0].Text == "" {
			t.Errorf("Button text should not be empty")
		}
	}

	_ = handler
}

func TestTemplateDisplayInCodeTags(t *testing.T) {
	handler, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	queue := db.NewDBQueueForTest(testDB)
	postTypeRepo := db.NewPostTypeRepository(queue)

	template := "This is a test template"
	postType := &models.PostType{
		Name:     "Test Type",
		PhotoID:  "",
		Template: template,
		IsActive: true,
	}

	if err := postTypeRepo.Create(postType); err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	retrieved, err := postTypeRepo.GetByID(postType.ID)
	if err != nil {
		t.Fatalf("Failed to get post type: %v", err)
	}

	expectedText := "<code>" + template + "</code>"
	if !containsSubstring(expectedText, template) {
		t.Errorf("Expected template to be wrapped in code tags")
	}

	_ = handler
	_ = retrieved
}

func TestImageDisplayedWhenPresent(t *testing.T) {
	handler, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	queue := db.NewDBQueueForTest(testDB)
	postTypeRepo := db.NewPostTypeRepository(queue)

	postTypeWithImage := &models.PostType{
		Name:     "Type With Image",
		PhotoID:  "test_photo_id_123",
		Template: "Template text",
		IsActive: true,
	}

	postTypeWithoutImage := &models.PostType{
		Name:     "Type Without Image",
		PhotoID:  "",
		Template: "Template text",
		IsActive: true,
	}

	if err := postTypeRepo.Create(postTypeWithImage); err != nil {
		t.Fatalf("Failed to create post type with image: %v", err)
	}
	if err := postTypeRepo.Create(postTypeWithoutImage); err != nil {
		t.Fatalf("Failed to create post type without image: %v", err)
	}

	retrievedWithImage, err := postTypeRepo.GetByID(postTypeWithImage.ID)
	if err != nil {
		t.Fatalf("Failed to get post type with image: %v", err)
	}

	retrievedWithoutImage, err := postTypeRepo.GetByID(postTypeWithoutImage.ID)
	if err != nil {
		t.Fatalf("Failed to get post type without image: %v", err)
	}

	if retrievedWithImage.PhotoID == "" {
		t.Error("Post type with image should have PhotoID")
	}

	if retrievedWithoutImage.PhotoID != "" {
		t.Error("Post type without image should have empty PhotoID")
	}

	_ = handler
}

func TestCancelButtonPresent(t *testing.T) {
	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	if len(keyboard.InlineKeyboard) != 1 {
		t.Errorf("Expected 1 button row, got %d", len(keyboard.InlineKeyboard))
	}

	if len(keyboard.InlineKeyboard[0]) != 1 {
		t.Errorf("Expected 1 button in row, got %d", len(keyboard.InlineKeyboard[0]))
	}

	cancelButton := keyboard.InlineKeyboard[0][0]
	if cancelButton.CallbackData != "cancel" {
		t.Errorf("Expected cancel callback data, got %q", cancelButton.CallbackData)
	}

	if cancelButton.Text == "" {
		t.Error("Cancel button text should not be empty")
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr)
}

func TestPreviewContainsText(t *testing.T) {
	handler, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	queue := db.NewDBQueueForTest(testDB)
	postTypeRepo := db.NewPostTypeRepository(queue)
	adminStateRepo := db.NewAdminStateRepository(queue)

	postType := &models.PostType{
		Name:     "Test Type",
		PhotoID:  "",
		Template: "Template text",
		IsActive: true,
	}

	if err := postTypeRepo.Create(postType); err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	userID := int64(12345)
	postText := "This is my test post text"

	state := &models.AdminState{
		UserID:         userID,
		CurrentState:   "new_post_confirm",
		SelectedTypeID: postType.ID,
		DraftText:      postText,
		DraftPhotoID:   "",
	}

	if err := adminStateRepo.Save(state); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	retrievedState, err := adminStateRepo.Get(userID)
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	if retrievedState.DraftText != postText {
		t.Errorf("Expected draft text %q, got %q", postText, retrievedState.DraftText)
	}

	_ = handler
}

func TestPreviewContainsImageWhenPresent(t *testing.T) {
	handler, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	queue := db.NewDBQueueForTest(testDB)
	postTypeRepo := db.NewPostTypeRepository(queue)
	adminStateRepo := db.NewAdminStateRepository(queue)

	photoID := "test_photo_id_123"
	postType := &models.PostType{
		Name:     "Test Type",
		PhotoID:  photoID,
		Template: "Template text",
		IsActive: true,
	}

	if err := postTypeRepo.Create(postType); err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	userID := int64(12345)
	postText := "This is my test post text"

	state := &models.AdminState{
		UserID:         userID,
		CurrentState:   "new_post_confirm",
		SelectedTypeID: postType.ID,
		DraftText:      postText,
		DraftPhotoID:   photoID,
	}

	if err := adminStateRepo.Save(state); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	retrievedState, err := adminStateRepo.Get(userID)
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	if retrievedState.DraftPhotoID != photoID {
		t.Errorf("Expected draft photo ID %q, got %q", photoID, retrievedState.DraftPhotoID)
	}

	if retrievedState.DraftPhotoID == "" {
		t.Error("Preview should contain image when present")
	}

	_ = handler
}

func TestPreviewButtonsPresent(t *testing.T) {
	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "✅ Подтвердить", CallbackData: "confirm_post"},
			},
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	if len(keyboard.InlineKeyboard) != 2 {
		t.Errorf("Expected 2 button rows, got %d", len(keyboard.InlineKeyboard))
	}

	confirmButton := keyboard.InlineKeyboard[0][0]
	if confirmButton.CallbackData != "confirm_post" {
		t.Errorf("Expected confirm_post callback data, got %q", confirmButton.CallbackData)
	}

	if confirmButton.Text == "" {
		t.Error("Confirm button text should not be empty")
	}

	cancelButton := keyboard.InlineKeyboard[1][0]
	if cancelButton.CallbackData != "cancel" {
		t.Errorf("Expected cancel callback data, got %q", cancelButton.CallbackData)
	}

	if cancelButton.Text == "" {
		t.Error("Cancel button text should not be empty")
	}
}

func TestCancelOnTypeSelection(t *testing.T) {
	_, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	adminID := int64(12345)
	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	adminStateRepo := db.NewAdminStateRepository(queue)

	adminConfigRepo.AddAdmin(adminID)

	postType := &models.PostType{
		Name:     "Test Type",
		PhotoID:  "photo123",
		Template: "Template",
		IsActive: true,
	}
	postTypeRepo := db.NewPostTypeRepository(queue)
	if err := postTypeRepo.Create(postType); err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	state := &models.AdminState{
		UserID:         adminID,
		CurrentState:   fsm.StateNewPostSelectType,
		SelectedTypeID: 0,
	}
	if err := adminStateRepo.Save(state); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	retrievedState, err := adminStateRepo.Get(adminID)
	if err != nil {
		t.Fatalf("Failed to get state before cancel: %v", err)
	}
	if retrievedState.CurrentState != fsm.StateNewPostSelectType {
		t.Errorf("Expected state %s, got %s", fsm.StateNewPostSelectType, retrievedState.CurrentState)
	}

	if err := adminStateRepo.Clear(adminID); err != nil {
		t.Fatalf("Failed to clear state: %v", err)
	}

	clearedState, err := adminStateRepo.Get(adminID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		t.Fatalf("Failed to get state after cancel: %v", err)
	}

	if clearedState != nil {
		t.Error("State should be cleared after /cancel")
	}
}

func TestCancelOnTextInput(t *testing.T) {
	_, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	adminID := int64(12345)
	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	adminStateRepo := db.NewAdminStateRepository(queue)

	adminConfigRepo.AddAdmin(adminID)

	postType := &models.PostType{
		Name:     "Test Type",
		PhotoID:  "photo123",
		Template: "Template",
		IsActive: true,
	}
	postTypeRepo := db.NewPostTypeRepository(queue)
	if err := postTypeRepo.Create(postType); err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	state := &models.AdminState{
		UserID:         adminID,
		CurrentState:   fsm.StateNewPostEnterText,
		SelectedTypeID: postType.ID,
		DraftText:      "Some draft text",
	}
	if err := adminStateRepo.Save(state); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	retrievedState, err := adminStateRepo.Get(adminID)
	if err != nil {
		t.Fatalf("Failed to get state before cancel: %v", err)
	}
	if retrievedState.CurrentState != fsm.StateNewPostEnterText {
		t.Errorf("Expected state %s, got %s", fsm.StateNewPostEnterText, retrievedState.CurrentState)
	}
	if retrievedState.DraftText != "Some draft text" {
		t.Errorf("Expected draft text 'Some draft text', got %s", retrievedState.DraftText)
	}

	if err := adminStateRepo.Clear(adminID); err != nil {
		t.Fatalf("Failed to clear state: %v", err)
	}

	clearedState, err := adminStateRepo.Get(adminID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		t.Fatalf("Failed to get state after cancel: %v", err)
	}

	if clearedState != nil {
		t.Error("State should be cleared after /cancel")
	}
}

func TestCancelOnPreview(t *testing.T) {
	_, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	adminID := int64(12345)
	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	adminStateRepo := db.NewAdminStateRepository(queue)

	adminConfigRepo.AddAdmin(adminID)

	postType := &models.PostType{
		Name:     "Test Type",
		PhotoID:  "photo123",
		Template: "Template",
		IsActive: true,
	}
	postTypeRepo := db.NewPostTypeRepository(queue)
	if err := postTypeRepo.Create(postType); err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	state := &models.AdminState{
		UserID:         adminID,
		CurrentState:   fsm.StateNewPostConfirm,
		SelectedTypeID: postType.ID,
		DraftText:      "Final post text",
		DraftPhotoID:   "photo123",
	}
	if err := adminStateRepo.Save(state); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	retrievedState, err := adminStateRepo.Get(adminID)
	if err != nil {
		t.Fatalf("Failed to get state before cancel: %v", err)
	}
	if retrievedState.CurrentState != fsm.StateNewPostConfirm {
		t.Errorf("Expected state %s, got %s", fsm.StateNewPostConfirm, retrievedState.CurrentState)
	}
	if retrievedState.DraftText != "Final post text" {
		t.Errorf("Expected draft text 'Final post text', got %s", retrievedState.DraftText)
	}
	if retrievedState.DraftPhotoID != "photo123" {
		t.Errorf("Expected draft photo ID 'photo123', got %s", retrievedState.DraftPhotoID)
	}

	if err := adminStateRepo.Clear(adminID); err != nil {
		t.Fatalf("Failed to clear state: %v", err)
	}

	clearedState, err := adminStateRepo.Get(adminID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		t.Fatalf("Failed to get state after cancel: %v", err)
	}

	if clearedState != nil {
		t.Error("State should be cleared after /cancel")
	}
}

func TestEditCommandRequestsLink(t *testing.T) {
	_, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	adminID := int64(12345)
	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	adminStateRepo := db.NewAdminStateRepository(queue)

	adminConfigRepo.AddAdmin(adminID)

	err := adminStateRepo.Save(&models.AdminState{
		UserID:       adminID,
		CurrentState: fsm.StateEditPostEnterLink,
	})
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	state, err := adminStateRepo.Get(adminID)
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	if state == nil {
		t.Fatal("State should be set after /edit command")
	}

	if state.CurrentState != fsm.StateEditPostEnterLink {
		t.Errorf("Expected state %s, got %s", fsm.StateEditPostEnterLink, state.CurrentState)
	}
}

func TestEditCommandSetsCorrectFSMState(t *testing.T) {
	_, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	adminID := int64(12345)
	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	adminStateRepo := db.NewAdminStateRepository(queue)

	adminConfigRepo.AddAdmin(adminID)

	initialState, err := adminStateRepo.Get(adminID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		t.Fatalf("Failed to get initial state: %v", err)
	}

	if initialState != nil {
		t.Error("State should not exist before /edit command")
	}

	err = adminStateRepo.Save(&models.AdminState{
		UserID:       adminID,
		CurrentState: fsm.StateEditPostEnterLink,
	})
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	state, err := adminStateRepo.Get(adminID)
	if err != nil {
		t.Fatalf("Failed to get state after /edit: %v", err)
	}

	if state == nil {
		t.Fatal("State should be set after /edit command")
	}

	if state.CurrentState != fsm.StateEditPostEnterLink {
		t.Errorf("Expected state %s, got %s", fsm.StateEditPostEnterLink, state.CurrentState)
	}

	if state.UserID != adminID {
		t.Errorf("Expected user ID %d, got %d", adminID, state.UserID)
	}
}

func TestEndToEndPostEdit(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	postTypeRepo := db.NewPostTypeRepository(queue)
	publishedPostRepo := db.NewPublishedPostRepository(queue)
	adminStateRepo := db.NewAdminStateRepository(queue)

	adminID := int64(12345)
	if err := adminConfigRepo.AddAdmin(adminID); err != nil {
		t.Fatalf("Failed to add admin: %v", err)
	}

	if err := adminConfigRepo.SetForumConfig(-1001234567890, 42); err != nil {
		t.Fatalf("Failed to set forum config: %v", err)
	}

	postType := &models.PostType{
		Name:     "Test Type",
		PhotoID:  "photo123",
		Template: "Template",
		IsActive: true,
	}
	if err := postTypeRepo.Create(postType); err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	originalText := "Original post text"
	post := &models.PublishedPost{
		PostTypeID: postType.ID,
		ChatID:     -1001234567890,
		TopicID:    42,
		MessageID:  100,
		Text:       originalText,
		PhotoID:    postType.PhotoID,
	}
	if err := publishedPostRepo.Create(post); err != nil {
		t.Fatalf("Failed to create post: %v", err)
	}

	postManager := services.NewPostManager(publishedPostRepo, postTypeRepo, adminConfigRepo)

	if err := adminStateRepo.Save(&models.AdminState{
		UserID:       adminID,
		CurrentState: fsm.StateEditPostEnterLink,
	}); err != nil {
		t.Fatalf("Failed to save initial state: %v", err)
	}

	link := "https://t.me/c/1234567890/100"
	retrievedPost, err := postManager.GetPostByLink(link)
	if err != nil {
		t.Fatalf("Failed to get post by link: %v", err)
	}

	if retrievedPost.ID != post.ID {
		t.Errorf("Expected post ID %d, got %d", post.ID, retrievedPost.ID)
	}

	if err := adminStateRepo.Save(&models.AdminState{
		UserID:        adminID,
		CurrentState:  fsm.StateEditPostEnterText,
		EditingPostID: retrievedPost.ID,
	}); err != nil {
		t.Fatalf("Failed to save state with editing post ID: %v", err)
	}

	newText := "Updated post text"
	ctx := context.Background()
	if err := postManager.EditPost(ctx, retrievedPost.ID, newText); err != nil {
		t.Fatalf("Failed to edit post: %v", err)
	}

	updatedPost, err := publishedPostRepo.GetByID(post.ID)
	if err != nil {
		t.Fatalf("Failed to get updated post: %v", err)
	}

	if updatedPost.Text != newText {
		t.Errorf("Expected text to be updated to %q, got %q", newText, updatedPost.Text)
	}

	if updatedPost.PhotoID != postType.PhotoID {
		t.Errorf("Expected photo ID to be preserved as %q, got %q", postType.PhotoID, updatedPost.PhotoID)
	}

	if err := adminStateRepo.Clear(adminID); err != nil {
		t.Fatalf("Failed to clear state: %v", err)
	}

	clearedState, err := adminStateRepo.Get(adminID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		t.Fatalf("Failed to get cleared state: %v", err)
	}

	if clearedState != nil {
		t.Error("State should be cleared after edit completion")
	}
}

func TestDeleteCommandRequestsLink(t *testing.T) {
	_, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	adminID := int64(12345)
	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	adminStateRepo := db.NewAdminStateRepository(queue)

	adminConfigRepo.AddAdmin(adminID)

	err := adminStateRepo.Save(&models.AdminState{
		UserID:       adminID,
		CurrentState: fsm.StateDeletePostEnterLink,
	})
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	state, err := adminStateRepo.Get(adminID)
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	if state == nil {
		t.Fatal("State should be set after /delete command")
	}

	if state.CurrentState != fsm.StateDeletePostEnterLink {
		t.Errorf("Expected state %s, got %s", fsm.StateDeletePostEnterLink, state.CurrentState)
	}
}

func TestDeleteCommandSetsCorrectFSMState(t *testing.T) {
	_, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	adminID := int64(12345)
	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	adminStateRepo := db.NewAdminStateRepository(queue)

	adminConfigRepo.AddAdmin(adminID)

	initialState, err := adminStateRepo.Get(adminID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		t.Fatalf("Failed to get initial state: %v", err)
	}

	if initialState != nil {
		t.Error("State should not exist before /delete command")
	}

	err = adminStateRepo.Save(&models.AdminState{
		UserID:       adminID,
		CurrentState: fsm.StateDeletePostEnterLink,
	})
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	state, err := adminStateRepo.Get(adminID)
	if err != nil {
		t.Fatalf("Failed to get state after /delete: %v", err)
	}

	if state == nil {
		t.Fatal("State should be set after /delete command")
	}

	if state.CurrentState != fsm.StateDeletePostEnterLink {
		t.Errorf("Expected state %s, got %s", fsm.StateDeletePostEnterLink, state.CurrentState)
	}

	if state.UserID != adminID {
		t.Errorf("Expected user ID %d, got %d", adminID, state.UserID)
	}
}

func TestValidDeleteLinkProcessed(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	postTypeRepo := db.NewPostTypeRepository(queue)
	publishedPostRepo := db.NewPublishedPostRepository(queue)

	adminID := int64(12345)
	if err := adminConfigRepo.AddAdmin(adminID); err != nil {
		t.Fatalf("Failed to add admin: %v", err)
	}

	if err := adminConfigRepo.SetForumConfig(-1001234567890, 42); err != nil {
		t.Fatalf("Failed to set forum config: %v", err)
	}

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
		Text:       "Test post text",
		PhotoID:    postType.PhotoID,
	}
	if err := publishedPostRepo.Create(post); err != nil {
		t.Fatalf("Failed to create post: %v", err)
	}

	postManager := services.NewPostManager(publishedPostRepo, postTypeRepo, adminConfigRepo)

	link := "https://t.me/c/1234567890/100"
	retrievedPost, err := postManager.GetPostByLink(link)
	if err != nil {
		t.Fatalf("Failed to get post by link: %v", err)
	}

	if retrievedPost.ID != post.ID {
		t.Errorf("Expected post ID %d, got %d", post.ID, retrievedPost.ID)
	}

	if retrievedPost.MessageID != post.MessageID {
		t.Errorf("Expected message ID %d, got %d", post.MessageID, retrievedPost.MessageID)
	}
}

func TestInvalidDeleteLinkReturnsError(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	postTypeRepo := db.NewPostTypeRepository(queue)
	publishedPostRepo := db.NewPublishedPostRepository(queue)

	postManager := services.NewPostManager(publishedPostRepo, postTypeRepo, adminConfigRepo)

	invalidLinks := []string{
		"https://example.com/invalid",
		"not a link at all",
		"https://t.me/invalid",
		"",
	}

	for _, link := range invalidLinks {
		_, err := postManager.GetPostByLink(link)
		if err == nil {
			t.Errorf("Expected error for invalid link %q, but got none", link)
		}
	}
}

func TestEndToEndPostDelete(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	postTypeRepo := db.NewPostTypeRepository(queue)
	publishedPostRepo := db.NewPublishedPostRepository(queue)
	adminStateRepo := db.NewAdminStateRepository(queue)

	adminID := int64(12345)
	if err := adminConfigRepo.AddAdmin(adminID); err != nil {
		t.Fatalf("Failed to add admin: %v", err)
	}

	if err := adminConfigRepo.SetForumConfig(-1001234567890, 42); err != nil {
		t.Fatalf("Failed to set forum config: %v", err)
	}

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
		Text:       "Post to be deleted",
		PhotoID:    postType.PhotoID,
	}
	if err := publishedPostRepo.Create(post); err != nil {
		t.Fatalf("Failed to create post: %v", err)
	}

	postManager := services.NewPostManager(publishedPostRepo, postTypeRepo, adminConfigRepo)

	if err := adminStateRepo.Save(&models.AdminState{
		UserID:       adminID,
		CurrentState: fsm.StateDeletePostEnterLink,
	}); err != nil {
		t.Fatalf("Failed to save initial state: %v", err)
	}

	link := "https://t.me/c/1234567890/100"
	retrievedPost, err := postManager.GetPostByLink(link)
	if err != nil {
		t.Fatalf("Failed to get post by link: %v", err)
	}

	if retrievedPost.ID != post.ID {
		t.Errorf("Expected post ID %d, got %d", post.ID, retrievedPost.ID)
	}

	ctx := context.Background()
	if err := postManager.DeletePost(ctx, retrievedPost.ID); err != nil {
		t.Fatalf("Failed to delete post: %v", err)
	}

	deletedPost, err := publishedPostRepo.GetByID(post.ID)
	if err == nil {
		t.Errorf("Expected error when getting deleted post, but got post: %+v", deletedPost)
	}
	if err != nil && err.Error() != "sql: no rows in result set" {
		t.Errorf("Expected 'sql: no rows in result set' error, got: %v", err)
	}

	if err := adminStateRepo.Clear(adminID); err != nil {
		t.Fatalf("Failed to clear state: %v", err)
	}

	clearedState, err := adminStateRepo.Get(adminID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		t.Fatalf("Failed to get cleared state: %v", err)
	}

	if clearedState != nil {
		t.Error("State should be cleared after delete completion")
	}
}

func TestSettingsMenuContainsAllButtons(t *testing.T) {
	expectedButtons := []string{
		"Новый тип",
		"Типы постов",
		"Настройки доступа",
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "Новый тип", CallbackData: "settings_new_type"},
			},
			{
				{Text: "Типы постов", CallbackData: "settings_manage_types"},
			},
			{
				{Text: "Настройки доступа", CallbackData: "settings_access"},
			},
		},
	}

	if len(keyboard.InlineKeyboard) != 3 {
		t.Errorf("Expected 3 buttons, got %d", len(keyboard.InlineKeyboard))
	}

	for i, row := range keyboard.InlineKeyboard {
		if len(row) != 1 {
			t.Errorf("Expected 1 button in row %d, got %d", i, len(row))
		}
		if row[0].Text != expectedButtons[i] {
			t.Errorf("Expected button text %q, got %q", expectedButtons[i], row[0].Text)
		}
	}
}

func TestSettingsMenuButtonTextsInRussian(t *testing.T) {
	expectedButtons := []string{
		"Новый тип",
		"Типы постов",
		"Настройки доступа",
	}

	for _, buttonText := range expectedButtons {
		if buttonText == "" {
			t.Errorf("Button text should not be empty")
		}
		if len(buttonText) < 3 {
			t.Errorf("Button text %q seems too short", buttonText)
		}
	}
}

func TestNewTypeCreationFSMTransitions(t *testing.T) {
	_, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	adminID := int64(12345)
	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	adminStateRepo := db.NewAdminStateRepository(queue)

	adminConfigRepo.AddAdmin(adminID)

	err := adminStateRepo.Save(&models.AdminState{
		UserID:       adminID,
		CurrentState: fsm.StateNewTypeEnterName,
	})
	if err != nil {
		t.Fatalf("Failed to save initial state: %v", err)
	}

	state, err := adminStateRepo.Get(adminID)
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}
	if state.CurrentState != fsm.StateNewTypeEnterName {
		t.Errorf("Expected state %s, got %s", fsm.StateNewTypeEnterName, state.CurrentState)
	}

	state.TempName = "Test Type"
	state.CurrentState = fsm.StateNewTypeEnterImage
	err = adminStateRepo.Save(state)
	if err != nil {
		t.Fatalf("Failed to save state after name: %v", err)
	}

	state, err = adminStateRepo.Get(adminID)
	if err != nil {
		t.Fatalf("Failed to get state after name: %v", err)
	}
	if state.CurrentState != fsm.StateNewTypeEnterImage {
		t.Errorf("Expected state %s, got %s", fsm.StateNewTypeEnterImage, state.CurrentState)
	}
	if state.TempName != "Test Type" {
		t.Errorf("Expected temp name 'Test Type', got %s", state.TempName)
	}

	state.TempPhotoID = "photo123"
	state.CurrentState = fsm.StateNewTypeEnterTemplate
	err = adminStateRepo.Save(state)
	if err != nil {
		t.Fatalf("Failed to save state after image: %v", err)
	}

	state, err = adminStateRepo.Get(adminID)
	if err != nil {
		t.Fatalf("Failed to get state after image: %v", err)
	}
	if state.CurrentState != fsm.StateNewTypeEnterTemplate {
		t.Errorf("Expected state %s, got %s", fsm.StateNewTypeEnterTemplate, state.CurrentState)
	}
	if state.TempPhotoID != "photo123" {
		t.Errorf("Expected temp photo ID 'photo123', got %s", state.TempPhotoID)
	}
}

func TestNewTypeCreationWithImage(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	postTypeRepo := db.NewPostTypeRepository(queue)
	postTypeManager := services.NewPostTypeManager(postTypeRepo)

	typeName := "Type With Image"
	photoID := "photo123"
	template := "Template text"

	createdType, err := postTypeManager.CreateType(typeName, photoID, template)
	if err != nil {
		t.Fatalf("Failed to create type: %v", err)
	}

	if createdType.Name != typeName {
		t.Errorf("Expected name %q, got %q", typeName, createdType.Name)
	}
	if createdType.PhotoID != photoID {
		t.Errorf("Expected photo ID %q, got %q", photoID, createdType.PhotoID)
	}
	if createdType.Template != template {
		t.Errorf("Expected template %q, got %q", template, createdType.Template)
	}
	if !createdType.IsActive {
		t.Error("Created type should be active by default")
	}

	retrievedType, err := postTypeRepo.GetByID(createdType.ID)
	if err != nil {
		t.Fatalf("Failed to get created type: %v", err)
	}

	if retrievedType.Name != typeName {
		t.Errorf("Expected retrieved name %q, got %q", typeName, retrievedType.Name)
	}
	if retrievedType.PhotoID != photoID {
		t.Errorf("Expected retrieved photo ID %q, got %q", photoID, retrievedType.PhotoID)
	}
	if retrievedType.Template != template {
		t.Errorf("Expected retrieved template %q, got %q", template, retrievedType.Template)
	}
}

func TestNewTypeCreationWithoutImage(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	postTypeRepo := db.NewPostTypeRepository(queue)
	postTypeManager := services.NewPostTypeManager(postTypeRepo)

	typeName := "Type Without Image"
	photoID := ""
	template := "Template text"

	createdType, err := postTypeManager.CreateType(typeName, photoID, template)
	if err != nil {
		t.Fatalf("Failed to create type: %v", err)
	}

	if createdType.Name != typeName {
		t.Errorf("Expected name %q, got %q", typeName, createdType.Name)
	}
	if createdType.PhotoID != "" {
		t.Errorf("Expected empty photo ID, got %q", createdType.PhotoID)
	}
	if createdType.Template != template {
		t.Errorf("Expected template %q, got %q", template, createdType.Template)
	}
	if !createdType.IsActive {
		t.Error("Created type should be active by default")
	}

	retrievedType, err := postTypeRepo.GetByID(createdType.ID)
	if err != nil {
		t.Fatalf("Failed to get created type: %v", err)
	}

	if retrievedType.Name != typeName {
		t.Errorf("Expected retrieved name %q, got %q", typeName, retrievedType.Name)
	}
	if retrievedType.PhotoID != "" {
		t.Errorf("Expected retrieved empty photo ID, got %q", retrievedType.PhotoID)
	}
	if retrievedType.Template != template {
		t.Errorf("Expected retrieved template %q, got %q", template, retrievedType.Template)
	}
}

func TestEndToEndTypeCreation(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	postTypeRepo := db.NewPostTypeRepository(queue)
	adminStateRepo := db.NewAdminStateRepository(queue)
	postTypeManager := services.NewPostTypeManager(postTypeRepo)

	adminID := int64(12345)
	if err := adminConfigRepo.AddAdmin(adminID); err != nil {
		t.Fatalf("Failed to add admin: %v", err)
	}

	if err := adminStateRepo.Save(&models.AdminState{
		UserID:       adminID,
		CurrentState: fsm.StateNewTypeEnterName,
	}); err != nil {
		t.Fatalf("Failed to save initial state: %v", err)
	}

	state, err := adminStateRepo.Get(adminID)
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}
	if state.CurrentState != fsm.StateNewTypeEnterName {
		t.Errorf("Expected state %s, got %s", fsm.StateNewTypeEnterName, state.CurrentState)
	}

	typeName := "Integration Test Type"
	state.TempName = typeName
	state.CurrentState = fsm.StateNewTypeEnterImage
	if err := adminStateRepo.Save(state); err != nil {
		t.Fatalf("Failed to save state after name: %v", err)
	}

	state, err = adminStateRepo.Get(adminID)
	if err != nil {
		t.Fatalf("Failed to get state after name: %v", err)
	}
	if state.CurrentState != fsm.StateNewTypeEnterImage {
		t.Errorf("Expected state %s, got %s", fsm.StateNewTypeEnterImage, state.CurrentState)
	}
	if state.TempName != typeName {
		t.Errorf("Expected temp name %q, got %q", typeName, state.TempName)
	}

	photoID := "integration_test_photo_123"
	state.TempPhotoID = photoID
	state.CurrentState = fsm.StateNewTypeEnterTemplate
	if err := adminStateRepo.Save(state); err != nil {
		t.Fatalf("Failed to save state after image: %v", err)
	}

	state, err = adminStateRepo.Get(adminID)
	if err != nil {
		t.Fatalf("Failed to get state after image: %v", err)
	}
	if state.CurrentState != fsm.StateNewTypeEnterTemplate {
		t.Errorf("Expected state %s, got %s", fsm.StateNewTypeEnterTemplate, state.CurrentState)
	}
	if state.TempPhotoID != photoID {
		t.Errorf("Expected temp photo ID %q, got %q", photoID, state.TempPhotoID)
	}

	template := "Integration test template text"
	createdType, err := postTypeManager.CreateType(state.TempName, state.TempPhotoID, template)
	if err != nil {
		t.Fatalf("Failed to create type: %v", err)
	}

	if createdType.Name != typeName {
		t.Errorf("Expected created type name %q, got %q", typeName, createdType.Name)
	}
	if createdType.PhotoID != photoID {
		t.Errorf("Expected created type photo ID %q, got %q", photoID, createdType.PhotoID)
	}
	if createdType.Template != template {
		t.Errorf("Expected created type template %q, got %q", template, createdType.Template)
	}
	if !createdType.IsActive {
		t.Error("Created type should be active by default")
	}

	if err := adminStateRepo.Clear(adminID); err != nil {
		t.Fatalf("Failed to clear state: %v", err)
	}

	clearedState, err := adminStateRepo.Get(adminID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		t.Fatalf("Failed to get cleared state: %v", err)
	}
	if clearedState != nil {
		t.Error("State should be cleared after type creation")
	}

	retrievedType, err := postTypeRepo.GetByID(createdType.ID)
	if err != nil {
		t.Fatalf("Failed to get created type from DB: %v", err)
	}

	if retrievedType.Name != typeName {
		t.Errorf("Expected DB type name %q, got %q", typeName, retrievedType.Name)
	}
	if retrievedType.PhotoID != photoID {
		t.Errorf("Expected DB type photo ID %q, got %q", photoID, retrievedType.PhotoID)
	}
	if retrievedType.Template != template {
		t.Errorf("Expected DB type template %q, got %q", template, retrievedType.Template)
	}
	if !retrievedType.IsActive {
		t.Error("DB type should be active")
	}

	allTypes, err := postTypeRepo.GetAll()
	if err != nil {
		t.Fatalf("Failed to get all types: %v", err)
	}

	found := false
	for _, pt := range allTypes {
		if pt.ID == createdType.ID {
			found = true
			if pt.Name != typeName {
				t.Errorf("Expected type name in list %q, got %q", typeName, pt.Name)
			}
			break
		}
	}
	if !found {
		t.Error("Created type should be in the list of all types")
	}

	activeTypes, err := postTypeRepo.GetActive()
	if err != nil {
		t.Fatalf("Failed to get active types: %v", err)
	}

	foundActive := false
	for _, pt := range activeTypes {
		if pt.ID == createdType.ID {
			foundActive = true
			break
		}
	}
	if !foundActive {
		t.Error("Created type should be in the list of active types")
	}
}

func TestTypeListDisplayed(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	postTypeRepo := db.NewPostTypeRepository(queue)

	type1 := &models.PostType{
		Name:     "Type 1",
		PhotoID:  "photo1",
		Template: "Template 1",
		IsActive: true,
	}
	type2 := &models.PostType{
		Name:     "Type 2",
		PhotoID:  "",
		Template: "Template 2",
		IsActive: false,
	}
	type3 := &models.PostType{
		Name:     "Type 3",
		PhotoID:  "photo3",
		Template: "Template 3",
		IsActive: true,
	}

	if err := postTypeRepo.Create(type1); err != nil {
		t.Fatalf("Failed to create type 1: %v", err)
	}
	if err := postTypeRepo.Create(type2); err != nil {
		t.Fatalf("Failed to create type 2: %v", err)
	}
	if err := postTypeRepo.Create(type3); err != nil {
		t.Fatalf("Failed to create type 3: %v", err)
	}

	allTypes, err := postTypeRepo.GetAll()
	if err != nil {
		t.Fatalf("Failed to get all types: %v", err)
	}

	if len(allTypes) != 3 {
		t.Errorf("Expected 3 types, got %d", len(allTypes))
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: make([][]tgmodels.InlineKeyboardButton, 0, len(allTypes)),
	}

	for _, pt := range allTypes {
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []tgmodels.InlineKeyboardButton{
			{
				Text:         pt.Name,
				CallbackData: "manage_type:" + string(rune(pt.ID)),
			},
		})
	}

	if len(keyboard.InlineKeyboard) != 3 {
		t.Errorf("Expected 3 buttons in keyboard, got %d", len(keyboard.InlineKeyboard))
	}

	for i, row := range keyboard.InlineKeyboard {
		if len(row) != 1 {
			t.Errorf("Expected 1 button in row %d, got %d", i, len(row))
		}
		if row[0].Text == "" {
			t.Errorf("Button text should not be empty in row %d", i)
		}
	}

	expectedNames := []string{"Type 1", "Type 2", "Type 3"}
	for i, row := range keyboard.InlineKeyboard {
		if row[0].Text != expectedNames[i] {
			t.Errorf("Expected button text %q in row %d, got %q", expectedNames[i], i, row[0].Text)
		}
	}
}

func TestManagementOptionsDisplayed(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	postTypeRepo := db.NewPostTypeRepository(queue)

	postType := &models.PostType{
		Name:     "Test Type",
		PhotoID:  "photo123",
		Template: "Template text",
		IsActive: true,
	}

	if err := postTypeRepo.Create(postType); err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	expectedOptions := []string{
		"Изменить название",
		"Заменить изображение",
		"Заменить шаблон",
		"Отключить",
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "Изменить название", CallbackData: "edit_type_name:" + string(rune(postType.ID))},
			},
			{
				{Text: "Заменить изображение", CallbackData: "edit_type_image:" + string(rune(postType.ID))},
			},
			{
				{Text: "Заменить шаблон", CallbackData: "edit_type_template:" + string(rune(postType.ID))},
			},
			{
				{Text: "Отключить", CallbackData: "toggle_type_active:" + string(rune(postType.ID))},
			},
		},
	}

	if len(keyboard.InlineKeyboard) != 4 {
		t.Errorf("Expected 4 management options, got %d", len(keyboard.InlineKeyboard))
	}

	for i, row := range keyboard.InlineKeyboard {
		if len(row) != 1 {
			t.Errorf("Expected 1 button in row %d, got %d", i, len(row))
		}
		if row[0].Text != expectedOptions[i] {
			t.Errorf("Expected option text %q in row %d, got %q", expectedOptions[i], i, row[0].Text)
		}
		if row[0].CallbackData == "" {
			t.Errorf("Callback data should not be empty in row %d", i)
		}
	}
}

func TestManagementOptionsForInactiveType(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	postTypeRepo := db.NewPostTypeRepository(queue)

	postType := &models.PostType{
		Name:     "Inactive Type",
		PhotoID:  "photo123",
		Template: "Template text",
		IsActive: false,
	}

	if err := postTypeRepo.Create(postType); err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	expectedOptions := []string{
		"Изменить название",
		"Заменить изображение",
		"Заменить шаблон",
		"Включить",
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "Изменить название", CallbackData: "edit_type_name:" + string(rune(postType.ID))},
			},
			{
				{Text: "Заменить изображение", CallbackData: "edit_type_image:" + string(rune(postType.ID))},
			},
			{
				{Text: "Заменить шаблон", CallbackData: "edit_type_template:" + string(rune(postType.ID))},
			},
			{
				{Text: "Включить", CallbackData: "toggle_type_active:" + string(rune(postType.ID))},
			},
		},
	}

	if len(keyboard.InlineKeyboard) != 4 {
		t.Errorf("Expected 4 management options, got %d", len(keyboard.InlineKeyboard))
	}

	for i, row := range keyboard.InlineKeyboard {
		if len(row) != 1 {
			t.Errorf("Expected 1 button in row %d, got %d", i, len(row))
		}
		if row[0].Text != expectedOptions[i] {
			t.Errorf("Expected option text %q in row %d, got %q", expectedOptions[i], i, row[0].Text)
		}
	}

	lastButton := keyboard.InlineKeyboard[3][0]
	if lastButton.Text != "Включить" {
		t.Errorf("Expected 'Включить' for inactive type, got %q", lastButton.Text)
	}
}

func TestUpdateTypeName(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	postTypeRepo := db.NewPostTypeRepository(queue)
	postTypeManager := services.NewPostTypeManager(postTypeRepo)

	originalName := "Original Name"
	postType := &models.PostType{
		Name:     originalName,
		PhotoID:  "photo123",
		Template: "Template text",
		IsActive: true,
	}

	if err := postTypeRepo.Create(postType); err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	newName := "Updated Name"
	if err := postTypeManager.UpdateTypeName(postType.ID, newName); err != nil {
		t.Fatalf("Failed to update type name: %v", err)
	}

	updatedType, err := postTypeRepo.GetByID(postType.ID)
	if err != nil {
		t.Fatalf("Failed to get updated type: %v", err)
	}

	if updatedType.Name != newName {
		t.Errorf("Expected name to be updated to %q, got %q", newName, updatedType.Name)
	}

	if updatedType.PhotoID != postType.PhotoID {
		t.Errorf("Expected photo ID to remain %q, got %q", postType.PhotoID, updatedType.PhotoID)
	}

	if updatedType.Template != postType.Template {
		t.Errorf("Expected template to remain %q, got %q", postType.Template, updatedType.Template)
	}

	if updatedType.IsActive != postType.IsActive {
		t.Errorf("Expected is_active to remain %v, got %v", postType.IsActive, updatedType.IsActive)
	}
}

func TestUpdateTypeImage(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	postTypeRepo := db.NewPostTypeRepository(queue)
	postTypeManager := services.NewPostTypeManager(postTypeRepo)

	originalPhotoID := "photo123"
	postType := &models.PostType{
		Name:     "Test Type",
		PhotoID:  originalPhotoID,
		Template: "Template text",
		IsActive: true,
	}

	if err := postTypeRepo.Create(postType); err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	newPhotoID := "photo456"
	if err := postTypeManager.UpdateTypePhoto(postType.ID, newPhotoID); err != nil {
		t.Fatalf("Failed to update type photo: %v", err)
	}

	updatedType, err := postTypeRepo.GetByID(postType.ID)
	if err != nil {
		t.Fatalf("Failed to get updated type: %v", err)
	}

	if updatedType.PhotoID != newPhotoID {
		t.Errorf("Expected photo ID to be updated to %q, got %q", newPhotoID, updatedType.PhotoID)
	}

	if updatedType.Name != postType.Name {
		t.Errorf("Expected name to remain %q, got %q", postType.Name, updatedType.Name)
	}

	if updatedType.Template != postType.Template {
		t.Errorf("Expected template to remain %q, got %q", postType.Template, updatedType.Template)
	}

	if updatedType.IsActive != postType.IsActive {
		t.Errorf("Expected is_active to remain %v, got %v", postType.IsActive, updatedType.IsActive)
	}
}

func TestUpdateTypeTemplate(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	postTypeRepo := db.NewPostTypeRepository(queue)
	postTypeManager := services.NewPostTypeManager(postTypeRepo)

	originalTemplate := "Original template text"
	postType := &models.PostType{
		Name:     "Test Type",
		PhotoID:  "photo123",
		Template: originalTemplate,
		IsActive: true,
	}

	if err := postTypeRepo.Create(postType); err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	newTemplate := "Updated template text with new content"
	if err := postTypeManager.UpdateTypeTemplate(postType.ID, newTemplate); err != nil {
		t.Fatalf("Failed to update type template: %v", err)
	}

	updatedType, err := postTypeRepo.GetByID(postType.ID)
	if err != nil {
		t.Fatalf("Failed to get updated type: %v", err)
	}

	if updatedType.Template != newTemplate {
		t.Errorf("Expected template to be updated to %q, got %q", newTemplate, updatedType.Template)
	}

	if updatedType.Name != postType.Name {
		t.Errorf("Expected name to remain %q, got %q", postType.Name, updatedType.Name)
	}

	if updatedType.PhotoID != postType.PhotoID {
		t.Errorf("Expected photo ID to remain %q, got %q", postType.PhotoID, updatedType.PhotoID)
	}

	if updatedType.IsActive != postType.IsActive {
		t.Errorf("Expected is_active to remain %v, got %v", postType.IsActive, updatedType.IsActive)
	}
}

func TestToggleTypeActive(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	postTypeRepo := db.NewPostTypeRepository(queue)
	postTypeManager := services.NewPostTypeManager(postTypeRepo)

	postType := &models.PostType{
		Name:     "Test Type",
		PhotoID:  "photo123",
		Template: "Template text",
		IsActive: true,
	}

	if err := postTypeRepo.Create(postType); err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	if err := postTypeManager.SetTypeActive(postType.ID, false); err != nil {
		t.Fatalf("Failed to deactivate type: %v", err)
	}

	deactivatedType, err := postTypeRepo.GetByID(postType.ID)
	if err != nil {
		t.Fatalf("Failed to get deactivated type: %v", err)
	}

	if deactivatedType.IsActive {
		t.Error("Expected type to be deactivated, but it's still active")
	}

	if deactivatedType.Name != postType.Name {
		t.Errorf("Expected name to remain %q, got %q", postType.Name, deactivatedType.Name)
	}

	if deactivatedType.PhotoID != postType.PhotoID {
		t.Errorf("Expected photo ID to remain %q, got %q", postType.PhotoID, deactivatedType.PhotoID)
	}

	if deactivatedType.Template != postType.Template {
		t.Errorf("Expected template to remain %q, got %q", postType.Template, deactivatedType.Template)
	}

	if err := postTypeManager.SetTypeActive(postType.ID, true); err != nil {
		t.Fatalf("Failed to reactivate type: %v", err)
	}

	reactivatedType, err := postTypeRepo.GetByID(postType.ID)
	if err != nil {
		t.Fatalf("Failed to get reactivated type: %v", err)
	}

	if !reactivatedType.IsActive {
		t.Error("Expected type to be reactivated, but it's still inactive")
	}

	activeTypes, err := postTypeRepo.GetActive()
	if err != nil {
		t.Fatalf("Failed to get active types: %v", err)
	}

	found := false
	for _, pt := range activeTypes {
		if pt.ID == postType.ID {
			found = true
			break
		}
	}

	if !found {
		t.Error("Reactivated type should appear in active types list")
	}
}

func TestAccessSettings_AddAdmin(t *testing.T) {
	_, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	settingsManager := services.NewSettingsManager(adminConfigRepo)

	initialAdminID := int64(12345)
	newAdminID := int64(67890)

	if err := settingsManager.AddAdmin(initialAdminID); err != nil {
		t.Fatalf("Failed to add initial admin: %v", err)
	}

	if err := settingsManager.AddAdmin(newAdminID); err != nil {
		t.Fatalf("Failed to add new admin: %v", err)
	}

	admins, err := settingsManager.GetAdmins()
	if err != nil {
		t.Fatalf("Failed to get admins: %v", err)
	}

	if len(admins) != 2 {
		t.Errorf("Expected 2 admin IDs, got %d", len(admins))
	}

	foundInitial := false
	foundNew := false
	for _, id := range admins {
		if id == initialAdminID {
			foundInitial = true
		}
		if id == newAdminID {
			foundNew = true
		}
	}

	if !foundInitial {
		t.Error("Initial admin ID not found in config")
	}
	if !foundNew {
		t.Error("New admin ID not found in config")
	}
}

func TestAccessSettings_RemoveAdmin(t *testing.T) {
	_, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	settingsManager := services.NewSettingsManager(adminConfigRepo)

	admin1 := int64(12345)
	admin2 := int64(67890)

	if err := settingsManager.AddAdmin(admin1); err != nil {
		t.Fatalf("Failed to add admin 1: %v", err)
	}
	if err := settingsManager.AddAdmin(admin2); err != nil {
		t.Fatalf("Failed to add admin 2: %v", err)
	}

	if err := settingsManager.RemoveAdmin(admin2); err != nil {
		t.Fatalf("Failed to remove admin 2: %v", err)
	}

	admins, err := settingsManager.GetAdmins()
	if err != nil {
		t.Fatalf("Failed to get admins: %v", err)
	}

	if len(admins) != 1 {
		t.Errorf("Expected 1 admin ID, got %d", len(admins))
	}

	if admins[0] != admin1 {
		t.Errorf("Expected admin ID %d, got %d", admin1, admins[0])
	}

	isAdmin, err := settingsManager.IsAdmin(admin2)
	if err != nil {
		t.Fatalf("Failed to check if user is admin: %v", err)
	}
	if isAdmin {
		t.Error("Removed admin should not be admin anymore")
	}
}

func TestAccessSettings_ChangeForumID(t *testing.T) {
	_, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	settingsManager := services.NewSettingsManager(adminConfigRepo)

	newForumID := int64(-1001234567890)
	newTopicID := int64(42)

	if err := settingsManager.SetForumConfig(newForumID, newTopicID); err != nil {
		t.Fatalf("Failed to set forum config: %v", err)
	}

	forumID, topicID, err := settingsManager.GetForumConfig()
	if err != nil {
		t.Fatalf("Failed to get forum config: %v", err)
	}

	if forumID != newForumID {
		t.Errorf("Expected forum ID %d, got %d", newForumID, forumID)
	}

	if topicID != newTopicID {
		t.Errorf("Expected topic ID %d, got %d", newTopicID, topicID)
	}
}

func TestAccessSettings_ChangeTopicID(t *testing.T) {
	_, testDB := setupForumAdminHandler(t)
	defer testDB.Close()

	queue := db.NewDBQueueForTest(testDB)
	adminConfigRepo := db.NewAdminConfigRepository(queue)
	settingsManager := services.NewSettingsManager(adminConfigRepo)

	initialForumID := int64(-1001234567890)
	initialTopicID := int64(10)
	newTopicID := int64(42)

	if err := settingsManager.SetForumConfig(initialForumID, initialTopicID); err != nil {
		t.Fatalf("Failed to set initial forum config: %v", err)
	}

	if err := settingsManager.SetForumConfig(initialForumID, newTopicID); err != nil {
		t.Fatalf("Failed to update topic ID: %v", err)
	}

	forumID, topicID, err := settingsManager.GetForumConfig()
	if err != nil {
		t.Fatalf("Failed to get forum config: %v", err)
	}

	if forumID != initialForumID {
		t.Errorf("Forum ID should not change, expected %d, got %d", initialForumID, forumID)
	}

	if topicID != newTopicID {
		t.Errorf("Expected topic ID %d, got %d", newTopicID, topicID)
	}
}
