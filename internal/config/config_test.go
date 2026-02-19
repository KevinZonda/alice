package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadFromFile_WithDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "feishu_app_id: cli_xxx\nfeishu_app_secret: sss\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}

	if cfg.FeishuBaseURL != "https://open.feishu.cn" {
		t.Fatalf("unexpected feishu_base_url: %s", cfg.FeishuBaseURL)
	}
	if cfg.CodexCommand != "codex" {
		t.Fatalf("unexpected codex_command: %s", cfg.CodexCommand)
	}
	if cfg.CodexTimeout != 120*time.Second {
		t.Fatalf("unexpected codex_timeout: %s", cfg.CodexTimeout)
	}
	if cfg.QueueCapacity != 256 {
		t.Fatalf("unexpected queue_capacity: %d", cfg.QueueCapacity)
	}
	if cfg.ThinkingMessage != "正在思考中..." {
		t.Fatalf("unexpected thinking_message: %s", cfg.ThinkingMessage)
	}
}

func TestLoadFromFile_RequiredKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "feishu_app_id: cli_xxx\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	_, err := LoadFromFile(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "feishu_app_secret is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
