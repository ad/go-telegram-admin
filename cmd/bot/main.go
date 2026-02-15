package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ad/go-telegram-admin/internal/db"
	"github.com/ad/go-telegram-admin/internal/handlers"
	"github.com/ad/go-telegram-admin/internal/models"
	"github.com/ad/go-telegram-admin/internal/services"
	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	_ "github.com/joho/godotenv/autoload"
	_ "modernc.org/sqlite"
)

func main() {
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN environment variable is required")
	}

	adminIDsStr := os.Getenv("ADMIN_IDS")
	forumChatIDStr := os.Getenv("FORUM_CHAT_ID")
	topicIDStr := os.Getenv("TOPIC_ID")

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "admin.db"
	}

	sqlDB, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer sqlDB.Close()

	if err := db.InitSchema(sqlDB); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}

	dbQueue := db.NewDBQueue(sqlDB)
	defer dbQueue.Close()

	postTypeRepo := db.NewPostTypeRepository(dbQueue)
	publishedPostRepo := db.NewPublishedPostRepository(dbQueue)
	adminConfigRepo := db.NewAdminConfigRepository(dbQueue)
	adminStateRepo := db.NewAdminStateRepository(dbQueue)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if adminIDsStr != "" || forumChatIDStr != "" || topicIDStr != "" {
		config, err := adminConfigRepo.Get()
		if err != nil {
			config = &models.AdminConfig{AdminIDs: []int64{}}
		}

		if adminIDsStr != "" {
			adminIDs := []int64{}
			parts := strings.Split(adminIDsStr, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					if id, err := strconv.ParseInt(part, 10, 64); err == nil {
						adminIDs = append(adminIDs, id)
					}
				}
			}
			if len(adminIDs) > 0 {
				config.AdminIDs = adminIDs
			}
		}

		if forumChatIDStr != "" {
			if forumChatID, err := strconv.ParseInt(forumChatIDStr, 10, 64); err == nil {
				config.ForumChatID = forumChatID
			}
		}

		if topicIDStr != "" {
			if topicID, err := strconv.ParseInt(topicIDStr, 10, 64); err == nil {
				config.TopicID = topicID
			}
		}

		if err := adminConfigRepo.Save(config); err != nil {
			log.Printf("Warning: Failed to save admin config from environment: %v", err)
		}
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	b, err := bot.New(botToken, bot.WithHTTPClient(15*time.Second, httpClient))
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	for i := 0; i < 3; i++ {
		log.Printf("Attempting to connect to Telegram API (attempt %d/3)...", i+1)
		getMeCtx, getMeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err = b.GetMe(getMeCtx)
		getMeCancel()
		if err == nil {
			log.Printf("Successfully connected to Telegram API")
			break
		}
		log.Printf("Failed to get bot info (attempt %d/3): %v", i+1, err)
		if i < 2 {
			log.Printf("Retrying in 2 seconds...")
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		log.Fatalf("Failed to get bot info after 3 attempts: %v", err)
	}

	postManager := services.NewPostManager(publishedPostRepo, postTypeRepo, adminConfigRepo)
	postTypeManager := services.NewPostTypeManager(postTypeRepo)
	settingsManager := services.NewSettingsManager(adminConfigRepo)
	backupManager := services.NewBackupManager(b, dbPath, dbQueue)
	adminAuthMiddleware := services.NewAdminAuthMiddleware(adminConfigRepo)

	forumAdminHandler := handlers.NewForumAdminHandler(
		b,
		adminAuthMiddleware,
		adminConfigRepo,
		postTypeRepo,
		publishedPostRepo,
		adminStateRepo,
		postManager,
		postTypeManager,
		settingsManager,
		backupManager,
	)

	b.RegisterHandlerMatchFunc(func(update *tgmodels.Update) bool {
		return true
	}, func(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
		if update.Message != nil {
			if forumAdminHandler.HandleCommand(ctx, update.Message) {
				return
			}
			forumAdminHandler.HandleMessage(ctx, update.Message)
		}
		if update.CallbackQuery != nil {
			forumAdminHandler.HandleCallback(ctx, update.CallbackQuery)
		}
	}, logMiddleware)

	log.Printf("Bot started. DB: %s", dbPath)

	b.Start(ctx)
}

func logMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
		if update.Message != nil {
			log.Printf("[MSG] from=%d text=%q", update.Message.From.ID, update.Message.Text)
		}
		if update.CallbackQuery != nil {
			log.Printf("[CALLBACK] from=%d data=%q", update.CallbackQuery.From.ID, update.CallbackQuery.Data)
		}
		next(ctx, b, update)
	}
}
