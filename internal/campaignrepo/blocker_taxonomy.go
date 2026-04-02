package campaignrepo

import "strings"

const (
	blockedClassRetryable = "retryable"
	blockedClassWaiting   = "waiting"
	blockedClassHuman     = "needs_human"
	blockedClassTerminal  = "terminal"
)

type blockedReasonMeta struct {
	Code         string
	Class        string
	RecoveryHint string
}

func classifyBlockedReason(reason string) blockedReasonMeta {
	normalized := strings.ToLower(strings.TrimSpace(reason))
	switch {
	case normalized == "":
		return blockedReasonMeta{}
	case strings.Contains(normalized, "post-run validation failed"),
		strings.Contains(normalized, "self-check proof"),
		strings.Contains(normalized, "self_check"),
		strings.Contains(normalized, "receipt"):
		return blockedReasonMeta{
			Code:         "post_run_validation",
			Class:        blockedClassRetryable,
			RecoveryHint: "继续补齐合法交接产物并重新通过 self-check",
		}
	case strings.Contains(normalized, "dependency `"),
		strings.Contains(normalized, "not done yet"):
		return blockedReasonMeta{
			Code:         "dependency_wait",
			Class:        blockedClassWaiting,
			RecoveryHint: "等待依赖任务推进后重试",
		}
	case strings.Contains(normalized, "leased to `"):
		return blockedReasonMeta{
			Code:         "lease_held",
			Class:        blockedClassWaiting,
			RecoveryHint: "等待当前 lease 结束或显式恢复",
		}
	case strings.Contains(normalized, "write scope overlaps"):
		return blockedReasonMeta{
			Code:         "write_scope_conflict",
			Class:        blockedClassWaiting,
			RecoveryHint: "等待冲突写入范围释放后再派发",
		}
	case strings.Contains(normalized, "merge conflict"),
		strings.Contains(normalized, "conflict (content)"),
		strings.Contains(normalized, "automatic merge failed"):
		return blockedReasonMeta{
			Code:         "integration_conflict",
			Class:        blockedClassRetryable,
			RecoveryHint: "在 task worktree 吸收默认分支新变化后重试集成",
		}
	case strings.Contains(normalized, "working_branches"),
		strings.Contains(normalized, "worktree"),
		strings.Contains(normalized, "default branch"),
		strings.Contains(normalized, "uncommitted changes"),
		strings.Contains(normalized, "local_path"),
		strings.Contains(normalized, "base_commit"):
		return blockedReasonMeta{
			Code:         "workspace_invalid",
			Class:        blockedClassRetryable,
			RecoveryHint: "修复 worktree / branch / base commit 合同后重试",
		}
	case LooksLikeGitIdentityProblem(reason):
		return blockedReasonMeta{
			Code:         "git_identity_missing",
			Class:        blockedClassTerminal,
			RecoveryHint: "先修复 git identity，再恢复执行",
		}
	case strings.Contains(normalized, "needs human"),
		strings.Contains(normalized, "人工"),
		strings.Contains(normalized, "human guidance"):
		return blockedReasonMeta{
			Code:         "needs_human",
			Class:        blockedClassHuman,
			RecoveryHint: "等待人工指导或裁决",
		}
	case strings.Contains(normalized, "external"),
		strings.Contains(normalized, "handoff"),
		strings.Contains(normalized, "remote job"):
		return blockedReasonMeta{
			Code:         "external_wait",
			Class:        blockedClassWaiting,
			RecoveryHint: "等待外部环境或远端作业变化后继续",
		}
	default:
		return blockedReasonMeta{
			Code:         "runtime_blocked",
			Class:        blockedClassRetryable,
			RecoveryHint: "继续修复 task-local 状态并重新尝试",
		}
	}
}

func applyBlockedReasonMetadata(task *TaskDocument, reason string) {
	if task == nil {
		return
	}
	meta := classifyBlockedReason(reason)
	task.Frontmatter.LastBlockedReason = strings.TrimSpace(reason)
	task.Frontmatter.BlockedCode = meta.Code
	task.Frontmatter.BlockedClass = meta.Class
	task.Frontmatter.RecoveryHint = meta.RecoveryHint
}

func clearBlockedReasonMetadata(task *TaskDocument) {
	if task == nil {
		return
	}
	task.Frontmatter.LastBlockedReason = ""
	task.Frontmatter.BlockedCode = ""
	task.Frontmatter.BlockedClass = ""
	task.Frontmatter.RecoveryHint = ""
}

func blockedReasonSeverity(reason string) string {
	meta := classifyBlockedReason(reason)
	switch meta.Class {
	case blockedClassHuman, blockedClassTerminal:
		return "error"
	default:
		return "warning"
	}
}
