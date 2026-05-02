package bootstrap

import (
	"testing"

	agentbridge "github.com/Alice-space/agentbridge"
)

func TestUpdateAssistantTextAccumulatesCodexDeltas(t *testing.T) {
	text := ""
	for _, event := range []agentbridge.TurnEvent{
		{Text: "你", Raw: `{"method":"item/agentMessage/delta"}`},
		{Text: "好", Raw: `{"method":"item/agentMessage/delta"}`},
	} {
		text = updateAssistantText(text, event)
	}
	if text != "你好" {
		t.Fatalf("expected accumulated deltas, got %q", text)
	}

	text = updateAssistantText(text, agentbridge.TurnEvent{
		Text: "你好，已完成。",
		Raw:  `{"method":"item/completed"}`,
	})
	if text != "你好，已完成。" {
		t.Fatalf("expected completed assistant message to replace accumulator, got %q", text)
	}
}
