package mcpbridge

import "testing"

func TestSessionContext_ToEnv(t *testing.T) {
	ctx := SessionContext{
		ReceiveIDType:   "chat_id",
		ReceiveID:       "oc_chat",
		ResourceRoot:    "/tmp/root",
		SourceMessageID: "om_source",
	}
	env := ctx.ToEnv()
	if env[EnvReceiveIDType] != "chat_id" {
		t.Fatalf("unexpected receive id type env: %#v", env)
	}
	if env[EnvReceiveID] != "oc_chat" {
		t.Fatalf("unexpected receive id env: %#v", env)
	}
	if env[EnvResourceRoot] != "/tmp/root" {
		t.Fatalf("unexpected resource root env: %#v", env)
	}
	if env[EnvSourceMessageID] != "om_source" {
		t.Fatalf("unexpected source message env: %#v", env)
	}
}

func TestSessionContextFromEnv(t *testing.T) {
	ctx := SessionContextFromEnv(func(key string) string {
		switch key {
		case EnvReceiveIDType:
			return "chat_id"
		case EnvReceiveID:
			return "oc_chat"
		case EnvResourceRoot:
			return "/tmp/root"
		case EnvSourceMessageID:
			return "om_source"
		default:
			return ""
		}
	})
	if ctx.ReceiveIDType != "chat_id" ||
		ctx.ReceiveID != "oc_chat" ||
		ctx.ResourceRoot != "/tmp/root" ||
		ctx.SourceMessageID != "om_source" {
		t.Fatalf("unexpected context: %+v", ctx)
	}
}

func TestSessionContextValidate(t *testing.T) {
	if err := (SessionContext{ReceiveIDType: "chat_id", ReceiveID: "oc_chat"}).Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if err := (SessionContext{ReceiveIDType: "", ReceiveID: "oc_chat"}).Validate(); err == nil {
		t.Fatal("expected missing receive id type validation error")
	}
	if err := (SessionContext{ReceiveIDType: "chat_id", ReceiveID: ""}).Validate(); err == nil {
		t.Fatal("expected missing receive id validation error")
	}
}
