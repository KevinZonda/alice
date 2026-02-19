package codex

import "testing"

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
