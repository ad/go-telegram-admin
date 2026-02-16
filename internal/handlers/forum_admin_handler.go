package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/ad/go-telegram-admin/internal/db"
	"github.com/ad/go-telegram-admin/internal/fsm"
	"github.com/ad/go-telegram-admin/internal/models"
	"github.com/ad/go-telegram-admin/internal/services"
	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

func utf16Length(s string) int {
	length := 0
	for _, r := range s {
		if r <= 0xFFFF {
			length++
		} else {
			length += 2
		}
	}
	return length
}

type ForumAdminHandler struct {
	bot               *bot.Bot
	authMiddleware    *services.AdminAuthMiddleware
	adminConfigRepo   *db.AdminConfigRepository
	postTypeRepo      *db.PostTypeRepository
	publishedPostRepo *db.PublishedPostRepository
	replyRepo         *db.ReplyRepository
	adminStateRepo    *db.AdminStateRepository
	postManager       *services.PostManager
	postTypeManager   *services.PostTypeManager
	settingsManager   *services.SettingsManager
	backupManager     *services.BackupManager
}

func NewForumAdminHandler(
	b *bot.Bot,
	authMiddleware *services.AdminAuthMiddleware,
	adminConfigRepo *db.AdminConfigRepository,
	postTypeRepo *db.PostTypeRepository,
	publishedPostRepo *db.PublishedPostRepository,
	replyRepo *db.ReplyRepository,
	adminStateRepo *db.AdminStateRepository,
	postManager *services.PostManager,
	postTypeManager *services.PostTypeManager,
	settingsManager *services.SettingsManager,
	backupManager *services.BackupManager,
) *ForumAdminHandler {
	return &ForumAdminHandler{
		bot:               b,
		authMiddleware:    authMiddleware,
		adminConfigRepo:   adminConfigRepo,
		postTypeRepo:      postTypeRepo,
		publishedPostRepo: publishedPostRepo,
		replyRepo:         replyRepo,
		adminStateRepo:    adminStateRepo,
		postManager:       postManager,
		postTypeManager:   postTypeManager,
		settingsManager:   settingsManager,
		backupManager:     backupManager,
	}
}

func (h *ForumAdminHandler) HandleCommand(ctx context.Context, msg *tgmodels.Message) bool {
	if h.authMiddleware.ShouldIgnore(msg.From.ID) {
		return false
	}

	switch msg.Text {
	case "/start", "/admin":
		h.showAdminMenu(ctx, msg.Chat.ID, 0)
		return true
	case "/new":
		h.handleNewCommand(ctx, msg.Chat.ID, 0)
		return true
	case "/edit":
		h.handleEditCommand(ctx, msg.From.ID, msg.Chat.ID, 0)
		return true
	case "/delete":
		h.handleDeleteCommand(ctx, msg.From.ID, msg.Chat.ID, 0)
		return true
	case "/cancel":
		h.handleCancelCommand(ctx, msg.From.ID, msg.Chat.ID)
		return true
	default:
		return false
	}
}

func (h *ForumAdminHandler) HandleMessage(ctx context.Context, msg *tgmodels.Message) bool {
	if h.authMiddleware.ShouldIgnore(msg.From.ID) {
		return false
	}

	state, err := h.adminStateRepo.Get(msg.From.ID)
	if err != nil || state == nil {
		return false
	}

	switch state.CurrentState {
	case fsm.StateNewPostEnterText:
		h.handlePostTextInput(ctx, msg, state)
		return true
	case fsm.StateEditPostEnterLink:
		h.handleEditPostLinkInput(ctx, msg, state)
		return true
	case fsm.StateEditPostEnterText:
		h.handleEditPostTextInput(ctx, msg, state)
		return true
	case fsm.StateDeletePostEnterLink:
		h.handleDeletePostLinkInput(ctx, msg, state)
		return true
	case fsm.StateNewTypeEnterName:
		h.handleNewTypeNameInput(ctx, msg, state)
		return true
	case fsm.StateNewTypeEnterEmoji:
		h.handleNewTypeEmojiInput(ctx, msg, state)
		return true
	case fsm.StateNewTypeEnterImage:
		h.handleNewTypeImageInput(ctx, msg, state)
		return true
	case fsm.StateNewTypeEnterTemplate:
		h.handleNewTypeTemplateInput(ctx, msg, state)
		return true
	case fsm.StateEditTypeName:
		h.handleEditTypeNameInput(ctx, msg, state)
		return true
	case fsm.StateEditTypeEmoji:
		h.handleEditTypeEmojiInput(ctx, msg, state)
		return true
	case fsm.StateEditTypeImage:
		h.handleEditTypeImageInput(ctx, msg, state)
		return true
	case fsm.StateEditTypeTemplate:
		h.handleEditTypeTemplateInput(ctx, msg, state)
		return true
	case fsm.StateEditAdminIDs:
		h.handleEditAdminIDsInput(ctx, msg, state)
		return true
	case fsm.StateEditForumID:
		h.handleEditForumIDInput(ctx, msg, state)
		return true
	case fsm.StateEditTopicID:
		h.handleEditTopicIDInput(ctx, msg, state)
		return true
	case fsm.StateReplyEnterLink:
		h.handleReplyLinkInput(ctx, msg, state)
		return true
	case fsm.StateReplyEnterText:
		h.handleReplyTextInput(ctx, msg, state)
		return true
	case fsm.StateEditReplyEnterText:
		h.handleEditReplyTextInput(ctx, msg, state)
		return true
	default:
		return false
	}
}

func (h *ForumAdminHandler) HandleCallback(ctx context.Context, callback *tgmodels.CallbackQuery) bool {
	if h.authMiddleware.ShouldIgnore(callback.From.ID) {
		return false
	}

	msg := callback.Message.Message
	if msg == nil {
		return false
	}

	h.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	})

	chatID := msg.Chat.ID
	messageID := msg.ID
	data := callback.Data

	log.Printf("[FORUM_ADMIN] Callback received: %s", data)

	if data == "cancel" {
		h.handleCancelCallback(ctx, callback.From.ID, chatID, messageID)
		return true
	}

	if data == "skip_emoji" {
		h.handleSkipEmojiCallback(ctx, callback.From.ID, chatID, messageID)
		return true
	}

	if data == "skip_image" {
		h.handleSkipImageCallback(ctx, callback.From.ID, chatID, messageID)
		return true
	}

	if data == "admin_settings" {
		h.showSettingsMenu(ctx, chatID, messageID)
		return true
	}

	if data == "admin_new_post" {
		h.handleNewCommand(ctx, chatID, messageID)
		return true
	}

	if data == "admin_edit_post" {
		h.handleEditCommand(ctx, callback.From.ID, chatID, messageID)
		return true
	}

	if data == "admin_delete_post" {
		h.handleDeleteCommand(ctx, callback.From.ID, chatID, messageID)
		return true
	}

	if data == "settings_new_type" {
		h.handleNewTypeStart(ctx, callback.From.ID, chatID, messageID)
		return true
	}

	if data == "settings_manage_types" {
		h.handleManageTypesMenu(ctx, chatID, messageID)
		return true
	}

	if data == "settings_access" {
		h.showAccessSettingsMenu(ctx, chatID, messageID)
		return true
	}

	if data == "settings_backup" {
		h.handleBackupCommand(ctx, callback.From.ID, chatID, messageID)
		return true
	}

	if data == "access_edit_admins" {
		h.handleEditAdminIDsStart(ctx, callback.From.ID, chatID, messageID)
		return true
	}

	if data == "access_edit_forum" {
		h.handleEditForumIDStart(ctx, callback.From.ID, chatID, messageID)
		return true
	}

	if data == "access_edit_topic" {
		h.handleEditTopicIDStart(ctx, callback.From.ID, chatID, messageID)
		return true
	}

	if data == "confirm_post" {
		h.handlePostConfirmation(ctx, callback.From.ID, chatID, messageID)
		return true
	}

	if strings.HasPrefix(data, "select_type:") {
		typeIDStr := strings.TrimPrefix(data, "select_type:")
		typeID, err := strconv.ParseInt(typeIDStr, 10, 64)
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to parse type ID: %v", err)
			return false
		}
		h.handleTypeSelection(ctx, callback.From.ID, chatID, messageID, typeID)
		return true
	}

	if strings.HasPrefix(data, "manage_type:") {
		typeIDStr := strings.TrimPrefix(data, "manage_type:")
		typeID, err := strconv.ParseInt(typeIDStr, 10, 64)
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to parse type ID: %v", err)
			return false
		}
		h.handleTypeManagementOptions(ctx, callback.From.ID, chatID, messageID, typeID)
		return true
	}

	if strings.HasPrefix(data, "edit_type_name:") {
		typeIDStr := strings.TrimPrefix(data, "edit_type_name:")
		typeID, err := strconv.ParseInt(typeIDStr, 10, 64)
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to parse type ID: %v", err)
			return false
		}
		h.handleEditTypeNameStart(ctx, callback.From.ID, chatID, messageID, typeID)
		return true
	}

	if strings.HasPrefix(data, "edit_type_emoji:") {
		typeIDStr := strings.TrimPrefix(data, "edit_type_emoji:")
		typeID, err := strconv.ParseInt(typeIDStr, 10, 64)
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to parse type ID: %v", err)
			return false
		}
		h.handleEditTypeEmojiStart(ctx, callback.From.ID, chatID, messageID, typeID)
		return true
	}

	if strings.HasPrefix(data, "edit_type_image:") {
		typeIDStr := strings.TrimPrefix(data, "edit_type_image:")
		typeID, err := strconv.ParseInt(typeIDStr, 10, 64)
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to parse type ID: %v", err)
			return false
		}
		h.handleEditTypeImageStart(ctx, callback.From.ID, chatID, messageID, typeID)
		return true
	}

	if strings.HasPrefix(data, "edit_type_template:") {
		typeIDStr := strings.TrimPrefix(data, "edit_type_template:")
		typeID, err := strconv.ParseInt(typeIDStr, 10, 64)
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to parse type ID: %v", err)
			return false
		}
		h.handleEditTypeTemplateStart(ctx, callback.From.ID, chatID, messageID, typeID)
		return true
	}

	if strings.HasPrefix(data, "toggle_type_active:") {
		typeIDStr := strings.TrimPrefix(data, "toggle_type_active:")
		typeID, err := strconv.ParseInt(typeIDStr, 10, 64)
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to parse type ID: %v", err)
			return false
		}
		h.handleToggleTypeActive(ctx, callback.From.ID, chatID, messageID, typeID)
		return true
	}

	if data == "admin_reply" {
		h.handleReplyStart(ctx, callback.From.ID, chatID, messageID)
		return true
	}

	if data == "confirm_reply" {
		h.handleReplyConfirmation(ctx, callback.From.ID, chatID, messageID)
		return true
	}

	if data == "admin_reply_list" {
		h.showReplyList(ctx, chatID, messageID, 0)
		return true
	}

	if data == "reply_list_back" {
		h.showAdminMenu(ctx, chatID, messageID)
		return true
	}

	if strings.HasPrefix(data, "reply_list_page:") {
		pageStr := strings.TrimPrefix(data, "reply_list_page:")
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to parse reply list page: %v", err)
			return false
		}
		h.showReplyList(ctx, chatID, messageID, page)
		return true
	}

	if strings.HasPrefix(data, "reply_details:") {
		// format: reply_details:{replyID}:{page}
		parts := strings.SplitN(strings.TrimPrefix(data, "reply_details:"), ":", 2)
		if len(parts) != 2 {
			return false
		}
		replyID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return false
		}
		page, err := strconv.Atoi(parts[1])
		if err != nil {
			return false
		}
		h.showReplyDetails(ctx, chatID, messageID, replyID, page)
		return true
	}

	if strings.HasPrefix(data, "reply_list_delete_confirm:") {
		// format: reply_list_delete_confirm:{replyID}:{page}
		parts := strings.SplitN(strings.TrimPrefix(data, "reply_list_delete_confirm:"), ":", 2)
		if len(parts) != 2 {
			return false
		}
		replyID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return false
		}
		h.handleDeleteReplyFromList(ctx, callback.From.ID, chatID, messageID, replyID)
		return true
	}

	if strings.HasPrefix(data, "reply_list_delete:") {
		// format: reply_list_delete:{replyID}:{page}
		parts := strings.SplitN(strings.TrimPrefix(data, "reply_list_delete:"), ":", 2)
		if len(parts) != 2 {
			return false
		}
		replyID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return false
		}
		page, err := strconv.Atoi(parts[1])
		if err != nil {
			return false
		}
		h.showDeleteReplyConfirm(ctx, chatID, messageID, replyID, page)
		return true
	}

	if strings.HasPrefix(data, "reply_list_edit:") {
		// format: reply_list_edit:{replyID}:{page}
		parts := strings.SplitN(strings.TrimPrefix(data, "reply_list_edit:"), ":", 2)
		if len(parts) != 2 {
			return false
		}
		replyID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return false
		}
		h.handleEditReplyFromList(ctx, callback.From.ID, chatID, messageID, replyID)
		return true
	}

	if data == "admin_post_list" {
		h.showPostList(ctx, chatID, messageID, 0)
		return true
	}

	if data == "post_list_back" {
		h.showAdminMenu(ctx, chatID, messageID)
		return true
	}

	if strings.HasPrefix(data, "post_list_page:") {
		pageStr := strings.TrimPrefix(data, "post_list_page:")
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to parse page: %v", err)
			return false
		}
		h.showPostList(ctx, chatID, messageID, page)
		return true
	}

	if strings.HasPrefix(data, "post_details:") {
		// format: post_details:{postID}:{page}
		parts := strings.SplitN(strings.TrimPrefix(data, "post_details:"), ":", 2)
		if len(parts) != 2 {
			return false
		}
		postID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to parse post ID: %v", err)
			return false
		}
		page, err := strconv.Atoi(parts[1])
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to parse page: %v", err)
			return false
		}
		h.showPostDetails(ctx, chatID, messageID, postID, page)
		return true
	}

	if strings.HasPrefix(data, "post_list_edit:") {
		// format: post_list_edit:{postID}:{page}
		parts := strings.SplitN(strings.TrimPrefix(data, "post_list_edit:"), ":", 2)
		if len(parts) != 2 {
			return false
		}
		postID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to parse post ID: %v", err)
			return false
		}
		h.handleEditPostFromList(ctx, callback.From.ID, chatID, messageID, postID)
		return true
	}

	if strings.HasPrefix(data, "post_list_delete_confirm:") {
		// format: post_list_delete_confirm:{postID}:{page}
		parts := strings.SplitN(strings.TrimPrefix(data, "post_list_delete_confirm:"), ":", 2)
		if len(parts) != 2 {
			return false
		}
		postID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to parse post ID: %v", err)
			return false
		}
		h.handleDeletePostFromList(ctx, callback.From.ID, chatID, messageID, postID)
		return true
	}

	if strings.HasPrefix(data, "post_list_delete:") {
		// format: post_list_delete:{postID}:{page}
		parts := strings.SplitN(strings.TrimPrefix(data, "post_list_delete:"), ":", 2)
		if len(parts) != 2 {
			return false
		}
		postID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to parse post ID: %v", err)
			return false
		}
		page, err := strconv.Atoi(parts[1])
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to parse page: %v", err)
			return false
		}
		h.showDeletePostConfirm(ctx, chatID, messageID, postID, page)
		return true
	}

	return false
}

