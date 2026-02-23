package connector

import (
	"encoding/json"
	"strings"
)

func extractMentionUserIDs(content *string) []string {
	rawContent := strings.TrimSpace(deref(content))
	if rawContent == "" {
		return nil
	}

	if userIDs := extractTextMentionUserIDs(rawContent); len(userIDs) > 0 {
		return userIDs
	}
	return extractPostMentionUserIDs(rawContent)
}

func extractTextMentionUserIDs(rawContent string) []string {
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(rawContent), &payload); err != nil {
		return nil
	}

	matches := mentionUserIDPattern.FindAllStringSubmatch(payload.Text, -1)
	if len(matches) == 0 {
		return nil
	}

	userIDs := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		userID := strings.TrimSpace(match[1])
		if userID == "" {
			continue
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs
}
