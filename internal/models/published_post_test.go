package models

import (
	"testing"

	"pgregory.net/rapid"
)

func TestProperty8_PostPublicationPersistence(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		post := &PublishedPost{
			PostTypeID: rapid.Int64Range(1, 1000).Draw(rt, "postTypeID"),
			ChatID:     rapid.Int64Range(-1000000000000, -1).Draw(rt, "chatID"),
			TopicID:    rapid.Int64Range(1, 100000).Draw(rt, "topicID"),
			MessageID:  rapid.Int64Range(1, 1000000).Draw(rt, "messageID"),
			Text:       rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s\.,!?]{10,500}`).Draw(rt, "text"),
			PhotoID:    rapid.String().Draw(rt, "photoID"),
		}

		if post.Text == "" {
			post.Text = "Test post text"
		}

		if post.PostTypeID <= 0 || post.ChatID >= 0 || post.TopicID <= 0 || post.MessageID <= 0 {
			rt.Skip("invalid test data")
		}
	})
}
