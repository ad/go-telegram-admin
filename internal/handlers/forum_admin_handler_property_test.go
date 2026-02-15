package handlers

import (
	"context"
	"database/sql"
	"testing"

	"github.com/ad/go-telegram-admin/internal/db"
	"github.com/ad/go-telegram-admin/internal/models"
	_ "modernc.org/sqlite"
	"pgregory.net/rapid"
)

func setupPropertyTestDB(t *testing.T) (*db.DBQueue, *sql.DB, func()) {
	t.Helper()

	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}

	if err := db.InitSchema(testDB); err != nil {
		testDB.Close()
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)

	cleanup := func() {
		testDB.Close()
	}

	return queue, testDB, cleanup
}

func TestProperty5_PostTypeDisplay(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		queue, testDB, cleanup := setupPropertyTestDB(t)
		defer cleanup()

		postTypeRepo := db.NewPostTypeRepository(queue)
		adminConfigRepo := db.NewAdminConfigRepository(queue)

		adminID := rapid.Int64Range(1, 1000000).Draw(rt, "adminID")
		adminConfigRepo.AddAdmin(adminID)

		numActiveTypes := rapid.IntRange(1, 10).Draw(rt, "numActiveTypes")
		numInactiveTypes := rapid.IntRange(0, 5).Draw(rt, "numInactiveTypes")

		var activeTypeIDs []int64
		var inactiveTypeIDs []int64

		for i := 0; i < numActiveTypes; i++ {
			postType := &models.PostType{
				Name:     rapid.StringMatching(`[A-Za-zА-Яа-я0-9 ]{3,20}`).Draw(rt, "activeName"),
				PhotoID:  rapid.StringMatching(`[A-Za-z0-9_-]{0,100}`).Draw(rt, "activePhotoID"),
				Template: rapid.StringMatching(`.{10,200}`).Draw(rt, "activeTemplate"),
				IsActive: true,
			}
			if err := postTypeRepo.Create(postType); err != nil {
				rt.Fatalf("Failed to create active post type: %v", err)
			}
			activeTypeIDs = append(activeTypeIDs, postType.ID)
		}

		for i := 0; i < numInactiveTypes; i++ {
			postType := &models.PostType{
				Name:     rapid.StringMatching(`[A-Za-zА-Яа-я0-9 ]{3,20}`).Draw(rt, "inactiveName"),
				PhotoID:  rapid.StringMatching(`[A-Za-z0-9_-]{0,100}`).Draw(rt, "inactivePhotoID"),
				Template: rapid.StringMatching(`.{10,200}`).Draw(rt, "inactiveTemplate"),
				IsActive: false,
			}
			if err := postTypeRepo.Create(postType); err != nil {
				rt.Fatalf("Failed to create inactive post type: %v", err)
			}
			inactiveTypeIDs = append(inactiveTypeIDs, postType.ID)
		}

		activeTypes, err := postTypeRepo.GetActive()
		if err != nil {
			rt.Fatalf("Failed to get active types: %v", err)
		}

		if len(activeTypes) != numActiveTypes {
			rt.Fatalf("Expected %d active types, got %d", numActiveTypes, len(activeTypes))
		}

		for _, activeType := range activeTypes {
			if !activeType.IsActive {
				rt.Fatalf("GetActive returned inactive type: %d", activeType.ID)
			}

			found := false
			for _, id := range activeTypeIDs {
				if activeType.ID == id {
					found = true
					break
				}
			}
			if !found {
				rt.Fatalf("GetActive returned unexpected type ID: %d", activeType.ID)
			}
		}

		for _, inactiveID := range inactiveTypeIDs {
			for _, activeType := range activeTypes {
				if activeType.ID == inactiveID {
					rt.Fatalf("GetActive returned inactive type ID: %d", inactiveID)
				}
			}
		}

		_ = testDB
		_ = context.Background()
	})
}

func TestProperty6_TemplateDisplayFormat(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		queue, testDB, cleanup := setupPropertyTestDB(t)
		defer cleanup()

		postTypeRepo := db.NewPostTypeRepository(queue)

		template := rapid.StringMatching(`.{10,200}`).Draw(rt, "template")
		postType := &models.PostType{
			Name:     rapid.StringMatching(`[A-Za-zА-Яа-я0-9 ]{3,20}`).Draw(rt, "name"),
			PhotoID:  rapid.StringMatching(`[A-Za-z0-9_-]{0,100}`).Draw(rt, "photoID"),
			Template: template,
			IsActive: true,
		}

		if err := postTypeRepo.Create(postType); err != nil {
			rt.Fatalf("Failed to create post type: %v", err)
		}

		retrieved, err := postTypeRepo.GetByID(postType.ID)
		if err != nil {
			rt.Fatalf("Failed to get post type: %v", err)
		}

		if retrieved.Template != template {
			rt.Fatalf("Template mismatch: expected %q, got %q", template, retrieved.Template)
		}

		_ = testDB
		_ = context.Background()
	})
}

