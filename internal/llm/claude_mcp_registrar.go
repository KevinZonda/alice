package llm

import (
	"context"

	coreclaude "github.com/Alice-space/alice/internal/llm/claude"
)

type claudeMCPRegistrar struct {
	command string
}

func newClaudeMCPRegistrar(cfg ClaudeConfig) *claudeMCPRegistrar {
	return &claudeMCPRegistrar{
		command: cfg.Command,
	}
}

func (r *claudeMCPRegistrar) EnsureMCPServerRegistered(ctx context.Context, req MCPRegistration) error {
	return coreclaude.EnsureMCPServerRegistered(ctx, coreclaude.MCPRegistration{
		ClaudeCommand: r.command,
		ServerName:    req.ServerName,
		ServerCommand: req.ServerCommand,
		ServerArgs:    req.ServerArgs,
	})
}

var _ MCPRegistrar = (*claudeMCPRegistrar)(nil)