func (h *ForumAdminHandler) handleTypeSelection(ctx context.Context, userID, chatID int64, messageID int, typeID int64) {
	postType, err := h.postTypeRepo.GetByID(typeID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get post type: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка получения типа поста",
		})
		return
	}

	err = h.adminStateRepo.Save(&models.AdminState{
		UserID:         userID,
		CurrentState:   fsm.StateNewPostEnterText,
		SelectedTypeID: typeID,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка сохранения состояния",
		})
		return
	}

	if messageID > 0 {
		_, err = h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: messageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete message: %v", err)
		}
	}

	templatePrefix := fmt.Sprintf("Шаблон для типа \"%s\":\n\n", postType.Name)
	templateText := templatePrefix + postType.Template + "\n\nОтправьте текст поста."

	var templateEntities []tgmodels.MessageEntity
	if postType.TemplateEntities != "" {
		var entities []tgmodels.MessageEntity
		json.Unmarshal([]byte(postType.TemplateEntities), &entities)
		offsetAdjustment := utf16Length(templatePrefix)
		for _, entity := range entities {
			adjustedEntity := entity
			adjustedEntity.Offset += offsetAdjustment
			templateEntities = append(templateEntities, adjustedEntity)
		}
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	var sentMsg *tgmodels.Message
	if postType.PhotoID != "" {
		sentMsg, err = h.bot.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID:          chatID,
			Photo:           &tgmodels.InputFileString{Data: postType.PhotoID},
			Caption:         templateText,
			CaptionEntities: templateEntities,
			ReplyMarkup:     keyboard,
		})
	} else {
		sentMsg, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        templateText,
			Entities:    templateEntities,
			ReplyMarkup: keyboard,
		})
	}

	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send template: %v", err)
	} else if sentMsg != nil {
		state, _ := h.adminStateRepo.Get(userID)
		if state != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
	}

	log.Printf("[FORUM_ADMIN] Type %d selected by user %d, state set to StateNewPostEnterText", typeID, userID)
}

func (h *ForumAdminHandler) showAdminMenu(ctx context.Context, chatID int64, messageID int) {
	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "➕ Новый пост", CallbackData: "admin_new_post"},
			},
			{
				{Text: "✏️ Редактировать пост", CallbackData: "admin_edit_post"},
			},
			{
				{Text: "🗑 Удалить пост", CallbackData: "admin_delete_post"},
			},
			{
				{Text: "📋 Список постов", CallbackData: "admin_post_list"},
			},
			{
				{Text: "💬 Ответить на сообщение", CallbackData: "admin_reply"},
			},
			{
				{Text: "📨 Список ответов", CallbackData: "admin_reply_list"},
			},
			{
				{Text: "⚙️ Настройки", CallbackData: "admin_settings"},
			},
		},
	}

	text := "Админ-панель управления постами"

	if messageID > 0 {
		_, err := h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to edit admin menu: %v", err)
		}
	} else {
		_, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to send admin menu: %v", err)
		}
	}
}

func (h *ForumAdminHandler) showSettingsMenu(ctx context.Context, chatID int64, messageID int) {
	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "➕ Новый тип", CallbackData: "settings_new_type"},
			},
			{
				{Text: "📋 Типы постов", CallbackData: "settings_manage_types"},
			},
			{
				{Text: "🔐 Настройки доступа", CallbackData: "settings_access"},
			},
			{
				{Text: "💾 Бэкап", CallbackData: "settings_backup"},
			},
			{
				{Text: "← Назад", CallbackData: "cancel"},
			},
		},
	}

	text := "Настройки"

	if messageID > 0 {
		_, err := h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to edit settings menu: %v", err)
		}
	} else {
		_, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to send settings menu: %v", err)
		}
	}
}

func (h *ForumAdminHandler) handleNewCommand(ctx context.Context, chatID int64, messageID int) {
	log.Printf("[FORUM_ADMIN] /new command for chat %d", chatID)

	activeTypes, err := h.postTypeRepo.GetActive()
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get active types: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка получения типов постов",
		})
		return
	}

	if len(activeTypes) == 0 {
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Нет доступных типов постов. Создайте тип в настройках.",
		})
		return
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: make([][]tgmodels.InlineKeyboardButton, 0, len(activeTypes)),
	}

	for _, pt := range activeTypes {
		buttonText := pt.Name
		if pt.Emoji != "" {
			buttonText = pt.Emoji + " " + pt.Name
		}
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []tgmodels.InlineKeyboardButton{
			{
				Text:         buttonText,
				CallbackData: fmt.Sprintf("select_type:%d", pt.ID),
			},
		})
	}

	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []tgmodels.InlineKeyboardButton{
		{Text: "← Назад", CallbackData: "cancel"},
	})

	text := "Выберите тип поста:"

	if messageID > 0 {
		_, err = h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
	} else {
		_, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
	}

	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send type selection: %v", err)
	}
}

func (h *ForumAdminHandler) handleEditCommand(ctx context.Context, userID, chatID int64, messageID int) {
	log.Printf("[FORUM_ADMIN] /edit command for chat %d", chatID)

	err := h.adminStateRepo.Save(&models.AdminState{
		UserID:       userID,
		CurrentState: fsm.StateEditPostEnterLink,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка сохранения состояния",
		})
		return
	}

	text := "Отправьте ссылку на пост, который хотите отредактировать."

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	var sentMsg *tgmodels.Message
	if messageID > 0 {
		sentMsg, err = h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
	} else {
		sentMsg, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
	}

	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send edit prompt: %v", err)
	} else if sentMsg != nil {
		state, _ := h.adminStateRepo.Get(userID)
		if state != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
	}
}

func (h *ForumAdminHandler) handleDeleteCommand(ctx context.Context, userID, chatID int64, messageID int) {
	log.Printf("[FORUM_ADMIN] /delete command for chat %d", chatID)

	err := h.adminStateRepo.Save(&models.AdminState{
		UserID:       userID,
		CurrentState: fsm.StateDeletePostEnterLink,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка сохранения состояния",
		})
		return
	}

	text := "Отправьте ссылку на пост, который хотите удалить."

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	var sentMsg *tgmodels.Message
	if messageID > 0 {
		sentMsg, err = h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
	} else {
		sentMsg, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
	}

	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send delete prompt: %v", err)
	} else if sentMsg != nil {
		state, _ := h.adminStateRepo.Get(userID)
		if state != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
	}
}

func (h *ForumAdminHandler) handleCancelCommand(ctx context.Context, userID, chatID int64) {
	log.Printf("[FORUM_ADMIN] /cancel command for user %d, chat %d", userID, chatID)

	err := h.adminStateRepo.Clear(userID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to clear state: %v", err)
	}

	h.showAdminMenu(ctx, chatID, 0)
}

func (h *ForumAdminHandler) handleCancelCallback(ctx context.Context, userID, chatID int64, messageID int) {
	err := h.adminStateRepo.Clear(userID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to clear state: %v", err)
	}

	if messageID > 0 {
		_, err = h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: messageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete message: %v", err)
		}
	}

	h.showAdminMenu(ctx, chatID, 0)
	log.Printf("[FORUM_ADMIN] Cancel callback for user %d", userID)
}

