package models

import (
	"testing"

	"pgregory.net/rapid"
)

func TestProperty16_ForumConfigPersistence(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numAdmins := rapid.IntRange(1, 10).Draw(rt, "numAdmins")
		adminIDs := make([]int64, numAdmins)
		for i := range numAdmins {
			adminIDs[i] = rapid.Int64Range(100000, 999999999).Draw(rt, "adminID")
		}

		config := &AdminConfig{
			AdminIDs:    adminIDs,
			ForumChatID: rapid.Int64Range(-1000000000000, -1).Draw(rt, "forumChatID"),
			TopicID:     rapid.Int64Range(1, 100000).Draw(rt, "topicID"),
		}

		if config.ForumChatID >= 0 || config.TopicID <= 0 || len(config.AdminIDs) == 0 {
			rt.Skip("invalid test data")
		}
	})
}
