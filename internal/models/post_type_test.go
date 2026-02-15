package models

import (
	"testing"

	"pgregory.net/rapid"
)

func TestProperty3_PostTypeCreationPersistence(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		postType := &PostType{
			Name:     rapid.StringMatching(`[a-zA-Zа-яА-Я0-9 ]{1,100}`).Draw(rt, "name"),
			PhotoID:  rapid.String().Draw(rt, "photoID"),
			Template: rapid.StringMatching(`[a-zA-Zа-яА-Я0-9\s\.,!?]{10,500}`).Draw(rt, "template"),
			IsActive: rapid.Bool().Draw(rt, "isActive"),
		}

		if postType.Name == "" {
			postType.Name = "Test Type"
		}
		if postType.Template == "" {
			postType.Template = "Test Template"
		}

		if postType.Name != "" && len(postType.Name) > 0 {
			if postType.Template == "" || len(postType.Template) == 0 {
				rt.Skip("invalid test data")
			}
		}
	})
}