func (h *ForumAdminHandler) handlePostTextInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if msg.Text == "" {
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Пожалуйста, отправьте текст поста",
		})
		return
	}

	if state.LastBotMessageID > 0 {
		_, err := h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    msg.Chat.ID,
			MessageID: state.LastBotMessageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete template message: %v", err)
		}
		state.LastBotMessageID = 0
	}

	postType, err := h.postTypeRepo.GetByID(state.SelectedTypeID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get post type: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Ошибка получения типа поста",
		})
		return
	}

	state.DraftText = msg.Text
	state.DraftPhotoID = postType.PhotoID
	if len(msg.Entities) > 0 {
		entitiesJSON, _ := json.Marshal(msg.Entities)
		state.DraftEntities = string(entitiesJSON)
		// log.Printf("[FORUM_ADMIN] Received entities: %s", string(entitiesJSON))
	}
	state.CurrentState = fsm.StateNewPostConfirm
	err = h.adminStateRepo.Save(state)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Ошибка сохранения состояния",
		})
		return
	}

	previewPrefix := "Предпросмотр поста:\n\n"
	previewText := previewPrefix + msg.Text

	var previewEntities []tgmodels.MessageEntity
	if len(msg.Entities) > 0 {
		offsetAdjustment := utf16Length(previewPrefix)
		for _, entity := range msg.Entities {
			adjustedEntity := entity
			adjustedEntity.Offset += offsetAdjustment
			previewEntities = append(previewEntities, adjustedEntity)
		}
	}

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

	if postType.PhotoID != "" {
		_, err = h.bot.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID:          msg.Chat.ID,
			Photo:           &tgmodels.InputFileString{Data: postType.PhotoID},
			Caption:         previewText,
			ReplyMarkup:     keyboard,
			CaptionEntities: previewEntities,
		})
	} else {
		_, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      msg.Chat.ID,
			Text:        previewText,
			ReplyMarkup: keyboard,
			Entities:    previewEntities,
		})
	}

	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send preview: %v", err)
	}

	log.Printf("[FORUM_ADMIN] Preview shown to user %d, state set to StateNewPostConfirm", msg.From.ID)
}

func (h *ForumAdminHandler) handlePostConfirmation(ctx context.Context, userID, chatID int64, messageID int) {
	state, err := h.adminStateRepo.Get(userID)
	if err != nil || state == nil || state.CurrentState != fsm.StateNewPostConfirm {
		log.Printf("[FORUM_ADMIN] Invalid state for confirmation: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка: неверное состояние",
		})
		return
	}

	config, err := h.adminConfigRepo.Get()
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get config: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка получения конфигурации",
		})
		return
	}

	var entities []tgmodels.MessageEntity
	if state.DraftEntities != "" {
		json.Unmarshal([]byte(state.DraftEntities), &entities)
		// log.Printf("[FORUM_ADMIN] Publishing with %d entities: %s", len(entities), state.DraftEntities)
	}

	var publishedMsg *tgmodels.Message
	if state.DraftPhotoID != "" {
		// log.Printf("[FORUM_ADMIN] Sending photo with caption and %d caption entities", len(entities))
		publishedMsg, err = h.bot.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID:          config.ForumChatID,
			MessageThreadID: int(config.TopicID),
			Photo:           &tgmodels.InputFileString{Data: state.DraftPhotoID},
			Caption:         state.DraftText,
			CaptionEntities: entities,
		})
	} else {
		// log.Printf("[FORUM_ADMIN] Sending message with %d entities", len(entities))
		publishedMsg, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          config.ForumChatID,
			MessageThreadID: int(config.TopicID),
			Text:            state.DraftText,
			Entities:        entities,
		})
	}

	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to publish post: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Не удалось опубликовать пост: %v", err),
		})
		return
	}

	publishedPost := &models.PublishedPost{
		PostTypeID: state.SelectedTypeID,
		ChatID:     config.ForumChatID,
		TopicID:    config.TopicID,
		MessageID:  int64(publishedMsg.ID),
		Text:       state.DraftText,
		PhotoID:    state.DraftPhotoID,
		Entities:   state.DraftEntities,
	}

	err = h.publishedPostRepo.Create(publishedPost)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save published post to DB: %v", err)
		h.adminStateRepo.Clear(userID)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("⚠️ Пост опубликован, но не удалось сохранить запись в БД: %v\nРедактирование и удаление через бота будет недоступно.", err),
		})
		h.showAdminMenu(ctx, chatID, 0)
		return
	}

	err = h.adminStateRepo.Clear(userID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to clear state: %v", err)
	}

	_, err = h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: messageID,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to delete confirmation message: %v", err)
	}

	h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "✅ Пост успешно опубликован!",
	})

	h.showAdminMenu(ctx, chatID, 0)

	log.Printf("[FORUM_ADMIN] Post published successfully by user %d, message ID: %d", userID, publishedMsg.ID)
}

func (h *ForumAdminHandler) handleEditPostLinkInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if state.LastBotMessageID > 0 {
		_, err := h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    msg.Chat.ID,
			MessageID: state.LastBotMessageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete prompt message: %v", err)
		}
		state.LastBotMessageID = 0
	}

	if msg.Text == "" {
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Пожалуйста, отправьте ссылку на пост",
		})
		return
	}

	post, err := h.postManager.GetPostByLink(msg.Text)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get post by link: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Неверный формат ссылки или пост не был создан этим ботом",
		})
		return
	}

	state.EditingPostID = post.ID
	state.CurrentState = fsm.StateEditPostEnterText
	err = h.adminStateRepo.Save(state)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Ошибка сохранения состояния",
		})
		return
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	previewText := fmt.Sprintf("Текущий текст поста:\n\n%s\n\nОтправьте новый текст.", post.Text)
	var previewEntities []tgmodels.MessageEntity
	if post.Entities != "" {
		var entities []tgmodels.MessageEntity
		if err := json.Unmarshal([]byte(post.Entities), &entities); err == nil {
			prefix := "Текущий текст поста:\n\n"
			offset := utf16Length(prefix)
			for _, e := range entities {
				e.Offset += offset
				previewEntities = append(previewEntities, e)
			}
		}
	}

	sentMsg, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      msg.Chat.ID,
		Text:        previewText,
		Entities:    previewEntities,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send post preview: %v", err)
	} else if sentMsg != nil {
		state.LastBotMessageID = sentMsg.ID
		h.adminStateRepo.Save(state)
	}

	log.Printf("[FORUM_ADMIN] Post %d found for editing by user %d", post.ID, msg.From.ID)
}

func (h *ForumAdminHandler) handleEditPostTextInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if state.LastBotMessageID > 0 {
		_, err := h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    msg.Chat.ID,
			MessageID: state.LastBotMessageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete prompt message: %v", err)
		}
		state.LastBotMessageID = 0
	}

	if msg.Text == "" {
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Пожалуйста, отправьте новый текст поста",
		})
		return
	}

	post, err := h.publishedPostRepo.GetByID(state.EditingPostID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get post: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Ошибка получения поста",
		})
		return
	}

	if post.PhotoID != "" {
		_, err = h.bot.EditMessageCaption(ctx, &bot.EditMessageCaptionParams{
			ChatID:          post.ChatID,
			MessageID:       int(post.MessageID),
			Caption:         msg.Text,
			CaptionEntities: msg.Entities,
		})
	} else {
		_, err = h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    post.ChatID,
			MessageID: int(post.MessageID),
			Text:      msg.Text,
			Entities:  msg.Entities,
		})
	}

	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to edit post in Telegram: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   fmt.Sprintf("❌ Не удалось отредактировать пост: %v", err),
		})
		return
	}

	post.Text = msg.Text
	if len(msg.Entities) > 0 {
		entitiesJSON, _ := json.Marshal(msg.Entities)
		post.Entities = string(entitiesJSON)
	} else {
		post.Entities = ""
	}
	err = h.publishedPostRepo.Update(post)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to update post in DB: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Ошибка сохранения изменений",
		})
		return
	}

	err = h.adminStateRepo.Clear(msg.From.ID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to clear state: %v", err)
	}

	h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   "✅ Пост успешно отредактирован!",
	})

	h.showAdminMenu(ctx, msg.Chat.ID, 0)

	log.Printf("[FORUM_ADMIN] Post %d edited successfully by user %d", post.ID, msg.From.ID)
}

func (h *ForumAdminHandler) handleDeletePostLinkInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if state.LastBotMessageID > 0 {
		_, err := h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    msg.Chat.ID,
			MessageID: state.LastBotMessageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete prompt message: %v", err)
		}
		state.LastBotMessageID = 0
	}

	sendError := func(text string) {
		sentMsg, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   text,
		})
		if err == nil && sentMsg != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
	}

	if msg.Text == "" {
		sendError("❌ Пожалуйста, отправьте ссылку на пост")
		return
	}

	post, err := h.postManager.GetPostByLink(msg.Text)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get post by link: %v", err)
		sendError("❌ Неверный формат ссылки или пост не был создан этим ботом")
		return
	}

	_, err = h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    post.ChatID,
		MessageID: int(post.MessageID),
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to delete post from Telegram: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   fmt.Sprintf("❌ Не удалось удалить пост: %v", err),
		})
		return
	}

	err = h.postManager.DeletePost(ctx, post.ID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to delete post from DB: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Ошибка удаления записи из базы данных",
		})
		return
	}

	err = h.adminStateRepo.Clear(msg.From.ID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to clear state: %v", err)
	}

	h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   "✅ Пост успешно удален!",
	})

	h.showAdminMenu(ctx, msg.Chat.ID, 0)

	log.Printf("[FORUM_ADMIN] Post %d deleted successfully by user %d", post.ID, msg.From.ID)
}

const postListPageSize = 10

func (h *ForumAdminHandler) showPostList(ctx context.Context, chatID int64, messageID int, page int) {
	total, err := h.publishedPostRepo.Count()
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to count posts: %v", err)
		return
	}

	offset := int64(page * postListPageSize)
	posts, err := h.publishedPostRepo.GetPaginated(postListPageSize, offset)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get paginated posts: %v", err)
		return
	}

	totalPages := int((total + postListPageSize - 1) / postListPageSize)
	if totalPages == 0 {
		totalPages = 1
	}

	var text string
	if total == 0 {
		text = "Список постов пуст"
	} else {
		text = fmt.Sprintf("Список постов (стр. %d/%d)", page+1, totalPages)
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: make([][]tgmodels.InlineKeyboardButton, 0),
	}

	for _, post := range posts {
		postType, err := h.postTypeRepo.GetByID(post.PostTypeID)
		var buttonText string
		if err == nil {
			typeLabel := postType.Name
			if postType.Emoji != "" {
				typeLabel = postType.Emoji + " " + postType.Name
			}
			buttonText = fmt.Sprintf("%s — %s", typeLabel, post.CreatedAt.Format("02.01.06 15:04"))
		} else {
			buttonText = fmt.Sprintf("#%d — %s", post.ID, post.CreatedAt.Format("02.01.06 15:04"))
		}
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []tgmodels.InlineKeyboardButton{
			{Text: buttonText, CallbackData: fmt.Sprintf("post_details:%d:%d", post.ID, page)},
		})
	}

	// Navigation row: [← Пред.] [← Назад] [След. →]
	var navRow []tgmodels.InlineKeyboardButton
	if totalPages > 1 && page > 0 {
		navRow = append(navRow, tgmodels.InlineKeyboardButton{
			Text:         "← Пред.",
			CallbackData: fmt.Sprintf("post_list_page:%d", page-1),
		})
	}
	navRow = append(navRow, tgmodels.InlineKeyboardButton{
		Text:         "Назад",
		CallbackData: "post_list_back",
	})
	if totalPages > 1 && page < totalPages-1 {
		navRow = append(navRow, tgmodels.InlineKeyboardButton{
			Text:         "След. →",
			CallbackData: fmt.Sprintf("post_list_page:%d", page+1),
		})
	}
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, navRow)

	if messageID > 0 {
		_, err = h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to edit post list: %v", err)
		}
	} else {
		_, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to send post list: %v", err)
		}
	}
}

