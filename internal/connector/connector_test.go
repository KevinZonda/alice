package connector

import (
	"context"
	"errors"
	"testing"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestBuildJob_TextMessage(t *testing.T) {
	event := &larkim.P2MessageReceiveV1{
		EventV2Base: &larkevent.EventV2Base{Header: &larkevent.EventHeader{EventID: "evt_1"}},
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: strPtr("text"),
				Content:     strPtr(`{"text":"<at user_id=\"ou_x\">Tom</at> 你好"}`),
				ChatId:      strPtr("oc_chat"),
			},
		},
	}

	job, err := BuildJob(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.ReceiveID != "oc_chat" || job.ReceiveIDType != "chat_id" {
		t.Fatalf("unexpected receive target: %+v", job)
	}
	if job.Text != "你好" {
		t.Fatalf("unexpected text: %q", job.Text)
	}
	if job.EventID != "evt_1" {
		t.Fatalf("unexpected event id: %s", job.EventID)
	}
}

func TestBuildJob_IgnoreNonText(t *testing.T) {
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: strPtr("image"),
				Content:     strPtr(`{"image_key":"abc"}`),
			},
		},
	}

	_, err := BuildJob(event)
	if !errors.Is(err, ErrIgnoreMessage) {
		t.Fatalf("expected ErrIgnoreMessage, got: %v", err)
	}
}

func TestProcessor_FallbackMessageOnCodexFailure(t *testing.T) {
	fakeCodex := codexStub{err: errors.New("boom")}
	sender := &senderStub{}
	processor := NewProcessor(fakeCodex, sender, "Codex 暂时不可用，请稍后重试。")

	processor.ProcessJob(context.Background(), Job{
		ReceiveID:     "oc_chat",
		ReceiveIDType: "chat_id",
		Text:          "hello",
	})

	if sender.calls != 1 {
		t.Fatalf("expected 1 send call, got %d", sender.calls)
	}
	if sender.lastText != "Codex 暂时不可用，请稍后重试。" {
		t.Fatalf("unexpected fallback text: %s", sender.lastText)
	}
}

type codexStub struct {
	resp string
	err  error
}

func (c codexStub) Run(_ context.Context, _ string) (string, error) {
	return c.resp, c.err
}

type senderStub struct {
	calls       int
	lastType    string
	lastReceive string
	lastText    string
}

func (s *senderStub) SendText(_ context.Context, receiveIDType, receiveID, text string) error {
	s.calls++
	s.lastType = receiveIDType
	s.lastReceive = receiveID
	s.lastText = text
	return nil
}

func strPtr(s string) *string { return &s }
