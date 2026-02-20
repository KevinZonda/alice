package connector

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"gitee.com/alicespace/alice/internal/logging"
)

func BuildJob(event *larkim.P2MessageReceiveV1) (*Job, error) {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil, ErrIgnoreMessage
	}

	message := event.Event.Message
	if strings.ToLower(deref(message.MessageType)) != "text" {
		return nil, ErrIgnoreMessage
	}

	text, err := extractText(message.Content)
	if err != nil {
		return nil, err
	}
	if text == "" {
		return nil, ErrIgnoreMessage
	}

	receiveID := strings.TrimSpace(deref(message.ChatId))
	receiveIDType := "chat_id"
	if receiveID == "" {
		receiveID = strings.TrimSpace(extractOpenID(event))
		receiveIDType = "open_id"
	}
	if receiveID == "" {
		return nil, errors.New("missing receive target")
	}

	return &Job{
		ReceiveID:            receiveID,
		ReceiveIDType:        receiveIDType,
		SourceMessageID:      strings.TrimSpace(deref(message.MessageId)),
		ReplyParentMessageID: extractReplyParentMessageID(message),
		Text:                 text,
		EventID:              eventID(event),
		ReceivedAt:           time.Now(),
		SessionKey:           buildSessionKey(receiveIDType, receiveID),
	}, nil
}

func logIncomingEventDebug(event *larkim.P2MessageReceiveV1) {
	if !logging.IsDebugEnabled() {
		return
	}
	if event == nil || event.Event == nil || event.Event.Message == nil {
		logging.Debugf("incoming message source=feishu_im event=<nil>")
		return
	}

	message := event.Event.Message
	logging.Debugf(
		"incoming message source=feishu_im event_id=%s message_id=%s message_type=%s chat_id=%s raw_content=%s",
		eventID(event),
		strings.TrimSpace(deref(message.MessageId)),
		strings.TrimSpace(deref(message.MessageType)),
		strings.TrimSpace(deref(message.ChatId)),
		deref(message.Content),
	)
}

func buildSessionKey(receiveIDType, receiveID string) string {
	idType := strings.TrimSpace(receiveIDType)
	if idType == "" {
		idType = "unknown"
	}

	id := strings.TrimSpace(receiveID)
	if id == "" {
		return ""
	}
	return idType + ":" + id
}

func extractText(content *string) (string, error) {
	if strings.TrimSpace(deref(content)) == "" {
		return "", ErrIgnoreMessage
	}

	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(deref(content)), &payload); err != nil {
		return "", fmt.Errorf("invalid text content json: %w", err)
	}

	text := mentionPattern.ReplaceAllString(payload.Text, "")
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ErrIgnoreMessage
	}
	return text, nil
}

func extractOpenID(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil || event.Event.Sender == nil || event.Event.Sender.SenderId == nil {
		return ""
	}
	return deref(event.Event.Sender.SenderId.OpenId)
}

func extractReplyParentMessageID(message *larkim.EventMessage) string {
	if message == nil {
		return ""
	}
	parentID := strings.TrimSpace(deref(message.ParentId))
	if parentID != "" {
		return parentID
	}
	return strings.TrimSpace(deref(message.RootId))
}

func eventID(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.EventV2Base == nil || event.EventV2Base.Header == nil {
		return ""
	}
	return event.EventV2Base.Header.EventID
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
