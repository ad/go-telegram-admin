package db

import (
	"database/sql"
	"testing"

	"github.com/ad/go-telegram-admin/internal/models"
	_ "modernc.org/sqlite"
	"pgregory.net/rapid"
)

func TestProperty4_ActiveTypeFiltering(t *testing.T) {
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
		repo := NewPostTypeRepository(queue)

		numTypes := rapid.IntRange(1, 20).Draw(rt, "numTypes")
		var activeCount int

		for i := 0; i < numTypes; i++ {
			isActive := rapid.Bool().Draw(rt, "isActive")
			if isActive {
				activeCount++
			}

			postType := &models.PostType{
				Name:     rapid.StringMatching(`[a-zA-Zа-яА-Я ]{3,30}`).Draw(rt, "name"),
				PhotoID:  rapid.StringMatching(`[a-zA-Z0-9_-]{0,50}`).Draw(rt, "photoID"),
				Template: rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s\.,!?]{10,200}`).Draw(rt, "template"),
				IsActive: isActive,
			}

			if err := repo.Create(postType); err != nil {
				rt.Fatal(err)
			}
		}

		activeTypes, err := repo.GetActive()
		if err != nil {
			rt.Fatal(err)
		}

		if len(activeTypes) != activeCount {
			rt.Fatalf("Expected %d active types, got %d", activeCount, len(activeTypes))
		}

		for _, pt := range activeTypes {
			if !pt.IsActive {
				rt.Fatalf("GetActive returned inactive type: %v", pt)
			}
		}
	})
}

func TestProperty13_PostTypeUpdate(t *testing.T) {
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
		repo := NewPostTypeRepository(queue)

		postType := &models.PostType{
			Name:     rapid.StringMatching(`[a-zA-Zа-яА-Я ]{3,30}`).Draw(rt, "name"),
			PhotoID:  rapid.StringMatching(`[a-zA-Z0-9_-]{0,50}`).Draw(rt, "photoID"),
			Template: rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s\.,!?]{10,200}`).Draw(rt, "template"),
			IsActive: rapid.Bool().Draw(rt, "isActive"),
		}

		if err := repo.Create(postType); err != nil {
			rt.Fatal(err)
		}

		newName := rapid.StringMatching(`[a-zA-Zа-яА-Я ]{3,30}`).Draw(rt, "newName")
		newPhotoID := rapid.StringMatching(`[a-zA-Z0-9_-]{0,50}`).Draw(rt, "newPhotoID")
		newTemplate := rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s\.,!?]{10,200}`).Draw(rt, "newTemplate")

		postType.Name = newName
		postType.PhotoID = newPhotoID
		postType.Template = newTemplate

		if err := repo.Update(postType); err != nil {
			rt.Fatal(err)
		}

		retrieved, err := repo.GetByID(postType.ID)
		if err != nil {
			rt.Fatal(err)
		}

		if retrieved.Name != newName {
			rt.Fatalf("Expected name %s, got %s", newName, retrieved.Name)
		}
		if retrieved.PhotoID != newPhotoID {
			rt.Fatalf("Expected photoID %s, got %s", newPhotoID, retrieved.PhotoID)
		}
		if retrieved.Template != newTemplate {
			rt.Fatalf("Expected template %s, got %s", newTemplate, retrieved.Template)
		}
	})
}

func TestProperty14_PostTypeDeactivation(t *testing.T) {
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
		repo := NewPostTypeRepository(queue)

		postType := &models.PostType{
			Name:     rapid.StringMatching(`[a-zA-Zа-яА-Я ]{3,30}`).Draw(rt, "name"),
			PhotoID:  rapid.StringMatching(`[a-zA-Z0-9_-]{0,50}`).Draw(rt, "photoID"),
			Template: rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s\.,!?]{10,200}`).Draw(rt, "template"),
			IsActive: true,
		}

		if err := repo.Create(postType); err != nil {
			rt.Fatal(err)
		}

		if err := repo.SetActive(postType.ID, false); err != nil {
			rt.Fatal(err)
		}

		activeTypes, err := repo.GetActive()
		if err != nil {
			rt.Fatal(err)
		}

		for _, pt := range activeTypes {
			if pt.ID == postType.ID {
				rt.Fatalf("Deactivated type %d should not appear in active types list", postType.ID)
			}
		}

		retrieved, err := repo.GetByID(postType.ID)
		if err != nil {
			rt.Fatal(err)
		}

		if retrieved.IsActive {
			rt.Fatalf("Expected IsActive to be false, got true")
		}
	})
}
