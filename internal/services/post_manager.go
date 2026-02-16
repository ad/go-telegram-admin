package services

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/ad/go-telegram-admin/internal/db"
	"github.com/ad/go-telegram-admin/internal/models"
)

type PostManager struct {
	postRepo     *db.PublishedPostRepository
	postTypeRepo *db.PostTypeRepository
	configRepo   *db.AdminConfigRepository
}

func NewPostManager(
	postRepo *db.PublishedPostRepository,
	postTypeRepo *db.PostTypeRepository,
	configRepo *db.AdminConfigRepository,
) *PostManager {
	return &PostManager{
		postRepo:     postRepo,
		postTypeRepo: postTypeRepo,
		configRepo:   configRepo,
	}
}

func (pm *PostManager) CreatePost(ctx context.Context, postTypeID int64, text string) (*models.PublishedPost, error) {
	postType, err := pm.postTypeRepo.GetByID(postTypeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get post type: %w", err)
	}

	config, err := pm.configRepo.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	post := &models.PublishedPost{
		PostTypeID: postTypeID,
		ChatID:     config.ForumChatID,
		TopicID:    config.TopicID,
		MessageID:  0,
		Text:       text,
		PhotoID:    postType.PhotoID,
	}

	return post, nil
}

func (pm *PostManager) EditPost(ctx context.Context, postID int64, newText string) error {
	post, err := pm.postRepo.GetByID(postID)
	if err != nil {
		return fmt.Errorf("failed to get post: %w", err)
	}

	post.Text = newText
	return pm.postRepo.Update(post)
}

func (pm *PostManager) DeletePost(ctx context.Context, postID int64) error {
	return pm.postRepo.Delete(postID)
}

func (pm *PostManager) ParsePostLink(link string) (chatID, messageID int64, err error) {
	privateChannelWithTopicPattern := `(?:t\.me|telegram\.me)/c/(\d+)/\d+/(\d+)`
	re := regexp.MustCompile(privateChannelWithTopicPattern)
	matches := re.FindStringSubmatch(link)

	if len(matches) == 3 {
		chatIDStr := matches[1]
		messageIDStr := matches[2]

		if chatID, err = strconv.ParseInt(chatIDStr, 10, 64); err == nil {
			if messageID, err = strconv.ParseInt(messageIDStr, 10, 64); err == nil {
				if chatID > 0 {
					chatID = -1000000000000 - chatID
				}
				return chatID, messageID, nil
			}
		}
	}

	privateChannelPattern := `(?:t\.me|telegram\.me)/c/(\d+)/(\d+)`
	re = regexp.MustCompile(privateChannelPattern)
	matches = re.FindStringSubmatch(link)

	if len(matches) == 3 {
		chatIDStr := matches[1]
		messageIDStr := matches[2]

		if chatID, err = strconv.ParseInt(chatIDStr, 10, 64); err == nil {
			if messageID, err = strconv.ParseInt(messageIDStr, 10, 64); err == nil {
				if chatID > 0 {
					chatID = -1000000000000 - chatID
				}
				return chatID, messageID, nil
			}
		}
	}

	publicChannelPattern := `(?:t\.me|telegram\.me)/([^/]+)/(\d+)`
	re = regexp.MustCompile(publicChannelPattern)
	matches = re.FindStringSubmatch(link)

	if len(matches) == 3 {
		messageIDStr := matches[2]
		if messageID, err = strconv.ParseInt(messageIDStr, 10, 64); err == nil {
			return 0, messageID, nil
		}
	}

	return 0, 0, fmt.Errorf("invalid post link format")
}

// ParsePostLinkFull parses a Telegram message link and returns chatID, messageID and threadID.
// threadID is non-zero when the link points to a message inside a forum topic.
func (pm *PostManager) ParsePostLinkFull(link string) (chatID, messageID, threadID int64, err error) {
	// Private channel with topic: t.me/c/{chatID}/{topicID}/{messageID}
	privateChannelWithTopicPattern := `(?:t\.me|telegram\.me)/c/(\d+)/(\d+)/(\d+)`
	re := regexp.MustCompile(privateChannelWithTopicPattern)
	matches := re.FindStringSubmatch(link)

	if len(matches) == 4 {
		var cid, tid, mid int64
		if cid, err = strconv.ParseInt(matches[1], 10, 64); err == nil {
			if tid, err = strconv.ParseInt(matches[2], 10, 64); err == nil {
				if mid, err = strconv.ParseInt(matches[3], 10, 64); err == nil {
					if cid > 0 {
						cid = -1000000000000 - cid
					}
					// topic=1 is the General topic; Bot API expects no MessageThreadID for it
					if tid == 1 {
						tid = 0
					}
					return cid, mid, tid, nil
				}
			}
		}
	}

	// Private channel without topic: t.me/c/{chatID}/{messageID}
	privateChannelPattern := `(?:t\.me|telegram\.me)/c/(\d+)/(\d+)`
	re = regexp.MustCompile(privateChannelPattern)
	matches = re.FindStringSubmatch(link)

	if len(matches) == 3 {
		var cid, mid int64
		if cid, err = strconv.ParseInt(matches[1], 10, 64); err == nil {
			if mid, err = strconv.ParseInt(matches[2], 10, 64); err == nil {
				if cid > 0 {
					cid = -1000000000000 - cid
				}
				return cid, mid, 0, nil
			}
		}
	}

	// Public channel: t.me/{username}/{messageID}
	publicChannelPattern := `(?:t\.me|telegram\.me)/([^/]+)/(\d+)`
	re = regexp.MustCompile(publicChannelPattern)
	matches = re.FindStringSubmatch(link)

	if len(matches) == 3 {
		var mid int64
		if mid, err = strconv.ParseInt(matches[2], 10, 64); err == nil {
			return 0, mid, 0, nil
		}
	}

	return 0, 0, 0, fmt.Errorf("invalid post link format")
}

func (pm *PostManager) GetPostByLink(link string) (*models.PublishedPost, error) {
	chatID, messageID, err := pm.ParsePostLink(link)
	if err != nil {
		return nil, err
	}

	if chatID == 0 {
		config, err := pm.configRepo.Get()
		if err != nil {
			return nil, fmt.Errorf("failed to get config: %w", err)
		}
		chatID = config.ForumChatID
	}

	return pm.postRepo.GetByMessageID(chatID, messageID)
}
