package services

import (
	"database/sql"
	"testing"

	"github.com/ad/go-telegram-admin/internal/db"
	"github.com/ad/go-telegram-admin/internal/models"
	_ "modernc.org/sqlite"
	"pgregory.net/rapid"
)

func setupAdminAuthTestDB(t *testing.T) (*sql.DB, *AdminAuthMiddleware) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	if err := db.InitSchema(testDB); err != nil {
		t.Fatal(err)
	}

	queue := db.NewDBQueueForTest(testDB)
	repo := db.NewAdminConfigRepository(queue)
	middleware := NewAdminAuthMiddleware(repo)

	return testDB, middleware
}

func TestProperty1_AdminAuthorizationCheck(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		testDB, middleware := setupAdminAuthTestDB(t)
		defer testDB.Close()

		queue := db.NewDBQueueForTest(testDB)
		repo := db.NewAdminConfigRepository(queue)

		numAdmins := rapid.IntRange(1, 10).Draw(rt, "numAdmins")
		adminIDs := make([]int64, numAdmins)
		for i := 0; i < numAdmins; i++ {
			adminIDs[i] = rapid.Int64Range(1, 1000000).Draw(rt, "adminID")
		}

		config := &models.AdminConfig{
			AdminIDs:    adminIDs,
			ForumChatID: rapid.Int64Range(-1000000000000, -1).Draw(rt, "forumChatID"),
			TopicID:     rapid.Int64Range(1, 1000000).Draw(rt, "topicID"),
		}

		if err := repo.Save(config); err != nil {
			rt.Fatal(err)
		}

		for _, adminID := range adminIDs {
			isAuthorized := middleware.IsAuthorized(adminID)
			if !isAuthorized {
				rt.Fatalf("Expected admin ID %d to be authorized", adminID)
			}

			shouldIgnore := middleware.ShouldIgnore(adminID)
			if shouldIgnore {
				rt.Fatalf("Expected admin ID %d to not be ignored", adminID)
			}
		}
	})
}

func TestProperty2_NonAdminIgnore(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		testDB, middleware := setupAdminAuthTestDB(t)
		defer testDB.Close()

		queue := db.NewDBQueueForTest(testDB)
		repo := db.NewAdminConfigRepository(queue)

		numAdmins := rapid.IntRange(1, 10).Draw(rt, "numAdmins")
		adminIDs := make([]int64, numAdmins)
		for i := 0; i < numAdmins; i++ {
			adminIDs[i] = rapid.Int64Range(1, 1000000).Draw(rt, "adminID")
		}

		config := &models.AdminConfig{
			AdminIDs:    adminIDs,
			ForumChatID: rapid.Int64Range(-1000000000000, -1).Draw(rt, "forumChatID"),
			TopicID:     rapid.Int64Range(1, 1000000).Draw(rt, "topicID"),
		}

		if err := repo.Save(config); err != nil {
			rt.Fatal(err)
		}

		nonAdminID := rapid.Int64Range(1000001, 2000000).Draw(rt, "nonAdminID")

		isInAdminList := false
		for _, adminID := range adminIDs {
			if adminID == nonAdminID {
				isInAdminList = true
				break
			}
		}

		if isInAdminList {
			rt.Skip("Generated non-admin ID is in admin list")
		}

		isAuthorized := middleware.IsAuthorized(nonAdminID)
		if isAuthorized {
			rt.Fatalf("Expected non-admin ID %d to not be authorized", nonAdminID)
		}

		shouldIgnore := middleware.ShouldIgnore(nonAdminID)
		if !shouldIgnore {
			rt.Fatalf("Expected non-admin ID %d to be ignored", nonAdminID)
		}
	})
}

func TestAdminAuthWithEmptyList(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatal(err)
	}

	queue := db.NewDBQueueForTest(testDB)
	repo := db.NewAdminConfigRepository(queue)
	middleware := NewAdminAuthMiddleware(repo)

	config := &models.AdminConfig{
		AdminIDs:    []int64{},
		ForumChatID: -1001234567890,
		TopicID:     42,
	}

	if err := repo.Save(config); err != nil {
		t.Fatal(err)
	}

	userID := int64(123456)
	isAuthorized := middleware.IsAuthorized(userID)
	if isAuthorized {
		t.Errorf("Expected user %d to not be authorized with empty admin list", userID)
	}

	shouldIgnore := middleware.ShouldIgnore(userID)
	if !shouldIgnore {
		t.Errorf("Expected user %d to be ignored with empty admin list", userID)
	}
}
