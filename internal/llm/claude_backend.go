package llm

import (
	"context"
	"strings"

	coreclaude "github.com/Alice-space/alice/internal/llm/claude"
	"github.com/Alice-space/alice/internal/prompting"
)

type claudeBackend struct {
	runner coreclaude.Runner
}

func newClaudeBackend(cfg ClaudeConfig, prompts *prompting.Loader) *claudeBackend {
	return &claudeBackend{
		runner: coreclaude.Runner{
			Command:      cfg.Command,
			Timeout:      cfg.Timeout,
			Env:          cfg.Env,
			PromptPrefix: cfg.PromptPrefix,
			WorkspaceDir: cfg.WorkspaceDir,
			Prompts:      prompts,
		},
	}
}

func (b *claudeBackend) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	reply, nextThreadID, err := b.runner.RunWithThreadAndProgress(
		ctx,
		strings.TrimSpace(req.ThreadID),
		strings.TrimSpace(req.AgentName),
		req.UserText,
		strings.TrimSpace(req.Model),
		strings.TrimSpace(req.Profile),
		strings.TrimSpace(req.Personality),
		strings.TrimSpace(req.NoReplyToken),
		req.Env,
		req.OnProgress,
	)
	return RunResult{
		Reply:        reply,
		NextThreadID: strings.TrimSpace(nextThreadID),
	}, err
}

var _ Backend = (*claudeBackend)(nil)
