package connector

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type postMessagePayload struct {
	Title   string                `json:"title"`
	Content [][]postMessageInline `json:"content"`
}

type postMessageInline struct {
	Tag      string `json:"tag"`
	Text     string `json:"text"`
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	ImageKey string `json:"image_key"`
	FileKey  string `json:"file_key"`
}

func extractPostContentWithMentions(content *string, mentions []*larkim.MentionEvent) (string, []Attachment, error) {
	payload, err := decodePostPayload(content)
	if err != nil {
		return "", nil, err
	}

	lines := make([]string, 0, len(payload.Content)+1)
	if title := strings.TrimSpace(payload.Title); title != "" {
		lines = append(lines, title)
	}

	attachments := make([]Attachment, 0, 2)
	for _, row := range payload.Content {
		var rowBuilder strings.Builder
		for _, inline := range row {
			switch strings.ToLower(strings.TrimSpace(inline.Tag)) {
			case "text", "a":
				rowBuilder.WriteString(inline.Text)
			case "img":
				imageKey := strings.TrimSpace(inline.ImageKey)
				fileKey := strings.TrimSpace(inline.FileKey)
				if imageKey == "" && fileKey == "" {
					continue
				}
				attachments = append(attachments, Attachment{
					Kind:     "image",
					ImageKey: imageKey,
					FileKey:  fileKey,
				})
			}
		}
		if text := strings.TrimSpace(rowBuilder.String()); text != "" {
			lines = append(lines, text)
		}
	}

	text := strings.TrimSpace(strings.Join(lines, "\n"))
	text = mentionPattern.ReplaceAllString(text, "")
	text = strings.TrimSpace(stripMentionKeys(text, mentions))
	if text == "" && len(attachments) > 0 {
		text = "用户发送了一条富文本消息。"
	}
	if text == "" && len(attachments) == 0 {
		return "", nil, ErrIgnoreMessage
	}
	return text, attachments, nil
}

func decodePostPayload(content *string) (postMessagePayload, error) {
	return decodePostPayloadRaw(strings.TrimSpace(deref(content)))
}

func decodePostPayloadRaw(raw string) (postMessagePayload, error) {
	if raw == "" {
		return postMessagePayload{}, ErrIgnoreMessage
	}

	var direct postMessagePayload
	if err := json.Unmarshal([]byte(raw), &direct); err != nil {
		return postMessagePayload{}, fmt.Errorf("invalid post content json: %w", err)
	}
	if hasPostContent(direct) {
		return direct, nil
	}

	var localized map[string]postMessagePayload
	if err := json.Unmarshal([]byte(raw), &localized); err != nil {
		return direct, nil
	}
	for _, locale := range []string{"zh_cn", "en_us"} {
		if payload, ok := localized[locale]; ok && hasPostContent(payload) {
			return payload, nil
		}
	}
	for _, payload := range localized {
		if hasPostContent(payload) {
			return payload, nil
		}
	}
	return direct, nil
}

func hasPostContent(payload postMessagePayload) bool {
	if strings.TrimSpace(payload.Title) != "" {
		return true
	}
	for _, row := range payload.Content {
		for _, inline := range row {
			tag := strings.ToLower(strings.TrimSpace(inline.Tag))
			switch tag {
			case "text", "a":
				if strings.TrimSpace(inline.Text) != "" {
					return true
				}
			case "img":
				if strings.TrimSpace(inline.ImageKey) != "" || strings.TrimSpace(inline.FileKey) != "" {
					return true
				}
			case "at":
				if strings.TrimSpace(inline.UserID) != "" {
					return true
				}
			}
		}
	}
	return false
}

func extractPostMentions(raw string) []MentionedUser {
	payload, err := decodePostPayloadRaw(strings.TrimSpace(raw))
	if err != nil {
		if !errors.Is(err, ErrIgnoreMessage) {
			return nil
		}
		return nil
	}

	mentioned := make([]MentionedUser, 0, 2)
	seen := make(map[string]struct{})
	for _, row := range payload.Content {
		for _, inline := range row {
			if strings.ToLower(strings.TrimSpace(inline.Tag)) != "at" {
				continue
			}
			userID := strings.TrimSpace(inline.UserID)
			if userID == "" {
				continue
			}
			userName := strings.TrimSpace(inline.UserName)
			dedupeKey := userID + "\x00" + userName
			if _, ok := seen[dedupeKey]; ok {
				continue
			}
			seen[dedupeKey] = struct{}{}
			candidate := MentionedUser{Name: userName}
			switch {
			case strings.HasPrefix(userID, "ou_"):
				candidate.OpenID = userID
			case strings.HasPrefix(userID, "on_"):
				candidate.UnionID = userID
			default:
				candidate.UserID = userID
			}
			mentioned = append(mentioned, candidate)
		}
	}
	return mentioned
}

func extractPostMentionUserIDs(raw string) []string {
	postMentions := extractPostMentions(raw)
	if len(postMentions) == 0 {
		return nil
	}

	userIDs := make([]string, 0, len(postMentions))
	for _, mention := range postMentions {
		id := preferredID(mention.OpenID, mention.UserID, mention.UnionID)
		if id == "" {
			continue
		}
		userIDs = append(userIDs, id)
	}
	return userIDs
}
