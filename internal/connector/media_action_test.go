package connector

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestProcessor_AppendsMediaActionGuideForMediaIntent(t *testing.T) {
	fakeCodex := &codexCaptureStub{resp: "ok"}
	sender := &senderStub{}
	processor := NewProcessor(fakeCodex, sender, "Codex 暂时不可用，请稍后重试。", "正在思考中...")

	processor.ProcessJob(context.Background(), Job{
		ReceiveID:     "oc_chat",
		ReceiveIDType: "chat_id",
		Text:          "请发送图片",
	})

	if !strings.Contains(fakeCodex.lastInput, "```alice_action```") {
		t.Fatalf("expected media action guide in codex input, got: %s", fakeCodex.lastInput)
	}
}

func TestParseMediaActionsReply_CodeFence(t *testing.T) {
	reply := "说明文本\n```alice_action\n{\"actions\":[{\"type\":\"send_image\",\"image_key\":\"img_1\"}],\"reply\":\"已发送\"}\n```"

	cleaned, actions, handled, err := parseMediaActionsReply(reply)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if !handled {
		t.Fatal("expected media action handled=true")
	}
	if cleaned != "说明文本" {
		t.Fatalf("unexpected cleaned text: %q", cleaned)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != mediaActionTypeSendImage || actions[0].ImageKey != "img_1" {
		t.Fatalf("unexpected action payload: %+v", actions[0])
	}
}

func TestParseMediaActionsReply_PlainEnvelope(t *testing.T) {
	reply := "{\"actions\":[{\"type\":\"send_file\",\"file_key\":\"file_1\"}],\"reply\":\"文件已发送\"}"

	cleaned, actions, handled, err := parseMediaActionsReply(reply)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if !handled {
		t.Fatal("expected media action handled=true")
	}
	if cleaned != "文件已发送" {
		t.Fatalf("unexpected cleaned text: %q", cleaned)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != mediaActionTypeSendFile || actions[0].FileKey != "file_1" {
		t.Fatalf("unexpected action payload: %+v", actions[0])
	}
}

func TestProcessor_NoSourceExecutesMediaAction(t *testing.T) {
	fakeCodex := codexStub{resp: "```alice_action\n{\"actions\":[{\"type\":\"send_image\",\"image_key\":\"img_123\"}]}\n```"}
	sender := &senderStub{}
	processor := NewProcessor(fakeCodex, sender, "Codex 暂时不可用，请稍后重试。", "正在思考中...")

	processor.ProcessJob(context.Background(), Job{
		ReceiveID:     "oc_chat",
		ReceiveIDType: "chat_id",
		Text:          "发送图片",
	})

	if sender.sendImageCalls != 1 {
		t.Fatalf("expected 1 send image call, got %d", sender.sendImageCalls)
	}
	if len(sender.sendImages) != 1 || sender.sendImages[0] != "img_123" {
		t.Fatalf("unexpected sent image keys: %#v", sender.sendImages)
	}
	if sender.sendCardCalls != 0 {
		t.Fatalf("expected no card reply when action has no text, got %d", sender.sendCardCalls)
	}
}

func TestProcessor_MediaActionFailureFallsBackToText(t *testing.T) {
	fakeCodex := codexStub{resp: "```alice_action\n{\"actions\":[{\"type\":\"send_image\",\"path\":\"/tmp/a.png\"}]}\n```"}
	sender := &senderStub{uploadImageErr: errors.New("upload failed")}
	processor := NewProcessor(fakeCodex, sender, "Codex 暂时不可用，请稍后重试。", "正在思考中...")

	processor.ProcessJob(context.Background(), Job{
		ReceiveID:     "oc_chat",
		ReceiveIDType: "chat_id",
		Text:          "发送图片",
	})

	if sender.sendImageCalls != 0 {
		t.Fatalf("expected no image sent on upload failure, got %d", sender.sendImageCalls)
	}
	if sender.sendCardCalls != 1 {
		t.Fatalf("expected fallback card send, got %d", sender.sendCardCalls)
	}
	if len(sender.sendCards) != 1 || !strings.Contains(sender.sendCards[0], "多媒体发送失败") {
		t.Fatalf("expected fallback error text in card, got %#v", sender.sendCards)
	}
}

func TestProcessor_ReplyFlowExecutesMediaAction(t *testing.T) {
	fakeCodex := codexStub{resp: "```alice_action\n{\"actions\":[{\"type\":\"send_file\",\"file_key\":\"file_123\"}]}\n```"}
	sender := &senderStub{}
	processor := NewProcessor(fakeCodex, sender, "Codex 暂时不可用，请稍后重试。", "正在思考中...")

	processor.ProcessJob(context.Background(), Job{
		ReceiveID:       "oc_chat",
		ReceiveIDType:   "chat_id",
		SourceMessageID: "om_src",
		Text:            "发文件",
	})

	if sender.replyCardCalls != 1 {
		t.Fatalf("expected only ack reply card, got %d", sender.replyCardCalls)
	}
	if sender.sendFileCalls != 1 {
		t.Fatalf("expected 1 send file call, got %d", sender.sendFileCalls)
	}
	if len(sender.sendFiles) != 1 || sender.sendFiles[0] != "file_123" {
		t.Fatalf("unexpected sent file keys: %#v", sender.sendFiles)
	}
}