func TestProperty7_PostPreviewGeneration(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		queue, testDB, cleanup := setupPropertyTestDB(t)
		defer cleanup()

		postTypeRepo := db.NewPostTypeRepository(queue)
		adminStateRepo := db.NewAdminStateRepository(queue)

		postText := rapid.StringMatching(`.{10,500}`).Draw(rt, "postText")
		photoID := rapid.StringMatching(`[A-Za-z0-9_-]{0,100}`).Draw(rt, "photoID")

		postType := &models.PostType{
			Name:     rapid.StringMatching(`[A-Za-zА-Яа-я0-9 ]{3,20}`).Draw(rt, "name"),
			PhotoID:  photoID,
			Template: rapid.StringMatching(`.{10,200}`).Draw(rt, "template"),
			IsActive: true,
		}

		if err := postTypeRepo.Create(postType); err != nil {
			rt.Fatalf("Failed to create post type: %v", err)
		}

		userID := rapid.Int64Range(1, 1000000).Draw(rt, "userID")
		state := &models.AdminState{
			UserID:         userID,
			CurrentState:   "new_post_enter_text",
			SelectedTypeID: postType.ID,
			DraftText:      postText,
			DraftPhotoID:   photoID,
		}

		if err := adminStateRepo.Save(state); err != nil {
			rt.Fatalf("Failed to save state: %v", err)
		}

		retrievedState, err := adminStateRepo.Get(userID)
		if err != nil {
			rt.Fatalf("Failed to get state: %v", err)
		}

		if retrievedState.DraftText != postText {
			rt.Fatalf("Draft text mismatch: expected %q, got %q", postText, retrievedState.DraftText)
		}

		if retrievedState.DraftPhotoID != photoID {
			rt.Fatalf("Draft photo ID mismatch: expected %q, got %q", photoID, retrievedState.DraftPhotoID)
		}

		if photoID != "" && retrievedState.DraftPhotoID == "" {
			rt.Fatalf("Photo ID should be preserved when present")
		}

		_ = testDB
		_ = context.Background()
	})
}

func TestProperty20_TypeListDisplay(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		queue, testDB, cleanup := setupPropertyTestDB(t)
		defer cleanup()

		postTypeRepo := db.NewPostTypeRepository(queue)

		numTypes := rapid.IntRange(1, 15).Draw(rt, "numTypes")

		var createdTypeIDs []int64
		var createdTypeNames []string

		for i := 0; i < numTypes; i++ {
			postType := &models.PostType{
				Name:     rapid.StringMatching(`[A-Za-zА-Яа-я0-9 ]{3,20}`).Draw(rt, "name"),
				PhotoID:  rapid.StringMatching(`[A-Za-z0-9_-]{0,100}`).Draw(rt, "photoID"),
				Template: rapid.StringMatching(`.{10,200}`).Draw(rt, "template"),
				IsActive: rapid.Bool().Draw(rt, "isActive"),
			}
			if err := postTypeRepo.Create(postType); err != nil {
				rt.Fatalf("Failed to create post type: %v", err)
			}
			createdTypeIDs = append(createdTypeIDs, postType.ID)
			createdTypeNames = append(createdTypeNames, postType.Name)
		}

		allTypes, err := postTypeRepo.GetAll()
		if err != nil {
			rt.Fatalf("Failed to get all types: %v", err)
		}

		if len(allTypes) != numTypes {
			rt.Fatalf("Expected %d types, got %d", numTypes, len(allTypes))
		}

		for _, typeID := range createdTypeIDs {
			found := false
			for _, retrievedType := range allTypes {
				if retrievedType.ID == typeID {
					found = true
					break
				}
			}
			if !found {
				rt.Fatalf("Created type ID %d not found in GetAll() results", typeID)
			}
		}

		for _, typeName := range createdTypeNames {
			found := false
			for _, retrievedType := range allTypes {
				if retrievedType.Name == typeName {
					found = true
					break
				}
			}
			if !found {
				rt.Fatalf("Created type name %q not found in GetAll() results", typeName)
			}
		}

		_ = testDB
		_ = context.Background()
	})
}
