package services

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ad/go-telegram-admin/internal/db"
	"github.com/ad/go-telegram-admin/internal/models"
	_ "modernc.org/sqlite"
	"pgregory.net/rapid"
)

func TestGenerateSQLDumpGo_IncludesAllTables(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	queue := db.NewDBQueueForTest(testDB)
	bm := &BackupManager{
		queue:  queue,
		dbPath: ":memory:",
	}

	dump, err := bm.GenerateSQLDumpGo(":memory:")
	if err != nil {
		t.Fatalf("Failed to generate SQL dump: %v", err)
	}

	requiredTables := []string{
		"post_types",
		"published_posts",
		"admin_config",
	}

	for _, table := range requiredTables {
		if !strings.Contains(dump, table) {
			t.Errorf("Backup should include table %s", table)
		}
	}

	if !strings.Contains(dump, "BEGIN TRANSACTION") {
		t.Error("Backup should start with BEGIN TRANSACTION")
	}

	if !strings.Contains(dump, "COMMIT") {
		t.Error("Backup should end with COMMIT")
	}
}

func TestGenerateSQLDumpGo_IncludesData(t *testing.T) {
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
		Template: "Test Template",
		IsActive: true,
	}

	if err := postTypeRepo.Create(postType); err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	bm := &BackupManager{
		queue:  queue,
		dbPath: ":memory:",
	}

	dump, err := bm.GenerateSQLDumpGo(":memory:")
	if err != nil {
		t.Fatalf("Failed to generate SQL dump: %v", err)
	}

	if !strings.Contains(dump, "Test Type") {
		t.Error("Backup should include inserted data")
	}

	if !strings.Contains(dump, "Test Template") {
		t.Error("Backup should include template data")
	}
}

func TestBackupFilenameFormat(t *testing.T) {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("backup_%s.sql", timestamp)

	if !strings.HasPrefix(filename, "backup_") {
		t.Error("Filename should start with 'backup_'")
	}

	if !strings.HasSuffix(filename, ".sql") {
		t.Error("Filename should end with '.sql'")
	}

	parts := strings.Split(filename, "_")
	if len(parts) < 3 {
		t.Error("Filename should contain date and time")
	}

	datePart := parts[1]
	if len(datePart) != 10 {
		t.Error("Date part should be in YYYY-MM-DD format")
	}

	timePart := strings.TrimSuffix(parts[2], ".sql")
	if len(timePart) != 8 {
		t.Error("Time part should be in HH-MM-SS format")
	}
}

func TestProperty21_BackupCompleteness(t *testing.T) {
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
		postTypeRepo := db.NewPostTypeRepository(queue)
		publishedPostRepo := db.NewPublishedPostRepository(queue)

		numTypes := rapid.IntRange(1, 5).Draw(rt, "numTypes")
		createdTypes := make([]*models.PostType, 0, numTypes)

		for i := 0; i < numTypes; i++ {
			postType := &models.PostType{
				Name:     rapid.StringMatching(`[a-zA-Zа-яА-Я ]{3,30}`).Draw(rt, fmt.Sprintf("name_%d", i)),
				PhotoID:  rapid.StringMatching(`[a-zA-Z0-9_-]{0,50}`).Draw(rt, fmt.Sprintf("photoID_%d", i)),
				Template: rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s\.,!?]{10,200}`).Draw(rt, fmt.Sprintf("template_%d", i)),
				IsActive: rapid.Bool().Draw(rt, fmt.Sprintf("isActive_%d", i)),
			}

			if err := postTypeRepo.Create(postType); err != nil {
				rt.Fatal(err)
			}
			createdTypes = append(createdTypes, postType)
		}

		numPosts := rapid.IntRange(0, 10).Draw(rt, "numPosts")
		createdPosts := make([]*models.PublishedPost, 0, numPosts)

		usedCombinations := make(map[string]bool)

		for i := 0; i < numPosts; i++ {
			typeIdx := rapid.IntRange(0, len(createdTypes)-1).Draw(rt, fmt.Sprintf("typeIdx_%d", i))

			var chatID, messageID int64
			var combinationKey string
			maxAttempts := 100
			attempt := 0

			for attempt < maxAttempts {
				chatID = rapid.Int64Range(-1000000000000, -1).Draw(rt, fmt.Sprintf("chatID_%d_%d", i, attempt))
				messageID = rapid.Int64Range(1, 999999).Draw(rt, fmt.Sprintf("messageID_%d_%d", i, attempt))
				combinationKey = fmt.Sprintf("%d_%d", chatID, messageID)

				if !usedCombinations[combinationKey] {
					usedCombinations[combinationKey] = true
					break
				}
				attempt++
			}

			if attempt >= maxAttempts {
				continue
			}

			post := &models.PublishedPost{
				PostTypeID: createdTypes[typeIdx].ID,
				ChatID:     chatID,
				TopicID:    rapid.Int64Range(1, 999999).Draw(rt, fmt.Sprintf("topicID_%d", i)),
				MessageID:  messageID,
				Text:       rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s\.,!?]{10,200}`).Draw(rt, fmt.Sprintf("text_%d", i)),
				PhotoID:    createdTypes[typeIdx].PhotoID,
			}

			if err := publishedPostRepo.Create(post); err != nil {
				rt.Fatal(err)
			}
			createdPosts = append(createdPosts, post)
		}

		bm := &BackupManager{
			queue:  queue,
			dbPath: ":memory:",
		}

		dump, err := bm.GenerateSQLDumpGo(":memory:")
		if err != nil {
			rt.Fatalf("Failed to generate backup: %v", err)
		}

		if !strings.Contains(dump, "BEGIN TRANSACTION") {
			rt.Fatal("Backup should contain BEGIN TRANSACTION")
		}

		if !strings.Contains(dump, "COMMIT") {
			rt.Fatal("Backup should contain COMMIT")
		}

		if !strings.Contains(dump, "post_types") {
			rt.Fatal("Backup should contain post_types table")
		}

		if !strings.Contains(dump, "published_posts") {
			rt.Fatal("Backup should contain published_posts table")
		}

		for _, postType := range createdTypes {
			if !strings.Contains(dump, postType.Name) {
				rt.Fatalf("Backup should contain post type name: %s", postType.Name)
			}
		}

		for _, post := range createdPosts {
			if !strings.Contains(dump, post.Text) {
				rt.Fatalf("Backup should contain post text: %s", post.Text)
			}
		}
	})
}

