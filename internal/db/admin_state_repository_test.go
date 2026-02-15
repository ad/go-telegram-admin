package db

import (
	"database/sql"
	"testing"

	"github.com/ad/go-telegram-admin/internal/fsm"
	"github.com/ad/go-telegram-admin/internal/models"
	_ "modernc.org/sqlite"
	"pgregory.net/rapid"
)

func setupAdminStateTestDB(t *testing.T) (*sql.DB, *AdminStateRepository) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}

	if err := InitSchema(db); err != nil {
		t.Fatal(err)
	}

	queue := NewDBQueueForTest(db)
	adminStateRepo := NewAdminStateRepository(queue)

	return db, adminStateRepo
}

func TestAdminStateRepository_GetNonExistentState(t *testing.T) {
	db, repo := setupAdminStateTestDB(t)
	defer db.Close()

	_, err := repo.Get(999)
	if err == nil {
		t.Error("Expected error for non-existent state, got nil")
	}
}

func TestProperty9_CancelStateReset(t *testing.T) {
	db, repo := setupAdminStateTestDB(t)
	defer db.Close()

	rapid.Check(t, func(t *rapid.T) {
		adminUserID := rapid.Int64Range(1, 1000000).Draw(t, "adminUserID")

		states := []string{
			fsm.StateNewPostSelectType,
			fsm.StateNewPostEnterText,
			fsm.StateNewPostConfirm,
			fsm.StateEditPostEnterLink,
			fsm.StateEditPostEnterText,
			fsm.StateDeletePostEnterLink,
			fsm.StateNewTypeEnterName,
			fsm.StateNewTypeEnterImage,
			fsm.StateNewTypeEnterTemplate,
			fsm.StateManageTypes,
			fsm.StateEditTypeName,
			fsm.StateEditTypeImage,
			fsm.StateEditTypeTemplate,
		}

		currentState := rapid.SampledFrom(states).Draw(t, "currentState")
		selectedTypeID := rapid.Int64Range(0, 100).Draw(t, "selectedTypeID")
		draftText := rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s]{0,100}`).Draw(t, "draftText")
		draftPhotoID := rapid.StringMatching(`[a-zA-Z0-9_-]{0,50}`).Draw(t, "draftPhotoID")
		editingPostID := rapid.Int64Range(0, 100).Draw(t, "editingPostID")
		editingTypeID := rapid.Int64Range(0, 100).Draw(t, "editingTypeID")
		tempName := rapid.StringMatching(`[a-zA-Zа-яА-Я ]{0,50}`).Draw(t, "tempName")
		tempPhotoID := rapid.StringMatching(`[a-zA-Z0-9_-]{0,50}`).Draw(t, "tempPhotoID")
		tempTemplate := rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s]{0,100}`).Draw(t, "tempTemplate")

		state := &models.AdminState{
			UserID:         adminUserID,
			CurrentState:   currentState,
			SelectedTypeID: selectedTypeID,
			DraftText:      draftText,
			DraftPhotoID:   draftPhotoID,
			EditingPostID:  editingPostID,
			EditingTypeID:  editingTypeID,
			TempName:       tempName,
			TempPhotoID:    tempPhotoID,
			TempTemplate:   tempTemplate,
		}

		err := repo.Save(state)
		if err != nil {
			t.Fatalf("Failed to save state: %v", err)
		}

		err = repo.Clear(adminUserID)
		if err != nil {
			t.Fatalf("Failed to clear state: %v", err)
		}

		_, err = repo.Get(adminUserID)
		if err == nil {
			t.Fatal("Expected error after clearing state, got nil")
		}
	})
}
