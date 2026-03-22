package imagegen

import (
	"context"
	"fmt"
	"strings"

	"github.com/Alice-space/alice/internal/config"
)

type Request struct {
	Prompt          string
	OutputPath      string
	ReferenceImages []string
	UserID          string
}

type Result struct {
	LocalPath     string
	RevisedPrompt string
}

type Provider interface {
	Generate(ctx context.Context, req Request) (Result, error)
}

func NewProvider(cfg config.ImageGenerationConfig, env map[string]string) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "", "openai":
		return newOpenAIProvider(cfg, env)
	default:
		return nil, fmt.Errorf("unsupported image generation provider %q", cfg.Provider)
	}
}