func (h *ForumAdminHandler) showPostDetails(ctx context.Context, chatID int64, messageID int, postID int64, page int) {
	post, err := h.publishedPostRepo.GetByID(postID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get post %d: %v", postID, err)
		return
	}

	postType, err := h.postTypeRepo.GetByID(post.PostTypeID)
	var typeLabel string
	if err == nil {
		typeLabel = postType.Name
		if postType.Emoji != "" {
			typeLabel = postType.Emoji + " " + postType.Name
		}
	} else {
		typeLabel = fmt.Sprintf("ID %d", post.PostTypeID)
	}

	preview := post.Text
	if len([]rune(preview)) > 200 {
		runes := []rune(preview)
		preview = string(runes[:200]) + "..."
	}

	text := fmt.Sprintf("Пост #%d\nТип: %s\nДата: %s\n\nТекст:\n%s",
		post.ID,
		typeLabel,
		post.CreatedAt.Format("02.01.2006 15:04"),
		preview,
	)

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "✏️ Редактировать", CallbackData: fmt.Sprintf("post_list_edit:%d:%d", post.ID, page)},
			},
			{
				{Text: "🗑 Удалить", CallbackData: fmt.Sprintf("post_list_delete:%d:%d", post.ID, page)},
			},
			{
				{Text: "← Назад", CallbackData: fmt.Sprintf("post_list_page:%d", page)},
			},
		},
	}

	if messageID > 0 {
		_, err = h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to edit post details: %v", err)
		}
	} else {
		_, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to send post details: %v", err)
		}
	}
}

func (h *ForumAdminHandler) showDeletePostConfirm(ctx context.Context, chatID int64, messageID int, postID int64, page int) {
	text := "Удалить этот пост? Это действие нельзя отменить."

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "✅ Да, удалить", CallbackData: fmt.Sprintf("post_list_delete_confirm:%d:%d", postID, page)},
			},
			{
				{Text: "← Назад", CallbackData: fmt.Sprintf("post_details:%d:%d", postID, page)},
			},
		},
	}

	if messageID > 0 {
		_, err := h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to edit delete confirm: %v", err)
		}
	} else {
		_, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to send delete confirm: %v", err)
		}
	}
}

func (h *ForumAdminHandler) handleEditPostFromList(ctx context.Context, userID, chatID int64, messageID int, postID int64) {
	post, err := h.publishedPostRepo.GetByID(postID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get post %d: %v", postID, err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка получения поста",
		})
		return
	}

	err = h.adminStateRepo.Save(&models.AdminState{
		UserID:        userID,
		CurrentState:  fsm.StateEditPostEnterText,
		EditingPostID: post.ID,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		return
	}

	if messageID > 0 {
		_, err = h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: messageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete message: %v", err)
		}
	}

	previewText := fmt.Sprintf("Текущий текст поста:\n\n%s\n\nОтправьте новый текст.", post.Text)
	var previewEntities []tgmodels.MessageEntity
	if post.Entities != "" {
		var entities []tgmodels.MessageEntity
		if err := json.Unmarshal([]byte(post.Entities), &entities); err == nil {
			prefix := "Текущий текст поста:\n\n"
			offset := utf16Length(prefix)
			for _, e := range entities {
				e.Offset += offset
				previewEntities = append(previewEntities, e)
			}
		}
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	sentMsg, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        previewText,
		Entities:    previewEntities,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send edit prompt: %v", err)
	} else if sentMsg != nil {
		state, _ := h.adminStateRepo.Get(userID)
		if state != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
	}

	log.Printf("[FORUM_ADMIN] Edit from list: post %d, user %d", postID, userID)
}

func (h *ForumAdminHandler) handleDeletePostFromList(ctx context.Context, userID, chatID int64, messageID int, postID int64) {
	post, err := h.publishedPostRepo.GetByID(postID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get post %d: %v", postID, err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка получения поста",
		})
		return
	}

	_, err = h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    post.ChatID,
		MessageID: int(post.MessageID),
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to delete post from Telegram: %v", err)
	}

	err = h.postManager.DeletePost(ctx, post.ID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to delete post from DB: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка удаления поста из базы данных",
		})
		return
	}

	log.Printf("[FORUM_ADMIN] Post %d deleted from list by user %d", postID, userID)

	h.showAdminMenu(ctx, chatID, messageID)
}

// ─── Reply flow ───────────────────────────────────────────────────────────────

func (h *ForumAdminHandler) handleReplyStart(ctx context.Context, userID, chatID int64, messageID int) {
	state := &models.AdminState{
		UserID:       userID,
		CurrentState: fsm.StateReplyEnterLink,
	}
	if messageID > 0 {
		// When we edit an existing bot message as a prompt, remember it so it can be deleted
		// after user sends the link.
		state.LastBotMessageID = messageID
	}

	err := h.adminStateRepo.Save(state)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		return
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{{Text: "❌ Отмена", CallbackData: "cancel"}},
		},
	}

	if messageID > 0 {
		_, err = h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        "Отправьте ссылку на сообщение, на которое нужно ответить",
			ReplyMarkup: keyboard,
		})
	} else {
		var sentMsg *tgmodels.Message
		sentMsg, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "Отправьте ссылку на сообщение, на которое нужно ответить",
			ReplyMarkup: keyboard,
		})
		if err == nil && sentMsg != nil {
			state, _ := h.adminStateRepo.Get(userID)
			if state != nil {
				state.LastBotMessageID = sentMsg.ID
				h.adminStateRepo.Save(state)
			}
		}
	}
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to show reply start: %v", err)
	}
}

func (h *ForumAdminHandler) handleReplyLinkInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if state.LastBotMessageID > 0 {
		h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: msg.Chat.ID, MessageID: state.LastBotMessageID})
		state.LastBotMessageID = 0
	}

	sendError := func(text string) {
		sentMsg, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   text,
			ReplyMarkup: &tgmodels.InlineKeyboardMarkup{
				InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
					{{Text: "❌ Отмена", CallbackData: "cancel"}},
				},
			},
		})
		if err == nil && sentMsg != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
	}

	if msg.Text == "" {
		sendError("❌ Пожалуйста, отправьте ссылку на сообщение")
		return
	}

	chatID, messageID, threadID, err := h.postManager.ParsePostLinkFull(msg.Text)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to parse reply link: %v", err)
		sendError("❌ Неверный формат ссылки. Используйте ссылку вида https://t.me/c/<chat>/<message>")
		return
	}

	if chatID == 0 {
		config, err := h.adminConfigRepo.Get()
		if err == nil {
			chatID = config.ForumChatID
		}
	}

	state.ReplyTargetChatID = chatID
	state.ReplyTargetMessageID = messageID
	state.TempName = fmt.Sprintf("%d", threadID)
	state.CurrentState = fsm.StateReplyEnterText
	if err = h.adminStateRepo.Save(state); err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		return
	}

	sentMsg, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   "Отправьте текст ответа. Можно прикрепить фото к сообщению.",
		ReplyMarkup: &tgmodels.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
				{{Text: "❌ Отмена", CallbackData: "cancel"}},
			},
		},
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send reply text prompt: %v", err)
	} else if sentMsg != nil {
		state.LastBotMessageID = sentMsg.ID
		h.adminStateRepo.Save(state)
	}
}

func (h *ForumAdminHandler) handleReplyTextInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if state.LastBotMessageID > 0 {
		h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: msg.Chat.ID, MessageID: state.LastBotMessageID})
		state.LastBotMessageID = 0
	}

	text := msg.Text
	photoID := ""
	if msg.Photo != nil && len(msg.Photo) > 0 {
		photoID = msg.Photo[len(msg.Photo)-1].FileID
		if text == "" {
			text = msg.Caption
		}
	}

	if text == "" && photoID == "" {
		sentMsg, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Пожалуйста, отправьте текст или фото с текстом",
			ReplyMarkup: &tgmodels.InlineKeyboardMarkup{
				InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
					{{Text: "❌ Отмена", CallbackData: "cancel"}},
				},
			},
		})
		if err == nil && sentMsg != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
		return
	}

	entities := msg.Entities
	if photoID != "" {
		entities = msg.CaptionEntities
	}

	state.DraftText = text
	state.DraftPhotoID = photoID
	if len(entities) > 0 {
		entJSON, _ := json.Marshal(entities)
		state.DraftEntities = string(entJSON)
	} else {
		state.DraftEntities = ""
	}
	state.CurrentState = fsm.StateReplyConfirm
	if err := h.adminStateRepo.Save(state); err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		return
	}

	previewPrefix := "Предпросмотр ответа:\n\n"
	previewText := previewPrefix + text

	var previewEntities []tgmodels.MessageEntity
	if len(entities) > 0 {
		offset := utf16Length(previewPrefix)
		for _, e := range entities {
			e.Offset += offset
			previewEntities = append(previewEntities, e)
		}
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{{Text: "✅ Подтвердить", CallbackData: "confirm_reply"}},
			{{Text: "❌ Отмена", CallbackData: "cancel"}},
		},
	}

	var err error
	if photoID != "" {
		_, err = h.bot.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID:          msg.Chat.ID,
			Photo:           &tgmodels.InputFileString{Data: photoID},
			Caption:         previewText,
			CaptionEntities: previewEntities,
			ReplyMarkup:     keyboard,
		})
	} else {
		_, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      msg.Chat.ID,
			Text:        previewText,
			Entities:    previewEntities,
			ReplyMarkup: keyboard,
		})
	}
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send reply preview: %v", err)
	}
}

