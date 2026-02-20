package codex

import (
	"slices"
	"testing"
)

func TestParseFinalMessage(t *testing.T) {
	output := `not-json
{"type":"item.started"}
{"type":"item.completed","item":{"type":"agent_message","text":"first"}}
{"type":"item.completed","item":{"type":"agent_message","text":"final answer"}}`

	msg, err := ParseFinalMessage(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg != "final answer" {
		t.Fatalf("unexpected message: %s", msg)
	}
}

func TestParseFinalMessage_NoAgentMessage(t *testing.T) {
	_, err := ParseFinalMessage(`{"type":"item.completed","item":{"type":"tool_call"}}`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMergeEnv_OverridesAndAppends(t *testing.T) {
	base := []string{"PATH=/usr/bin", "HTTPS_PROXY=http://old:7890"}
	overrides := map[string]string{
		"HTTPS_PROXY": "http://127.0.0.1:7890",
		"ALL_PROXY":   "socks5://127.0.0.1:7891",
	}

	merged := mergeEnv(base, overrides)

	if !slices.Contains(merged, "HTTPS_PROXY=http://127.0.0.1:7890") {
		t.Fatalf("expected HTTPS_PROXY override, got %#v", merged)
	}
	if !slices.Contains(merged, "ALL_PROXY=socks5://127.0.0.1:7891") {
		t.Fatalf("expected ALL_PROXY append, got %#v", merged)
	}
}
