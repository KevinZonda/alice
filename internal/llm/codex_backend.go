package llm

import (
	"context"
	"strings"

	corecodex "github.com/Alice-space/alice/internal/llm/codex"
	"github.com/Alice-space/alice/internal/prompting"
)

type codexBackend struct {
	runner corecodex.Runner
}

func newCodexBackend(cfg CodexConfig, prompts *prompting.Loader) *codexBackend {
	return &codexBackend{
		runner: corecodex.Runner{
			Command:                cfg.Command,
			Timeout:                cfg.Timeout,
			DefaultModel:           cfg.Model,
			DefaultReasoningEffort: cfg.ReasoningEffort,
			Env:                    cfg.Env,
			PromptPrefix:           cfg.PromptPrefix,
			WorkspaceDir:           cfg.WorkspaceDir,
			Prompts:                prompts,
		},
	}
}

func (b *codexBackend) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	policy := corecodex.ExecPolicyConfig{
		Sandbox:        strings.TrimSpace(req.ExecPolicy.Sandbox),
		AskForApproval: strings.TrimSpace(req.ExecPolicy.AskForApproval),
		AddDirs:        append([]string(nil), req.ExecPolicy.AddDirs...),
	}
	reply, nextThreadID, usage, err := b.runner.RunWithThreadAndProgressAndUsage(
		ctx,
		strings.TrimSpace(req.ThreadID),
		strings.TrimSpace(req.AgentName),
		req.UserText,
		policy,
		strings.TrimSpace(req.PromptPrefix),
		strings.TrimSpace(req.Model),
		strings.TrimSpace(req.Profile),
		strings.TrimSpace(req.ReasoningEffort),
		strings.TrimSpace(req.Personality),
		strings.TrimSpace(req.NoReplyToken),
		req.Env,
		req.OnProgress,
	)
	return RunResult{
		Reply:        reply,
		NextThreadID: strings.TrimSpace(nextThreadID),
		Usage: Usage{
			InputTokens:       usage.InputTokens,
			CachedInputTokens: usage.CachedInputTokens,
			OutputTokens:      usage.OutputTokens,
		},
	}, err
}

var _ Backend = (*codexBackend)(nil)