func (h *ForumAdminHandler) handleReplyConfirmation(ctx context.Context, userID, chatID int64, messageID int) {
	state, err := h.adminStateRepo.Get(userID)
	if err != nil || state == nil || state.CurrentState != fsm.StateReplyConfirm {
		log.Printf("[FORUM_ADMIN] Invalid state for reply confirmation")
		return
	}

	threadID := int64(0)
	if state.TempName != "" {
		threadID, _ = strconv.ParseInt(state.TempName, 10, 64)
	}

	var entities []tgmodels.MessageEntity
	if state.DraftEntities != "" {
		json.Unmarshal([]byte(state.DraftEntities), &entities)
	}

	sendAttempt := func(replyToID, useThreadID int64) (*tgmodels.Message, error) {
		replyParams := &tgmodels.ReplyParameters{MessageID: int(replyToID)}
		log.Printf("[FORUM_ADMIN] Reply send attempt: chat=%d thread=%d reply_to=%d", state.ReplyTargetChatID, useThreadID, replyToID)

		if state.DraftPhotoID != "" {
			return h.bot.SendPhoto(ctx, &bot.SendPhotoParams{
				ChatID:          state.ReplyTargetChatID,
				MessageThreadID: int(useThreadID),
				Photo:           &tgmodels.InputFileString{Data: state.DraftPhotoID},
				Caption:         state.DraftText,
				CaptionEntities: entities,
				ReplyParameters: replyParams,
			})
		}
		return h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          state.ReplyTargetChatID,
			MessageThreadID: int(useThreadID),
			Text:            state.DraftText,
			Entities:        entities,
			ReplyParameters: replyParams,
		})
	}

	var sentMsg *tgmodels.Message
	attempts := [][2]int64{{state.ReplyTargetMessageID, threadID}}
	if threadID > 0 && threadID != state.ReplyTargetMessageID {
		attempts = append(attempts,
			[2]int64{threadID, threadID},
			[2]int64{state.ReplyTargetMessageID, 0},
			[2]int64{threadID, 0},
		)
	}

	for _, a := range attempts {
		sentMsg, err = sendAttempt(a[0], a[1])
		if err == nil {
			state.ReplyTargetMessageID = a[0]
			break
		}
	}

	if err != nil || sentMsg == nil {
		log.Printf("[FORUM_ADMIN] Failed to send reply: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      fmt.Sprintf("❌ Не удалось отправить ответ: %v", err),
		})
		return
	}

	reply := &models.Reply{
		ChatID:           state.ReplyTargetChatID,
		ReplyToMessageID: state.ReplyTargetMessageID,
		MessageID:        int64(sentMsg.ID),
		Text:             state.DraftText,
		PhotoID:          state.DraftPhotoID,
		Entities:         state.DraftEntities,
	}
	if err := h.replyRepo.Create(reply); err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save reply to DB: %v", err)
	}

	h.adminStateRepo.Clear(userID)

	h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "✅ Ответ успешно отправлен!",
	})
	h.showAdminMenu(ctx, chatID, 0)

	log.Printf("[FORUM_ADMIN] Reply sent by user %d, saved as reply ID %d", userID, reply.ID)
}

const replyListPageSize = 10

func (h *ForumAdminHandler) showReplyList(ctx context.Context, chatID int64, messageID int, page int) {
	total, err := h.replyRepo.Count()
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to count replies: %v", err)
		return
	}

	offset := int64(page * replyListPageSize)
	replies, err := h.replyRepo.GetPaginated(replyListPageSize, offset)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get paginated replies: %v", err)
		return
	}

	totalPages := int((total + replyListPageSize - 1) / replyListPageSize)
	if totalPages == 0 {
		totalPages = 1
	}

	var text string
	if total == 0 {
		text = "Список ответов пуст"
	} else {
		text = fmt.Sprintf("Список ответов (стр. %d/%d)", page+1, totalPages)
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: make([][]tgmodels.InlineKeyboardButton, 0),
	}

	for _, reply := range replies {
		previewText := strings.TrimSpace(reply.Text)
		if previewText == "" && reply.PhotoID != "" {
			previewText = "Изображение"
		}
		preview := []rune(previewText)
		if len(preview) > 30 {
			preview = append(preview[:30], []rune("...")...)
		}
		buttonText := fmt.Sprintf("%s — %s", reply.CreatedAt.Format("02.01.06 15:04"), string(preview))
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []tgmodels.InlineKeyboardButton{
			{Text: buttonText, CallbackData: fmt.Sprintf("reply_details:%d:%d", reply.ID, page)},
		})
	}

	var navRow []tgmodels.InlineKeyboardButton
	if totalPages > 1 && page > 0 {
		navRow = append(navRow, tgmodels.InlineKeyboardButton{
			Text:         "← Пред.",
			CallbackData: fmt.Sprintf("reply_list_page:%d", page-1),
		})
	}
	navRow = append(navRow, tgmodels.InlineKeyboardButton{
		Text:         "← Назад",
		CallbackData: "reply_list_back",
	})
	if totalPages > 1 && page < totalPages-1 {
		navRow = append(navRow, tgmodels.InlineKeyboardButton{
			Text:         "След. →",
			CallbackData: fmt.Sprintf("reply_list_page:%d", page+1),
		})
	}
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, navRow)

	if messageID > 0 {
		_, err = h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			_, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:      chatID,
				Text:        text,
				ReplyMarkup: keyboard,
			})
		}
	} else {
		_, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
	}
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to show reply list: %v", err)
	}
}

func (h *ForumAdminHandler) showReplyDetails(ctx context.Context, chatID int64, messageID int, replyID int64, page int) {
	reply, err := h.replyRepo.GetByID(replyID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get reply %d: %v", replyID, err)
		return
	}

	displayText := reply.Text
	if reply.Entities == "" {
		preview := []rune(displayText)
		if len(preview) > 200 {
			displayText = string(append(preview[:200], []rune("...")...))
		}
	}

	prefix := fmt.Sprintf("Ответ #%d\nДата: %s\n\nТекст:\n",
		reply.ID,
		reply.CreatedAt.Format("02.01.2006 15:04"),
	)
	text := prefix + displayText

	var previewEntities []tgmodels.MessageEntity
	if reply.Entities != "" {
		var storedEntities []tgmodels.MessageEntity
		if err := json.Unmarshal([]byte(reply.Entities), &storedEntities); err != nil {
			log.Printf("[FORUM_ADMIN] Failed to parse reply entities for %d: %v", reply.ID, err)
		} else {
			prefixOffset := utf16Length(prefix)
			textLen := utf16Length(displayText)
			for _, e := range storedEntities {
				if e.Length <= 0 || e.Offset < 0 || e.Offset >= textLen {
					continue
				}
				if e.Offset+e.Length > textLen {
					e.Length = textLen - e.Offset
				}
				e.Offset += prefixOffset
				previewEntities = append(previewEntities, e)
			}
		}
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{{Text: "✏️ Редактировать", CallbackData: fmt.Sprintf("reply_list_edit:%d:%d", reply.ID, page)}},
			{{Text: "🗑 Удалить", CallbackData: fmt.Sprintf("reply_list_delete:%d:%d", reply.ID, page)}},
			{{Text: "← Назад", CallbackData: fmt.Sprintf("reply_list_page:%d", page)}},
		},
	}

	if reply.PhotoID != "" {
		captionPrefix := fmt.Sprintf("Ответ #%d\nДата: %s\n\nПодпись:\n",
			reply.ID,
			reply.CreatedAt.Format("02.01.2006 15:04"),
		)
		caption := captionPrefix
		if strings.TrimSpace(reply.Text) != "" {
			caption += reply.Text
		} else {
			caption += "—"
		}

		var captionEntities []tgmodels.MessageEntity
		if reply.Entities != "" {
			var storedEntities []tgmodels.MessageEntity
			if err := json.Unmarshal([]byte(reply.Entities), &storedEntities); err != nil {
				log.Printf("[FORUM_ADMIN] Failed to parse reply entities for %d: %v", reply.ID, err)
			} else {
				prefixOffset := utf16Length(captionPrefix)
				textLen := utf16Length(reply.Text)
				for _, e := range storedEntities {
					if e.Length <= 0 || e.Offset < 0 || e.Offset >= textLen {
						continue
					}
					if e.Offset+e.Length > textLen {
						e.Length = textLen - e.Offset
					}
					e.Offset += prefixOffset
					captionEntities = append(captionEntities, e)
				}
			}
		}

		if messageID > 0 {
			h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
				ChatID:    chatID,
				MessageID: messageID,
			})
		}
		_, err = h.bot.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID:          chatID,
			Photo:           &tgmodels.InputFileString{Data: reply.PhotoID},
			Caption:         caption,
			CaptionEntities: captionEntities,
			ReplyMarkup:     keyboard,
		})
	} else {
		if messageID > 0 {
			_, err = h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:      chatID,
				MessageID:   messageID,
				Text:        text,
				Entities:    previewEntities,
				ReplyMarkup: keyboard,
			})
			if err != nil {
				_, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
					ChatID:      chatID,
					Text:        text,
					Entities:    previewEntities,
					ReplyMarkup: keyboard,
				})
			}
		} else {
			_, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:      chatID,
				Text:        text,
				Entities:    previewEntities,
				ReplyMarkup: keyboard,
			})
		}
	}
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to show reply details: %v", err)
	}
}

func (h *ForumAdminHandler) showDeleteReplyConfirm(ctx context.Context, chatID int64, messageID int, replyID int64, page int) {
	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{{Text: "✅ Да, удалить", CallbackData: fmt.Sprintf("reply_list_delete_confirm:%d:%d", replyID, page)}},
			{{Text: "← Назад", CallbackData: fmt.Sprintf("reply_details:%d:%d", replyID, page)}},
		},
	}
	text := "Удалить этот ответ? Это действие нельзя отменить."

	if messageID > 0 {
		_, err := h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			_, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:      chatID,
				Text:        text,
				ReplyMarkup: keyboard,
			})
		}
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to edit delete reply confirm: %v", err)
		}
	} else {
		_, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to send delete reply confirm: %v", err)
		}
	}
}

func (h *ForumAdminHandler) handleEditReplyFromList(ctx context.Context, userID, chatID int64, messageID int, replyID int64) {
	reply, err := h.replyRepo.GetByID(replyID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get reply %d: %v", replyID, err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "❌ Ошибка получения ответа"})
		return
	}

	err = h.adminStateRepo.Save(&models.AdminState{
		UserID:        userID,
		CurrentState:  fsm.StateEditReplyEnterText,
		EditingPostID: reply.ID,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		return
	}

	if messageID > 0 {
		h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: chatID, MessageID: messageID})
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{{Text: "❌ Отмена", CallbackData: "cancel"}},
		},
	}

	var sentMsg *tgmodels.Message
	if reply.PhotoID != "" {
		previewCaption := "Текущий ответ (изображение):\n\n"
		if reply.Text != "" {
			previewCaption += reply.Text + "\n\n"
		}
		previewCaption += "Отправьте новый текст или фото с подписью."

		var previewCaptionEntities []tgmodels.MessageEntity
		if reply.Entities != "" {
			var ents []tgmodels.MessageEntity
			if err := json.Unmarshal([]byte(reply.Entities), &ents); err == nil {
				off := utf16Length("Текущий ответ (изображение):\n\n")
				for _, e := range ents {
					e.Offset += off
					previewCaptionEntities = append(previewCaptionEntities, e)
				}
			}
		}

		sentMsg, err = h.bot.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID:          chatID,
			Photo:           &tgmodels.InputFileString{Data: reply.PhotoID},
			Caption:         previewCaption,
			CaptionEntities: previewCaptionEntities,
			ReplyMarkup:     keyboard,
		})
	} else {
		previewText := fmt.Sprintf("Текущий текст ответа:\n\n%s\n\nОтправьте новый текст или фото с подписью.", reply.Text)
		var previewEntities []tgmodels.MessageEntity
		if reply.Entities != "" {
			var ents []tgmodels.MessageEntity
			if err := json.Unmarshal([]byte(reply.Entities), &ents); err == nil {
				prefix := "Текущий текст ответа:\n\n"
				off := utf16Length(prefix)
				for _, e := range ents {
					e.Offset += off
					previewEntities = append(previewEntities, e)
				}
			}
		}

		sentMsg, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        previewText,
			Entities:    previewEntities,
			ReplyMarkup: keyboard,
		})
	}
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send edit reply prompt: %v", err)
	} else if sentMsg != nil {
		state, _ := h.adminStateRepo.Get(userID)
		if state != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
	}
}

