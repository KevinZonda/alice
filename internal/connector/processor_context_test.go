package connector

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessorBuildPrompt_AppendsBotSoulForNewThread(t *testing.T) {
	soulPath := filepath.Join(t.TempDir(), "SOUL.md")
	soulText := "---\noutput_contract:\n  hidden_tags:\n    - reply_will\n    - motion\n  reply_will_tag: reply_will\n  reply_will_field: reply_will\n  motion_tag: motion\n  suppress_token: \"[[NO_REPLY]]\"\n---\n# Alice Chat\n- 回答前先澄清目标\n"
	if err := os.WriteFile(soulPath, []byte(soulText), 0o644); err != nil {
		t.Fatalf("write SOUL.md failed: %v", err)
	}

	processor := NewProcessor(codexStub{}, &senderStub{}, "", "")
	prompt := processor.buildPrompt(context.Background(), Job{
		EventID:  "evt_1",
		Text:     "帮我总结一下",
		BotName:  "Alice Chat",
		SoulPath: soulPath,
	}, "")

	if !strings.Contains(prompt, "Alice Chat") {
		t.Fatalf("expected prompt to include bot name, got %q", prompt)
	}
	if !strings.Contains(prompt, "回答前先澄清目标") {
		t.Fatalf("expected prompt to include soul content, got %q", prompt)
	}
	if strings.Contains(prompt, "output_contract:") {
		t.Fatalf("expected frontmatter to be stripped from prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "帮我总结一下") {
		t.Fatalf("expected prompt to include user text, got %q", prompt)
	}
}

func TestProcessorBuildPrompt_SkipsBotSoulForWorkScene(t *testing.T) {
	soulPath := filepath.Join(t.TempDir(), "SOUL.md")
	if err := os.WriteFile(soulPath, []byte("这段设定不应该出现在 work prompt"), 0o644); err != nil {
		t.Fatalf("write SOUL.md failed: %v", err)
	}

	processor := NewProcessor(codexStub{}, &senderStub{}, "", "")
	prompt := processor.buildPrompt(context.Background(), Job{
		EventID:  "evt_2",
		Text:     "执行任务",
		Scene:    jobSceneWork,
		BotName:  "Alice Work",
		SoulPath: soulPath,
	}, "")

	if strings.Contains(prompt, "SOUL.md") || strings.Contains(prompt, "这段设定不应该出现在 work prompt") {
		t.Fatalf("work prompt should not include soul content, got %q", prompt)
	}
	if !strings.Contains(prompt, "执行任务") {
		t.Fatalf("expected prompt to include user text, got %q", prompt)
	}
}