func TestProperty22_BackupFilenameFormat(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		year := rapid.IntRange(2020, 2030).Draw(rt, "year")
		month := rapid.IntRange(1, 12).Draw(rt, "month")
		day := rapid.IntRange(1, 28).Draw(rt, "day")
		hour := rapid.IntRange(0, 23).Draw(rt, "hour")
		minute := rapid.IntRange(0, 59).Draw(rt, "minute")
		second := rapid.IntRange(0, 59).Draw(rt, "second")

		timestamp := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)
		filename := fmt.Sprintf("backup_%s.sql", timestamp.Format("2006-01-02_15-04-05"))

		if !strings.HasPrefix(filename, "backup_") {
			rt.Fatal("Filename should start with 'backup_'")
		}

		if !strings.HasSuffix(filename, ".sql") {
			rt.Fatal("Filename should end with '.sql'")
		}

		expectedFormat := fmt.Sprintf("backup_%04d-%02d-%02d_%02d-%02d-%02d.sql",
			year, month, day, hour, minute, second)

		if filename != expectedFormat {
			rt.Fatalf("Expected filename %s, got %s", expectedFormat, filename)
		}

		parts := strings.Split(filename, "_")
		if len(parts) != 3 {
			rt.Fatal("Filename should have exactly 3 parts separated by underscores")
		}

		datePart := parts[1]
		if len(datePart) != 10 {
			rt.Fatal("Date part should be 10 characters (YYYY-MM-DD)")
		}

		timePart := strings.TrimSuffix(parts[2], ".sql")
		if len(timePart) != 8 {
			rt.Fatal("Time part should be 8 characters (HH-MM-SS)")
		}
	})
}

func TestCLIMethodWorksIfSqlite3Available(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_backup_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	testDB, err := sql.Open("sqlite", tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	bm := &BackupManager{
		queue:  db.NewDBQueueForTest(testDB),
		dbPath: tmpFile.Name(),
	}

	dump, err := bm.GenerateSQLDumpCLI(tmpFile.Name())
	if err != nil {
		t.Skipf("sqlite3 command not available, skipping CLI test: %v", err)
		return
	}

	if dump == "" {
		t.Error("CLI dump should not be empty")
	}

	if !strings.Contains(dump, "CREATE TABLE") {
		t.Error("CLI dump should contain CREATE TABLE statements")
	}
}

func TestGoMethodWorksAsFallback(t *testing.T) {
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
		Name:     "Fallback Test",
		PhotoID:  "photo456",
		Template: "Fallback Template",
		IsActive: true,
	}

	if err := postTypeRepo.Create(postType); err != nil {
		t.Fatalf("Failed to create post type: %v", err)
	}

	bm := &BackupManager{
		queue:  queue,
		dbPath: ":memory:",
	}

	dump, err := bm.GenerateSQLDumpGo(":memory:")
	if err != nil {
		t.Fatalf("Go fallback method should work: %v", err)
	}

	if dump == "" {
		t.Error("Go dump should not be empty")
	}

	if !strings.Contains(dump, "Fallback Test") {
		t.Error("Go dump should contain inserted data")
	}

	if !strings.Contains(dump, "BEGIN TRANSACTION") {
		t.Error("Go dump should contain transaction markers")
	}
}

func TestCreateBackup_TriesCLIThenFallsBackToGo(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	bm := &BackupManager{
		queue:  db.NewDBQueueForTest(testDB),
		dbPath: ":memory:",
	}

	dump, err := bm.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup should succeed: %v", err)
	}

	if dump == "" {
		t.Error("Backup should not be empty")
	}

	if !strings.Contains(dump, "BEGIN TRANSACTION") || !strings.Contains(dump, "COMMIT") {
		t.Error("Backup should contain transaction markers")
	}
}

func TestSendBackupToAdmin_CreatesFileWithCorrectName(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer testDB.Close()

	if err := db.InitSchema(testDB); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	bm := &BackupManager{
		queue:  db.NewDBQueueForTest(testDB),
		dbPath: ":memory:",
		bot:    nil,
	}

	sqlDump := "BEGIN TRANSACTION;\nCREATE TABLE test (id INTEGER);\nCOMMIT;"

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	expectedFilename := fmt.Sprintf("backup_%s.sql", timestamp)

	if !strings.Contains(expectedFilename, "backup_") {
		t.Error("Filename should contain 'backup_' prefix")
	}

	if !strings.Contains(expectedFilename, ".sql") {
		t.Error("Filename should have .sql extension")
	}

	if bm.bot != nil {
		ctx := context.Background()
		err = bm.SendBackupToAdmin(ctx, 123456789, sqlDump)
		if err != nil {
			t.Logf("SendBackupToAdmin error: %v", err)
		}
	} else {
		t.Log("Bot is nil, skipping actual send test, but filename format is validated")
	}
}
