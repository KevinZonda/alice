package connector

import (
	"context"
	"strings"
	"testing"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestApp_OnMessageReceive_GroupMentionMergesRecentPostMediaWindow(t *testing.T) {
	cfg := configForTest()
	cfg.FeishuBotOpenID = "ou_bot"
	app := NewApp(cfg, nil)

	postMediaEvent := &larkim.P2MessageReceiveV1{
		EventV2Base: &larkevent.EventV2Base{Header: &larkevent.EventHeader{EventID: "evt_post_media"}},
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: strPtr("ou_user_1"),
				},
			},
			Message: &larkim.EventMessage{
				MessageId:   strPtr("om_post_media"),
				MessageType: strPtr("post"),
				Content:     strPtr(`{"title":"","content":[[{"tag":"img","image_key":"img_post_123"}],[{"tag":"text","text":"刚发了一张图"}]]}`),
				ChatId:      strPtr("oc_chat"),
				ChatType:    strPtr("group"),
			},
		},
	}
	if err := app.onMessageReceive(context.Background(), postMediaEvent); err != nil {
		t.Fatalf("unexpected post media event error: %v", err)
	}
	if got := len(app.queue); got != 0 {
		t.Fatalf("expected queue len 0 after unmentioned post media, got %d", got)
	}

	mentionEvent := &larkim.P2MessageReceiveV1{
		EventV2Base: &larkevent.EventV2Base{Header: &larkevent.EventHeader{EventID: "evt_post_mention"}},
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: strPtr("ou_user_1"),
				},
			},
			Message: &larkim.EventMessage{
				MessageId:   strPtr("om_post_mention"),
				MessageType: strPtr("text"),
				Content:     strPtr(`{"text":"<at user_id=\"ou_bot\">Alice</at> 看下刚才这张图"}`),
				ChatId:      strPtr("oc_chat"),
				ChatType:    strPtr("group"),
				Mentions: []*larkim.MentionEvent{
					{
						Id: &larkim.UserId{
							OpenId: strPtr("ou_bot"),
						},
					},
				},
			},
		},
	}
	if err := app.onMessageReceive(context.Background(), mentionEvent); err != nil {
		t.Fatalf("unexpected mention event error: %v", err)
	}

	if got := len(app.queue); got != 1 {
		t.Fatalf("expected queue len 1, got %d", got)
	}
	job := <-app.queue
	if len(job.Attachments) != 1 {
		t.Fatalf("expected merged attachments count 1, got %d", len(job.Attachments))
	}
	if job.Attachments[0].SourceMessageID != "om_post_media" {
		t.Fatalf("unexpected merged source message id: %s", job.Attachments[0].SourceMessageID)
	}
	if !strings.Contains(job.Text, "已自动合并你在过去5分钟发送的1条多媒体消息") {
		t.Fatalf("expected merge hint in text, got: %q", job.Text)
	}
}
