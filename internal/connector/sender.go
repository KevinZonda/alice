package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type LarkSender struct {
	client *lark.Client
}

func NewLarkSender(client *lark.Client) *LarkSender {
	return &LarkSender{client: client}
}

func (s *LarkSender) SendText(ctx context.Context, receiveIDType, receiveID, text string) error {
	content := textMessageContent(text)

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveID).
			MsgType("text").
			Content(content).
			Build()).
		Build()

	resp, err := s.client.Im.V1.Message.Create(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("feishu api error code=%d msg=%s request_id=%s", resp.Code, resp.Msg, resp.RequestId())
	}
	return nil
}

func (s *LarkSender) ReplyText(ctx context.Context, sourceMessageID, text string) (string, error) {
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(sourceMessageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType("text").
			Content(textMessageContent(text)).
			ReplyInThread(false).
			Build()).
		Build()

	resp, err := s.client.Im.V1.Message.Reply(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("feishu api error code=%d msg=%s request_id=%s", resp.Code, resp.Msg, resp.RequestId())
	}
	if resp.Data == nil || resp.Data.MessageId == nil {
		return "", errors.New("reply success but response message_id is empty")
	}
	return strings.TrimSpace(*resp.Data.MessageId), nil
}

func (s *LarkSender) ReplyCard(ctx context.Context, sourceMessageID, cardContent string) (string, error) {
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(sourceMessageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType("interactive").
			Content(cardContent).
			ReplyInThread(false).
			Build()).
		Build()

	resp, err := s.client.Im.V1.Message.Reply(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("feishu api error code=%d msg=%s request_id=%s", resp.Code, resp.Msg, resp.RequestId())
	}
	if resp.Data == nil || resp.Data.MessageId == nil {
		return "", errors.New("reply card success but response message_id is empty")
	}
	return strings.TrimSpace(*resp.Data.MessageId), nil
}

func (s *LarkSender) PatchCard(ctx context.Context, messageID, cardContent string) error {
	req := larkim.NewPatchMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewPatchMessageReqBodyBuilder().
			Content(cardContent).
			Build()).
		Build()

	resp, err := s.client.Im.V1.Message.Patch(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("feishu api error code=%d msg=%s request_id=%s", resp.Code, resp.Msg, resp.RequestId())
	}
	return nil
}

func (s *LarkSender) GetMessageText(ctx context.Context, messageID string) (string, error) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return "", errors.New("message id is empty")
	}

	req := larkim.NewGetMessageReqBuilder().
		MessageId(messageID).
		Build()
	resp, err := s.client.Im.V1.Message.Get(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("feishu api error code=%d msg=%s request_id=%s", resp.Code, resp.Msg, resp.RequestId())
	}
	if resp.Data == nil || len(resp.Data.Items) == 0 || resp.Data.Items[0] == nil {
		return "", errors.New("get message success but items is empty")
	}

	item := resp.Data.Items[0]
	msgType := strings.ToLower(strings.TrimSpace(deref(item.MsgType)))
	content := ""
	if item.Body != nil {
		content = deref(item.Body.Content)
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return "", errors.New("get message success but content is empty")
	}

	switch msgType {
	case "text":
		text, err := extractText(&content)
		if err == nil {
			return text, nil
		}
	case "interactive":
		if text := extractReplyTextFromCard(content); text != "" {
			return text, nil
		}
	}

	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err == nil {
		text := strings.TrimSpace(payload.Text)
		if text != "" {
			text = mentionPattern.ReplaceAllString(text, "")
			text = strings.TrimSpace(text)
			if text != "" {
				return text, nil
			}
		}
	}
	return clipText(content, 1200), nil
}

func extractReplyTextFromCard(content string) string {
	var payload struct {
		Body struct {
			Elements []struct {
				Tag     string `json:"tag"`
				Content string `json:"content"`
			} `json:"elements"`
		} `json:"body"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return ""
	}

	const replyPrefix = "**回复**\n"
	for _, element := range payload.Body.Elements {
		if strings.ToLower(strings.TrimSpace(element.Tag)) != "markdown" {
			continue
		}
		md := strings.TrimSpace(element.Content)
		if md == "" {
			continue
		}
		if strings.HasPrefix(md, replyPrefix) {
			return strings.TrimSpace(strings.TrimPrefix(md, replyPrefix))
		}
	}
	return ""
}

func textMessageContent(text string) string {
	contentBytes, _ := json.Marshal(map[string]string{"text": text})
	return string(contentBytes)
}
