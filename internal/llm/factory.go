package llm

import (
	"fmt"
	"strings"
	"time"
)

const ProviderCodex = "codex"

type FactoryConfig struct {
	Provider string
	Codex    CodexConfig
}

type CodexConfig struct {
	Command      string
	Timeout      time.Duration
	Env          map[string]string
	PromptPrefix string
	WorkspaceDir string
}

func NewBackend(cfg FactoryConfig) (Backend, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = ProviderCodex
	}

	switch provider {
	case ProviderCodex:
		return newCodexBackend(cfg.Codex), nil
	default:
		return nil, fmt.Errorf("unsupported llm_provider %q", provider)
	}
}