func (h *ForumAdminHandler) handleEditReplyTextInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if state.LastBotMessageID > 0 {
		h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: msg.Chat.ID, MessageID: state.LastBotMessageID})
		state.LastBotMessageID = 0
	}

	text := msg.Text
	entities := msg.Entities
	newPhotoID := ""
	if msg.Photo != nil && len(msg.Photo) > 0 {
		newPhotoID = msg.Photo[len(msg.Photo)-1].FileID
		if msg.Caption != "" || text == "" {
			text = msg.Caption
		}
		entities = msg.CaptionEntities
	}

	if text == "" && newPhotoID == "" {
		h.bot.SendMessage(ctx, &bot.SendMessageParams{ChatID: msg.Chat.ID, Text: "❌ Пожалуйста, отправьте новый текст или фото с подписью"})
		return
	}

	reply, err := h.replyRepo.GetByID(state.EditingPostID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get reply: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{ChatID: msg.Chat.ID, Text: "❌ Ошибка получения ответа"})
		return
	}

	if newPhotoID != "" {
		if reply.PhotoID == "" {
			h.bot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: msg.Chat.ID,
				Text:   "❌ Нельзя заменить текстовый ответ на фото. Удалите этот ответ и создайте новый.",
			})
			return
		}
		_, err = h.bot.EditMessageMedia(ctx, &bot.EditMessageMediaParams{
			ChatID:    reply.ChatID,
			MessageID: int(reply.MessageID),
			Media: &tgmodels.InputMediaPhoto{
				Media:           newPhotoID,
				Caption:         text,
				CaptionEntities: entities,
			},
		})
	} else if reply.PhotoID != "" {
		_, err = h.bot.EditMessageCaption(ctx, &bot.EditMessageCaptionParams{
			ChatID:          reply.ChatID,
			MessageID:       int(reply.MessageID),
			Caption:         text,
			CaptionEntities: entities,
		})
	} else {
		_, err = h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    reply.ChatID,
			MessageID: int(reply.MessageID),
			Text:      text,
			Entities:  entities,
		})
	}
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to edit reply in Telegram: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{ChatID: msg.Chat.ID, Text: fmt.Sprintf("❌ Не удалось отредактировать ответ: %v", err)})
		return
	}

	reply.Text = text
	if newPhotoID != "" {
		reply.PhotoID = newPhotoID
	}
	if len(entities) > 0 {
		entJSON, _ := json.Marshal(entities)
		reply.Entities = string(entJSON)
	} else {
		reply.Entities = ""
	}
	if err := h.replyRepo.Update(reply); err != nil {
		log.Printf("[FORUM_ADMIN] Failed to update reply in DB: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{ChatID: msg.Chat.ID, Text: "❌ Ошибка сохранения изменений"})
		return
	}

	h.adminStateRepo.Clear(state.UserID)

	h.bot.SendMessage(ctx, &bot.SendMessageParams{ChatID: msg.Chat.ID, Text: "✅ Ответ успешно отредактирован!"})
	h.showAdminMenu(ctx, msg.Chat.ID, 0)

	log.Printf("[FORUM_ADMIN] Reply %d edited by user %d", reply.ID, state.UserID)
}

func (h *ForumAdminHandler) handleDeleteReplyFromList(ctx context.Context, userID, chatID int64, messageID int, replyID int64) {
	reply, err := h.replyRepo.GetByID(replyID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get reply %d: %v", replyID, err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "❌ Ошибка получения ответа"})
		return
	}

	_, err = h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    reply.ChatID,
		MessageID: int(reply.MessageID),
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to delete reply from Telegram: %v", err)
	}

	if err := h.replyRepo.Delete(reply.ID); err != nil {
		log.Printf("[FORUM_ADMIN] Failed to delete reply from DB: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка удаления ответа из базы данных",
		})
		return
	}

	log.Printf("[FORUM_ADMIN] Reply %d deleted from list by user %d", replyID, userID)
	h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "✅ Ответ успешно удалён!",
	})
	h.showAdminMenu(ctx, chatID, 0)
}

// ─────────────────────────────────────────────────────────────────────────────

func (h *ForumAdminHandler) handleNewTypeStart(ctx context.Context, userID, chatID int64, messageID int) {
	err := h.adminStateRepo.Save(&models.AdminState{
		UserID:       userID,
		CurrentState: fsm.StateNewTypeEnterName,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка сохранения состояния",
		})
		return
	}

	text := "Введите название нового типа поста."

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	sentMsg, err := h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send new type prompt: %v", err)
	} else if sentMsg != nil {
		state, _ := h.adminStateRepo.Get(userID)
		if state != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
	}

	log.Printf("[FORUM_ADMIN] New type creation started for user %d", userID)
}

func (h *ForumAdminHandler) handleNewTypeNameInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if state.LastBotMessageID > 0 {
		_, err := h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    msg.Chat.ID,
			MessageID: state.LastBotMessageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete prompt message: %v", err)
		}
		state.LastBotMessageID = 0
	}

	if msg.Text == "" {
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Пожалуйста, введите название типа",
		})
		return
	}

	state.TempName = msg.Text
	state.CurrentState = fsm.StateNewTypeEnterEmoji
	err := h.adminStateRepo.Save(state)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Ошибка сохранения состояния",
		})
		return
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "⏭ Пропустить", CallbackData: "skip_emoji"},
			},
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	sentMsg, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      msg.Chat.ID,
		Text:        "Отправьте эмодзи для типа поста (будет отображаться на кнопке) или нажмите \"Пропустить\".",
		ReplyMarkup: keyboard,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send emoji prompt: %v", err)
	} else if sentMsg != nil {
		state.LastBotMessageID = sentMsg.ID
		h.adminStateRepo.Save(state)
	}

	log.Printf("[FORUM_ADMIN] Type name '%s' saved for user %d, waiting for emoji", msg.Text, msg.From.ID)
}

func (h *ForumAdminHandler) handleNewTypeEmojiInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if state.LastBotMessageID > 0 {
		_, err := h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    msg.Chat.ID,
			MessageID: state.LastBotMessageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete prompt message: %v", err)
		}
		state.LastBotMessageID = 0
	}

	if msg.Text == "" {
		sentMsg, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Пожалуйста, отправьте эмодзи",
		})
		if err == nil && sentMsg != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
		return
	}

	state.TempEmoji = msg.Text
	state.CurrentState = fsm.StateNewTypeEnterImage
	err := h.adminStateRepo.Save(state)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Ошибка сохранения состояния",
		})
		return
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "⏭ Пропустить", CallbackData: "skip_image"},
			},
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	sentMsg, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      msg.Chat.ID,
		Text:        "Отправьте изображение для типа поста или нажмите \"Пропустить\" если изображение не требуется.",
		ReplyMarkup: keyboard,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send image prompt: %v", err)
	} else if sentMsg != nil {
		state.LastBotMessageID = sentMsg.ID
		h.adminStateRepo.Save(state)
	}

	log.Printf("[FORUM_ADMIN] Emoji saved for user %d, waiting for image", msg.From.ID)
}

func (h *ForumAdminHandler) handleNewTypeImageInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if state.LastBotMessageID > 0 {
		_, err := h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    msg.Chat.ID,
			MessageID: state.LastBotMessageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete prompt message: %v", err)
		}
		state.LastBotMessageID = 0
	}

	var photoID string

	if len(msg.Photo) > 0 {
		photoID = msg.Photo[len(msg.Photo)-1].FileID
		state.TempPhotoID = photoID
	} else if msg.Text != "" {
		sentMsg, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Пожалуйста, отправьте изображение или нажмите \"Пропустить\"",
		})
		if err == nil && sentMsg != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
		return
	}

	state.CurrentState = fsm.StateNewTypeEnterTemplate
	err := h.adminStateRepo.Save(state)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Ошибка сохранения состояния",
		})
		return
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	sentMsg, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      msg.Chat.ID,
		Text:        "Введите текстовый шаблон для типа поста.",
		ReplyMarkup: keyboard,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send template prompt: %v", err)
	} else if sentMsg != nil {
		state.LastBotMessageID = sentMsg.ID
		h.adminStateRepo.Save(state)
	}

	log.Printf("[FORUM_ADMIN] Image saved for user %d, waiting for template", msg.From.ID)
}

func (h *ForumAdminHandler) handleNewTypeTemplateInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if state.LastBotMessageID > 0 {
		_, err := h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    msg.Chat.ID,
			MessageID: state.LastBotMessageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete prompt message: %v", err)
		}
		state.LastBotMessageID = 0
	}

	if msg.Text == "" {
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Пожалуйста, введите текстовый шаблон",
		})
		return
	}

	var templateEntities string
	if len(msg.Entities) > 0 {
		entitiesJSON, _ := json.Marshal(msg.Entities)
		templateEntities = string(entitiesJSON)
	}

	postType := &models.PostType{
		Name:             state.TempName,
		Emoji:            state.TempEmoji,
		PhotoID:          state.TempPhotoID,
		Template:         msg.Text,
		TemplateEntities: templateEntities,
		IsActive:         true,
	}

	err := h.postTypeRepo.Create(postType)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to create post type: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   fmt.Sprintf("❌ Ошибка создания типа: %v", err),
		})
		return
	}

	err = h.adminStateRepo.Clear(msg.From.ID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to clear state: %v", err)
	}

	h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   fmt.Sprintf("✅ Тип поста \"%s\" успешно создан!", postType.Name),
	})

	h.showAdminMenu(ctx, msg.Chat.ID, 0)

	log.Printf("[FORUM_ADMIN] Post type %d created successfully by user %d", postType.ID, msg.From.ID)
}

func (h *ForumAdminHandler) handleSkipEmojiCallback(ctx context.Context, userID, chatID int64, messageID int) {
	state, err := h.adminStateRepo.Get(userID)
	if err != nil || state == nil || state.CurrentState != fsm.StateNewTypeEnterEmoji {
		log.Printf("[FORUM_ADMIN] Invalid state for skip emoji: %v", err)
		return
	}

	state.TempEmoji = ""
	state.CurrentState = fsm.StateNewTypeEnterImage
	err = h.adminStateRepo.Save(state)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка сохранения состояния",
		})
		return
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "⏭ Пропустить", CallbackData: "skip_image"},
			},
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	sentMsg, err := h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        "Отправьте изображение для типа поста или нажмите \"Пропустить\" если изображение не требуется.",
		ReplyMarkup: keyboard,
	})
	if err == nil && sentMsg != nil {
		state.LastBotMessageID = sentMsg.ID
		h.adminStateRepo.Save(state)
	}

	log.Printf("[FORUM_ADMIN] Emoji skipped for user %d, waiting for image", userID)
}

func (h *ForumAdminHandler) handleSkipImageCallback(ctx context.Context, userID, chatID int64, messageID int) {
	state, err := h.adminStateRepo.Get(userID)
	if err != nil || state == nil || state.CurrentState != fsm.StateNewTypeEnterImage {
		log.Printf("[FORUM_ADMIN] Invalid state for skip image: %v", err)
		return
	}

	state.TempPhotoID = ""
	state.CurrentState = fsm.StateNewTypeEnterTemplate
	err = h.adminStateRepo.Save(state)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка сохранения состояния",
		})
		return
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	sentMsg, err := h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        "Введите текстовый шаблон для типа поста.",
		ReplyMarkup: keyboard,
	})
	if err == nil && sentMsg != nil {
		state.LastBotMessageID = sentMsg.ID
		h.adminStateRepo.Save(state)
	}

	log.Printf("[FORUM_ADMIN] Image skipped for user %d, waiting for template", userID)
}

