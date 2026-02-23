package mcpserver

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

type senderStub struct {
	sendTextCalls  int
	lastSendText   string
	sendImageCalls int
	lastImageKey   string
	sendFileCalls  int
	lastFileKey    string

	uploadImageKey string
	uploadFileKey  string
	uploadImageErr error
	uploadFileErr  error
}

func (s *senderStub) SendText(_ context.Context, _, _ string, text string) error {
	s.sendTextCalls++
	s.lastSendText = strings.TrimSpace(text)
	return nil
}

func (s *senderStub) SendImage(_ context.Context, _, _ string, imageKey string) error {
	s.sendImageCalls++
	s.lastImageKey = strings.TrimSpace(imageKey)
	return nil
}

func (s *senderStub) SendFile(_ context.Context, _, _ string, fileKey string) error {
	s.sendFileCalls++
	s.lastFileKey = strings.TrimSpace(fileKey)
	return nil
}

func (s *senderStub) UploadImage(_ context.Context, _ string) (string, error) {
	if s.uploadImageErr != nil {
		return "", s.uploadImageErr
	}
	if strings.TrimSpace(s.uploadImageKey) != "" {
		return s.uploadImageKey, nil
	}
	return "img_uploaded", nil
}

func (s *senderStub) UploadFile(_ context.Context, _, _ string) (string, error) {
	if s.uploadFileErr != nil {
		return "", s.uploadFileErr
	}
	if strings.TrimSpace(s.uploadFileKey) != "" {
		return s.uploadFileKey, nil
	}
	return "file_uploaded", nil
}

func TestHandleSendImage_UsesImageKeyAndCaption(t *testing.T) {
	sender := &senderStub{}
	svc := &service{
		sender: sender,
		getenv: func(key string) string {
			switch key {
			case "ALICE_MCP_RECEIVE_ID_TYPE":
				return "chat_id"
			case "ALICE_MCP_RECEIVE_ID":
				return "oc_chat"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleSendImage(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"image_key": "img_123",
			"caption":   "done",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected non-error result, got %#v", result)
	}
	if sender.sendImageCalls != 1 || sender.lastImageKey != "img_123" {
		t.Fatalf("unexpected send image state: %+v", sender)
	}
	if sender.sendTextCalls != 1 || sender.lastSendText != "done" {
		t.Fatalf("unexpected caption send state: %+v", sender)
	}
}

func TestHandleSendImage_MissingSessionContext(t *testing.T) {
	sender := &senderStub{}
	svc := &service{sender: sender, getenv: func(string) string { return "" }}

	result, err := svc.handleSendImage(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{"image_key": "img_123"}},
	})
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected tool error result, got %#v", result)
	}
	if sender.sendImageCalls != 0 {
		t.Fatalf("should not send image on invalid context, got %d", sender.sendImageCalls)
	}
}

func TestHandleSendFile_UploadFromPath(t *testing.T) {
	sender := &senderStub{uploadFileKey: "file_abc"}
	svc := &service{
		sender: sender,
		getenv: func(key string) string {
			switch key {
			case "ALICE_MCP_RECEIVE_ID_TYPE":
				return "chat_id"
			case "ALICE_MCP_RECEIVE_ID":
				return "oc_chat"
			case "ALICE_MCP_RESOURCE_ROOT":
				return "/tmp/root"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleSendFile(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"path": "/tmp/root/a.txt",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected non-error result, got %#v", result)
	}
	if sender.sendFileCalls != 1 || sender.lastFileKey != "file_abc" {
		t.Fatalf("unexpected send file state: %+v", sender)
	}
}

func TestHandleSendFile_RejectPathOutsideRoot(t *testing.T) {
	sender := &senderStub{}
	svc := &service{
		sender: sender,
		getenv: func(key string) string {
			switch key {
			case "ALICE_MCP_RECEIVE_ID_TYPE":
				return "chat_id"
			case "ALICE_MCP_RECEIVE_ID":
				return "oc_chat"
			case "ALICE_MCP_RESOURCE_ROOT":
				return "/tmp/root"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleSendFile(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"path": "/tmp/outside.txt",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected tool error result, got %#v", result)
	}
	if sender.sendFileCalls != 0 {
		t.Fatalf("should not send file on invalid path, got %d", sender.sendFileCalls)
	}
}

func TestHandleSendImage_UploadFailureReturnedAsToolError(t *testing.T) {
	sender := &senderStub{uploadImageErr: errors.New("boom")}
	svc := &service{
		sender: sender,
		getenv: func(key string) string {
			switch key {
			case "ALICE_MCP_RECEIVE_ID_TYPE":
				return "chat_id"
			case "ALICE_MCP_RECEIVE_ID":
				return "oc_chat"
			case "ALICE_MCP_RESOURCE_ROOT":
				return "/tmp/root"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleSendImage(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{"path": "/tmp/root/a.png"}},
	})
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected upload failure as tool error result, got %#v", result)
	}
}

func TestValidatePathUnderRoot(t *testing.T) {
	if err := validatePathUnderRoot("/tmp/root/a.txt", "/tmp/root"); err != nil {
		t.Fatalf("expected allowed path, got %v", err)
	}
	if err := validatePathUnderRoot("/tmp/other/a.txt", "/tmp/root"); err == nil {
		t.Fatal("expected root validation error")
	}
}
