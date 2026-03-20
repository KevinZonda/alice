package connector

import (
	"context"
	"strings"

	"github.com/Alice-space/alice/internal/logging"
)

const helpCommandName = "/help"

func (p *Processor) processBuiltinCommand(ctx context.Context, job Job) (bool, JobProcessState) {
	if isHelpCommand(job.Text) {
		return true, p.processHelpCommand(ctx, job)
	}
	return false, JobProcessCompleted
}

func isBuiltinCommandText(text string) bool {
	return isHelpCommand(text)
}

func isHelpCommand(text string) bool {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(fields[0]), helpCommandName)
}

func (p *Processor) processHelpCommand(ctx context.Context, job Job) JobProcessState {
	reply := buildBuiltinHelpMarkdown()
	if err := p.replies.respond(ctx, job, reply); err != nil {
		logging.Errorf("send builtin help reply failed event_id=%s: %v", job.EventID, err)
	}
	return JobProcessCompleted
}

func buildBuiltinHelpMarkdown() string {
	return strings.Join([]string{
		"## Alice 内建命令",
		"",
		"- `/help`",
		"  显示当前可用的所有内建命令。",
	}, "\n")
}