func (h *ForumAdminHandler) handleManageTypesMenu(ctx context.Context, chatID int64, messageID int) {
	allTypes, err := h.postTypeRepo.GetAll()
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get all types: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка получения типов постов",
		})
		return
	}

	if len(allTypes) == 0 {
		keyboard := &tgmodels.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
				{
					{Text: "← Назад", CallbackData: "admin_settings"},
				},
			},
		}
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        "❌ Нет созданных типов постов. Создайте тип в настройках.",
			ReplyMarkup: keyboard,
		})
		return
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: make([][]tgmodels.InlineKeyboardButton, 0, len(allTypes)),
	}

	for _, pt := range allTypes {
		statusIcon := ""
		if !pt.IsActive {
			statusIcon = "❌"
		}
		buttonText := fmt.Sprintf("%s %s", statusIcon, pt.Name)
		if pt.Emoji != "" {
			buttonText = fmt.Sprintf("%s %s %s", statusIcon, pt.Emoji, pt.Name)
		}
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []tgmodels.InlineKeyboardButton{
			{
				Text:         buttonText,
				CallbackData: fmt.Sprintf("manage_type:%d", pt.ID),
			},
		})
	}

	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []tgmodels.InlineKeyboardButton{
		{Text: "← Назад", CallbackData: "admin_settings"},
	})

	text := "Выберите тип для управления:"

	_, err = h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send manage types menu: %v", err)
	}

	log.Printf("[FORUM_ADMIN] Manage types menu shown for chat %d", chatID)
}

func (h *ForumAdminHandler) handleTypeManagementOptions(ctx context.Context, userID, chatID int64, messageID int, typeID int64) {
	postType, err := h.postTypeRepo.GetByID(typeID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get post type: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка получения типа поста",
		})
		return
	}

	err = h.adminStateRepo.Save(&models.AdminState{
		UserID:        userID,
		CurrentState:  fsm.StateManageTypes,
		EditingTypeID: typeID,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка сохранения состояния",
		})
		return
	}

	toggleText := "🔴 Отключить"
	if !postType.IsActive {
		toggleText = "🟢 Включить"
	}

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "📝 Изменить название", CallbackData: fmt.Sprintf("edit_type_name:%d", typeID)},
			},
			{
				{Text: "✨ Заменить эмодзи", CallbackData: fmt.Sprintf("edit_type_emoji:%d", typeID)},
			},
			{
				{Text: "🖼 Заменить изображение", CallbackData: fmt.Sprintf("edit_type_image:%d", typeID)},
			},
			{
				{Text: "📄 Заменить шаблон", CallbackData: fmt.Sprintf("edit_type_template:%d", typeID)},
			},
			{
				{Text: toggleText, CallbackData: fmt.Sprintf("toggle_type_active:%d", typeID)},
			},
			{
				{Text: "← Назад", CallbackData: "settings_manage_types"},
			},
		},
	}

	text := fmt.Sprintf("Управление типом \"%s\"\n\nВыберите действие:", postType.Name)

	_, err = h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send type management options: %v", err)
	}

	log.Printf("[FORUM_ADMIN] Type management options shown for type %d to user %d", typeID, userID)
}

func (h *ForumAdminHandler) handleEditTypeNameStart(ctx context.Context, userID, chatID int64, messageID int, typeID int64) {
	postType, err := h.postTypeRepo.GetByID(typeID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get post type: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка получения типа поста",
		})
		return
	}

	err = h.adminStateRepo.Save(&models.AdminState{
		UserID:        userID,
		CurrentState:  fsm.StateEditTypeName,
		EditingTypeID: typeID,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка сохранения состояния",
		})
		return
	}

	text := fmt.Sprintf("Текущее название: \"%s\"\n\nВведите новое название.", postType.Name)

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	sentMsg, err := h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send edit name prompt: %v", err)
	} else if sentMsg != nil {
		state, _ := h.adminStateRepo.Get(userID)
		if state != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
	}

	log.Printf("[FORUM_ADMIN] Edit type name started for type %d by user %d", typeID, userID)
}

func (h *ForumAdminHandler) handleEditTypeNameInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if msg.Text == "" {
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Пожалуйста, введите новое название",
		})
		return
	}

	if state.LastBotMessageID > 0 {
		_, err := h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    msg.Chat.ID,
			MessageID: state.LastBotMessageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete prompt message: %v", err)
		}
		state.LastBotMessageID = 0
	}

	err := h.postTypeManager.UpdateTypeName(state.EditingTypeID, msg.Text)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to update type name: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   fmt.Sprintf("❌ Ошибка обновления названия: %v", err),
		})
		return
	}

	err = h.adminStateRepo.Clear(msg.From.ID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to clear state: %v", err)
	}

	h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   fmt.Sprintf("✅ Название типа обновлено на \"%s\"!", msg.Text),
	})

	h.showAdminMenu(ctx, msg.Chat.ID, 0)

	log.Printf("[FORUM_ADMIN] Type %d name updated to %q by user %d", state.EditingTypeID, msg.Text, msg.From.ID)
}

func (h *ForumAdminHandler) handleEditTypeEmojiStart(ctx context.Context, userID, chatID int64, messageID int, typeID int64) {
	postType, err := h.postTypeRepo.GetByID(typeID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get post type: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка получения типа поста",
		})
		return
	}

	err = h.adminStateRepo.Save(&models.AdminState{
		UserID:        userID,
		CurrentState:  fsm.StateEditTypeEmoji,
		EditingTypeID: typeID,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка сохранения состояния",
		})
		return
	}

	currentEmoji := "не установлен"
	if postType.Emoji != "" {
		currentEmoji = postType.Emoji
	}

	text := fmt.Sprintf("Текущий эмодзи: %s\n\nОтправьте новый эмодзи.", currentEmoji)

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	sentMsg, err := h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send edit emoji prompt: %v", err)
	} else if sentMsg != nil {
		state, _ := h.adminStateRepo.Get(userID)
		if state != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
	}

	log.Printf("[FORUM_ADMIN] Edit type emoji started for type %d by user %d", typeID, userID)
}

func (h *ForumAdminHandler) handleEditTypeEmojiInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if msg.Text == "" {
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Пожалуйста, отправьте эмодзи",
		})
		return
	}

	if state.LastBotMessageID > 0 {
		_, err := h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    msg.Chat.ID,
			MessageID: state.LastBotMessageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete prompt message: %v", err)
		}
		state.LastBotMessageID = 0
	}

	err := h.postTypeManager.UpdateTypeEmoji(state.EditingTypeID, msg.Text)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to update type emoji: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   fmt.Sprintf("❌ Ошибка обновления эмодзи: %v", err),
		})
		return
	}

	err = h.adminStateRepo.Clear(msg.From.ID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to clear state: %v", err)
	}

	h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   fmt.Sprintf("✅ Эмодзи типа обновлен на %s!", msg.Text),
	})

	h.showAdminMenu(ctx, msg.Chat.ID, 0)

	log.Printf("[FORUM_ADMIN] Type %d emoji updated to %q by user %d", state.EditingTypeID, msg.Text, msg.From.ID)
}

func (h *ForumAdminHandler) handleEditTypeImageStart(ctx context.Context, userID, chatID int64, messageID int, typeID int64) {
	postType, err := h.postTypeRepo.GetByID(typeID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get post type: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка получения типа поста",
		})
		return
	}

	err = h.adminStateRepo.Save(&models.AdminState{
		UserID:        userID,
		CurrentState:  fsm.StateEditTypeImage,
		EditingTypeID: typeID,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка сохранения состояния",
		})
		return
	}

	text := fmt.Sprintf("Тип: \"%s\"\n\nОтправьте новое изображение.", postType.Name)

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	var sentMsg *tgmodels.Message
	if postType.PhotoID != "" {
		sentMsg, err = h.bot.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID:      chatID,
			Photo:       &tgmodels.InputFileString{Data: postType.PhotoID},
			Caption:     text,
			ReplyMarkup: keyboard,
		})
	} else {
		sentMsg, err = h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
	}

	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send edit image prompt: %v", err)
	} else if sentMsg != nil {
		state, _ := h.adminStateRepo.Get(userID)
		if state != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
	}

	log.Printf("[FORUM_ADMIN] Edit type image started for type %d by user %d", typeID, userID)
}

func (h *ForumAdminHandler) handleEditTypeImageInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	var photoID string

	if len(msg.Photo) > 0 {
		photoID = msg.Photo[len(msg.Photo)-1].FileID
	} else {
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Пожалуйста, отправьте изображение",
		})
		return
	}

	if state.LastBotMessageID > 0 {
		_, err := h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    msg.Chat.ID,
			MessageID: state.LastBotMessageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete prompt message: %v", err)
		}
		state.LastBotMessageID = 0
	}

	err := h.postTypeManager.UpdateTypePhoto(state.EditingTypeID, photoID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to update type photo: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   fmt.Sprintf("❌ Ошибка обновления изображения: %v", err),
		})
		return
	}

	err = h.adminStateRepo.Clear(msg.From.ID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to clear state: %v", err)
	}

	h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   "✅ Изображение типа обновлено!",
	})

	h.showAdminMenu(ctx, msg.Chat.ID, 0)

	log.Printf("[FORUM_ADMIN] Type %d image updated by user %d", state.EditingTypeID, msg.From.ID)
}

func (h *ForumAdminHandler) handleEditTypeTemplateStart(ctx context.Context, userID, chatID int64, messageID int, typeID int64) {
	postType, err := h.postTypeRepo.GetByID(typeID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get post type: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка получения типа поста",
		})
		return
	}

	err = h.adminStateRepo.Save(&models.AdminState{
		UserID:        userID,
		CurrentState:  fsm.StateEditTypeTemplate,
		EditingTypeID: typeID,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка сохранения состояния",
		})
		return
	}

	text := fmt.Sprintf("Текущий шаблон для типа \"%s\":\n\n<pre>%s</pre>\n\nВведите новый шаблон.", postType.Name, postType.Template)

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	sentMsg, err := h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   tgmodels.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send edit template prompt: %v", err)
	} else if sentMsg != nil {
		state, _ := h.adminStateRepo.Get(userID)
		if state != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
	}

	log.Printf("[FORUM_ADMIN] Edit type template started for type %d by user %d", typeID, userID)
}

func (h *ForumAdminHandler) handleEditTypeTemplateInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if msg.Text == "" {
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Пожалуйста, введите новый шаблон",
		})
		return
	}

	if state.LastBotMessageID > 0 {
		_, err := h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    msg.Chat.ID,
			MessageID: state.LastBotMessageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete prompt message: %v", err)
		}
		state.LastBotMessageID = 0
	}

	postType, err := h.postTypeRepo.GetByID(state.EditingTypeID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get post type: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Ошибка получения типа поста",
		})
		return
	}

	postType.Template = msg.Text
	if len(msg.Entities) > 0 {
		entitiesJSON, _ := json.Marshal(msg.Entities)
		postType.TemplateEntities = string(entitiesJSON)
	} else {
		postType.TemplateEntities = ""
	}

	err = h.postTypeRepo.Update(postType)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to update type template: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   fmt.Sprintf("❌ Ошибка обновления шаблона: %v", err),
		})
		return
	}

	err = h.adminStateRepo.Clear(msg.From.ID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to clear state: %v", err)
	}

	h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   "✅ Шаблон типа обновлен!",
	})

	h.showAdminMenu(ctx, msg.Chat.ID, 0)

	log.Printf("[FORUM_ADMIN] Type %d template updated by user %d", state.EditingTypeID, msg.From.ID)
}

