package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const DefaultConfigPath = "config.yaml"

type Config struct {
	FeishuAppID     string `mapstructure:"feishu_app_id"`
	FeishuAppSecret string `mapstructure:"feishu_app_secret"`
	FeishuBaseURL   string `mapstructure:"feishu_base_url"`

	CodexCommand      string        `mapstructure:"codex_command"`
	CodexTimeout      time.Duration `mapstructure:"-"`
	CodexTimeoutSecs  int           `mapstructure:"codex_timeout_secs"`
	CodexPromptPrefix string        `mapstructure:"codex_prompt_prefix"`
	FailureMessage    string        `mapstructure:"failure_message"`
	ThinkingMessage   string        `mapstructure:"thinking_message"`
	WorkspaceDir      string        `mapstructure:"workspace_dir"`
	MemoryDir         string        `mapstructure:"memory_dir"`

	QueueCapacity     int `mapstructure:"queue_capacity"`
	WorkerConcurrency int `mapstructure:"worker_concurrency"`

	LogLevel string `mapstructure:"log_level"`
}

func LoadFromFile(path string) (Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetDefault("feishu_base_url", "https://open.feishu.cn")
	v.SetDefault("codex_command", "codex")
	v.SetDefault("codex_timeout_secs", 120)
	v.SetDefault("codex_prompt_prefix", "你是一个助手，请用中文简洁回答，不要使用 Markdown 标题。")
	v.SetDefault("failure_message", "Codex 暂时不可用，请稍后重试。")
	v.SetDefault("thinking_message", "正在思考中...")
	v.SetDefault("workspace_dir", ".")
	v.SetDefault("memory_dir", ".memory")
	v.SetDefault("queue_capacity", 256)
	v.SetDefault("worker_concurrency", 1)
	v.SetDefault("log_level", "info")

	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config file %q failed: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config yaml failed: %w", err)
	}

	cfg.FeishuAppID = strings.TrimSpace(cfg.FeishuAppID)
	cfg.FeishuAppSecret = strings.TrimSpace(cfg.FeishuAppSecret)
	cfg.FeishuBaseURL = strings.TrimSpace(cfg.FeishuBaseURL)
	cfg.CodexCommand = strings.TrimSpace(cfg.CodexCommand)
	cfg.CodexPromptPrefix = strings.TrimSpace(cfg.CodexPromptPrefix)
	cfg.FailureMessage = strings.TrimSpace(cfg.FailureMessage)
	cfg.ThinkingMessage = strings.TrimSpace(cfg.ThinkingMessage)
	cfg.WorkspaceDir = strings.TrimSpace(cfg.WorkspaceDir)
	cfg.MemoryDir = strings.TrimSpace(cfg.MemoryDir)
	cfg.LogLevel = strings.ToLower(strings.TrimSpace(cfg.LogLevel))

	if cfg.FeishuAppID == "" {
		return Config{}, errors.New("feishu_app_id is required")
	}
	if cfg.FeishuAppSecret == "" {
		return Config{}, errors.New("feishu_app_secret is required")
	}
	if cfg.FeishuBaseURL == "" {
		cfg.FeishuBaseURL = "https://open.feishu.cn"
	}
	if cfg.CodexCommand == "" {
		cfg.CodexCommand = "codex"
	}
	if cfg.WorkspaceDir == "" {
		cfg.WorkspaceDir = "."
	}
	if cfg.MemoryDir == "" {
		cfg.MemoryDir = ".memory"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.CodexPromptPrefix == "" {
		cfg.CodexPromptPrefix = "你是一个助手，请用中文简洁回答，不要使用 Markdown 标题。"
	}
	if cfg.FailureMessage == "" {
		cfg.FailureMessage = "Codex 暂时不可用，请稍后重试。"
	}
	if cfg.ThinkingMessage == "" {
		cfg.ThinkingMessage = "正在思考中..."
	}

	if cfg.CodexTimeoutSecs <= 0 {
		return Config{}, errors.New("codex_timeout_secs must be > 0")
	}
	if cfg.QueueCapacity <= 0 {
		return Config{}, errors.New("queue_capacity must be > 0")
	}
	if cfg.WorkerConcurrency <= 0 {
		return Config{}, errors.New("worker_concurrency must be > 0")
	}
	cfg.CodexTimeout = time.Duration(cfg.CodexTimeoutSecs) * time.Second

	return cfg, nil
}
