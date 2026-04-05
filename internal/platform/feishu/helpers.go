package feishu

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var mentionPattern = regexp.MustCompile(`<at[^>]*>.*?</at>`)

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// extractText parses a Feishu "text" message content JSON and returns the
// plain text with mention tags stripped out.
func extractText(content *string) (string, error) {
	raw := strings.TrimSpace(deref(content))
	if raw == "" {
		return "", fmt.Errorf("empty text content")
	}
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", fmt.Errorf("invalid text content json: %w", err)
	}
	text := mentionPattern.ReplaceAllString(payload.Text, "")
	text = strings.TrimSpace(text)
	return text, nil
}