func (h *ForumAdminHandler) handleToggleTypeActive(ctx context.Context, userID, chatID int64, messageID int, typeID int64) {
	postType, err := h.postTypeRepo.GetByID(typeID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get post type: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка получения типа поста",
		})
		return
	}

	newActiveState := !postType.IsActive
	err = h.postTypeManager.SetTypeActive(typeID, newActiveState)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to toggle type active state: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      fmt.Sprintf("❌ Ошибка изменения статуса: %v", err),
		})
		return
	}

	statusText := "активирован"
	if !newActiveState {
		statusText = "деактивирован"
	}

	h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      fmt.Sprintf("✅ Тип \"%s\" %s!", postType.Name, statusText),
	})

	h.showAdminMenu(ctx, chatID, 0)

	log.Printf("[FORUM_ADMIN] Type %d active state toggled to %v by user %d", typeID, newActiveState, userID)
}

func (h *ForumAdminHandler) showAccessSettingsMenu(ctx context.Context, chatID int64, messageID int) {
	config, err := h.adminConfigRepo.Get()
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get config: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка получения конфигурации",
		})
		return
	}

	adminIDsStr := ""
	for i, id := range config.AdminIDs {
		if i > 0 {
			adminIDsStr += ", "
		}
		adminIDsStr += strconv.FormatInt(id, 10)
	}
	if adminIDsStr == "" {
		adminIDsStr = "не настроены"
	}

	forumIDStr := strconv.FormatInt(config.ForumChatID, 10)
	if config.ForumChatID == 0 {
		forumIDStr = "не настроен"
	}

	topicIDStr := strconv.FormatInt(config.TopicID, 10)
	if config.TopicID == 0 {
		topicIDStr = "не настроен"
	}

	text := fmt.Sprintf("Настройки доступа:\n\n"+
		"👥 ID администраторов: %s\n"+
		"💬 ID целевой группы: %s\n"+
		"📌 ID топика: %s\n\n"+
		"Выберите настройку для изменения:",
		adminIDsStr, forumIDStr, topicIDStr)

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "👥 ID администраторов", CallbackData: "access_edit_admins"},
			},
			{
				{Text: "💬 ID целевой группы", CallbackData: "access_edit_forum"},
			},
			{
				{Text: "📌 ID топика", CallbackData: "access_edit_topic"},
			},
			{
				{Text: "← Назад", CallbackData: "admin_settings"},
			},
		},
	}

	_, err = h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send access settings menu: %v", err)
	}

	log.Printf("[FORUM_ADMIN] Access settings menu shown for chat %d", chatID)
}

func (h *ForumAdminHandler) handleEditAdminIDsStart(ctx context.Context, userID, chatID int64, messageID int) {
	err := h.adminStateRepo.Save(&models.AdminState{
		UserID:       userID,
		CurrentState: fsm.StateEditAdminIDs,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка сохранения состояния",
		})
		return
	}

	config, err := h.adminConfigRepo.Get()
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get config: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка получения конфигурации",
		})
		return
	}

	adminIDsStr := ""
	for i, id := range config.AdminIDs {
		if i > 0 {
			adminIDsStr += ", "
		}
		adminIDsStr += strconv.FormatInt(id, 10)
	}
	if adminIDsStr == "" {
		adminIDsStr = "не настроены"
	}

	text := fmt.Sprintf("Текущие ID администраторов: %s\n\n"+
		"Отправьте ID администраторов через запятую (например: 123456789, 987654321)", adminIDsStr)

	keyboard := &tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: "❌ Отмена", CallbackData: "cancel"},
			},
		},
	}

	sentMsg, err := h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send edit admin IDs prompt: %v", err)
	} else if sentMsg != nil {
		state, _ := h.adminStateRepo.Get(userID)
		if state != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
	}

	log.Printf("[FORUM_ADMIN] Edit admin IDs started for user %d", userID)
}

func (h *ForumAdminHandler) handleEditAdminIDsInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if state.LastBotMessageID > 0 {
		_, err := h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    msg.Chat.ID,
			MessageID: state.LastBotMessageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete prompt message: %v", err)
		}
		state.LastBotMessageID = 0
	}

	sendError := func(text string) {
		sentMsg, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   text,
		})
		if err == nil && sentMsg != nil {
			state.LastBotMessageID = sentMsg.ID
			h.adminStateRepo.Save(state)
		}
	}

	if msg.Text == "" {
		sendError("❌ Пожалуйста, отправьте ID администраторов")
		return
	}

	parts := strings.Split(msg.Text, ",")
	adminIDs := []int64{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			sendError(fmt.Sprintf("❌ Неверный формат ID: %s", part))
			return
		}
		adminIDs = append(adminIDs, id)
	}

	if len(adminIDs) == 0 {
		sendError("❌ Необходимо указать хотя бы один ID администратора")
		return
	}

	selfIncluded := false
	for _, id := range adminIDs {
		if id == msg.From.ID {
			selfIncluded = true
			break
		}
	}
	if !selfIncluded {
		sendError(fmt.Sprintf("❌ Список должен содержать ваш ID (%d), иначе вы потеряете доступ к боту", msg.From.ID))
		return
	}

	config, err := h.adminConfigRepo.Get()
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get config: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Ошибка получения конфигурации",
		})
		return
	}

	config.AdminIDs = adminIDs
	err = h.adminConfigRepo.Save(config)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save config: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Ошибка сохранения конфигурации",
		})
		return
	}

	err = h.adminStateRepo.Clear(msg.From.ID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to clear state: %v", err)
	}

	h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   "✅ ID администраторов обновлены!",
	})

	h.showAdminMenu(ctx, msg.Chat.ID, 0)

	log.Printf("[FORUM_ADMIN] Admin IDs updated by user %d", msg.From.ID)
}

func (h *ForumAdminHandler) handleEditForumIDStart(ctx context.Context, userID, chatID int64, messageID int) {
	err := h.adminStateRepo.Save(&models.AdminState{
		UserID:       userID,
		CurrentState: fsm.StateEditForumID,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка сохранения состояния",
		})
		return
	}

	config, err := h.adminConfigRepo.Get()
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get config: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка получения конфигурации",
		})
		return
	}

	forumIDStr := strconv.FormatInt(config.ForumChatID, 10)
	if config.ForumChatID == 0 {
		forumIDStr = "не настроен"
	}

	text := fmt.Sprintf("Текущий ID целевой группы: %s\n\n"+
		"Отправьте новый ID целевой группы-форума (например: -1001234567890)\n"+
		"Или используйте /cancel для отмены.", forumIDStr)

	_, err = h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send edit forum ID prompt: %v", err)
	}

	log.Printf("[FORUM_ADMIN] Edit forum ID started for user %d", userID)
}

func (h *ForumAdminHandler) handleEditForumIDInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if msg.Text == "" {
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Пожалуйста, отправьте ID целевой группы",
		})
		return
	}

	forumID, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
	if err != nil {
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Неверный формат ID",
		})
		return
	}

	config, err := h.adminConfigRepo.Get()
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get config: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Ошибка получения конфигурации",
		})
		return
	}

	config.ForumChatID = forumID
	err = h.adminConfigRepo.Save(config)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save config: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Ошибка сохранения конфигурации",
		})
		return
	}

	err = h.adminStateRepo.Clear(msg.From.ID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to clear state: %v", err)
	}

	h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   "✅ ID целевой группы обновлен!",
	})

	h.showAdminMenu(ctx, msg.Chat.ID, 0)

	log.Printf("[FORUM_ADMIN] Forum ID updated to %d by user %d", forumID, msg.From.ID)
}

func (h *ForumAdminHandler) handleEditTopicIDStart(ctx context.Context, userID, chatID int64, messageID int) {
	err := h.adminStateRepo.Save(&models.AdminState{
		UserID:       userID,
		CurrentState: fsm.StateEditTopicID,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save state: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка сохранения состояния",
		})
		return
	}

	config, err := h.adminConfigRepo.Get()
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get config: %v", err)
		h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "❌ Ошибка получения конфигурации",
		})
		return
	}

	topicIDStr := strconv.FormatInt(config.TopicID, 10)
	if config.TopicID == 0 {
		topicIDStr = "не настроен"
	}

	text := fmt.Sprintf("Текущий ID топика: %s\n\n"+
		"Отправьте новый ID топика (например: 42)\n"+
		"Или используйте /cancel для отмены.", topicIDStr)

	_, err = h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
	})
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send edit topic ID prompt: %v", err)
	}

	log.Printf("[FORUM_ADMIN] Edit topic ID started for user %d", userID)
}

func (h *ForumAdminHandler) handleEditTopicIDInput(ctx context.Context, msg *tgmodels.Message, state *models.AdminState) {
	if msg.Text == "" {
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Пожалуйста, отправьте ID топика",
		})
		return
	}

	topicID, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
	if err != nil {
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Неверный формат ID",
		})
		return
	}

	config, err := h.adminConfigRepo.Get()
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to get config: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Ошибка получения конфигурации",
		})
		return
	}

	config.TopicID = topicID
	err = h.adminConfigRepo.Save(config)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to save config: %v", err)
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   "❌ Ошибка сохранения конфигурации",
		})
		return
	}

	err = h.adminStateRepo.Clear(msg.From.ID)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to clear state: %v", err)
	}

	h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   "✅ ID топика обновлен!",
	})

	h.showAdminMenu(ctx, msg.Chat.ID, 0)

	log.Printf("[FORUM_ADMIN] Topic ID updated to %d by user %d", topicID, msg.From.ID)
}

func (h *ForumAdminHandler) handleBackupCommand(ctx context.Context, userID, chatID int64, messageID int) {
	log.Printf("[FORUM_ADMIN] Backup command for user %d, chat %d", userID, chatID)

	var loadingMessageID int
	if messageID > 0 {
		_, err := h.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "⏳ Создание бэкапа...",
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to edit message: %v", err)
		}
		loadingMessageID = messageID
	} else {
		msg, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "⏳ Создание бэкапа...",
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to send message: %v", err)
		} else {
			loadingMessageID = msg.ID
		}
	}

	sqlDump, err := h.backupManager.CreateBackup()
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to create backup: %v", err)
		if loadingMessageID > 0 {
			h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
				ChatID:    chatID,
				MessageID: loadingMessageID,
			})
		}
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Ошибка при создании бэкапа: %v", err),
		})
		h.showAdminMenu(ctx, chatID, 0)
		return
	}

	err = h.backupManager.SendBackupToAdmin(ctx, userID, sqlDump)
	if err != nil {
		log.Printf("[FORUM_ADMIN] Failed to send backup: %v", err)
		if loadingMessageID > 0 {
			h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
				ChatID:    chatID,
				MessageID: loadingMessageID,
			})
		}
		h.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Ошибка при отправке файла: %v", err),
		})
		h.showAdminMenu(ctx, chatID, 0)
		return
	}

	if loadingMessageID > 0 {
		_, err = h.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: loadingMessageID,
		})
		if err != nil {
			log.Printf("[FORUM_ADMIN] Failed to delete loading message: %v", err)
		}
	}

	h.showAdminMenu(ctx, chatID, 0)

	log.Printf("[FORUM_ADMIN] Backup sent successfully to user %d", userID)
}
