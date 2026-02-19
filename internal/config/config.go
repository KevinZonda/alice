package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	FeishuAppID     string
	FeishuAppSecret string
	FeishuBaseURL   string

	CodexCommand      string
	CodexTimeout      time.Duration
	CodexPromptPrefix string
	FailureMessage    string
	WorkspaceDir      string

	QueueCapacity     int
	WorkerConcurrency int

	LogLevel string
}

func LoadFromEnv() (Config, error) {
	cfg := Config{
		FeishuAppID:       strings.TrimSpace(os.Getenv("FEISHU_APP_ID")),
		FeishuAppSecret:   strings.TrimSpace(os.Getenv("FEISHU_APP_SECRET")),
		FeishuBaseURL:     envOrDefault("FEISHU_BASE_URL", "https://open.feishu.cn"),
		CodexCommand:      envOrDefault("CODEX_COMMAND", "codex"),
		CodexPromptPrefix: envOrDefault("CODEX_PROMPT_PREFIX", "你是一个助手，请用中文简洁回答，不要使用 Markdown 标题。"),
		FailureMessage:    envOrDefault("FAILURE_MESSAGE", "Codex 暂时不可用，请稍后重试。"),
		WorkspaceDir:      envOrDefault("WORKSPACE_DIR", "."),
		LogLevel:          strings.ToLower(envOrDefault("LOG_LEVEL", "info")),
	}

	if cfg.FeishuAppID == "" {
		return Config{}, errors.New("FEISHU_APP_ID is required")
	}
	if cfg.FeishuAppSecret == "" {
		return Config{}, errors.New("FEISHU_APP_SECRET is required")
	}

	timeoutSecs, err := intFromEnv("CODEX_TIMEOUT_SECS", 120)
	if err != nil {
		return Config{}, err
	}
	cfg.CodexTimeout = time.Duration(timeoutSecs) * time.Second

	cfg.QueueCapacity, err = intFromEnv("QUEUE_CAPACITY", 256)
	if err != nil {
		return Config{}, err
	}
	if cfg.QueueCapacity <= 0 {
		return Config{}, errors.New("QUEUE_CAPACITY must be > 0")
	}

	cfg.WorkerConcurrency, err = intFromEnv("WORKER_CONCURRENCY", 1)
	if err != nil {
		return Config{}, err
	}
	if cfg.WorkerConcurrency <= 0 {
		return Config{}, errors.New("WORKER_CONCURRENCY must be > 0")
	}

	return cfg, nil
}

func envOrDefault(key, defaultValue string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	return value
}

func intFromEnv(key string, defaultValue int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return value, nil
}
