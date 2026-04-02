package campaignrepo

import (
	"fmt"
	"strings"
)

func ValidateDispatchCompletion(root, campaignID string, kind DispatchKind, taskID string, planRound int) (ReconcileEvent, bool, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return ReconcileEvent{}, false, nil
	}
	switch kind {
	case DispatchKindPlanner, DispatchKindPlannerReviewer:
		validation, err := ValidatePlanPostRun(root, kind, planRound)
		if err != nil || validation.Valid {
			return ReconcileEvent{}, false, err
		}
		reason := summarizeValidationIssues(validation.Issues, 3)
		return ReconcileEvent{
			Kind:       EventPlanningBlocked,
			CampaignID: campaignID,
			PlanRound:  planRound,
			Title:      "规划收尾校验失败",
			Detail:     fmt.Sprintf("规划角色 `%s` 在第 %d 轮结束后未通过收尾校验，已阻止继续推进。\n\n**问题**:\n%s", kind, planRound, reason),
			Severity:   "error",
		}, true, nil
	case DispatchKindExecutor:
		validation, err := ValidateTaskPostRun(root, taskID, kind)
		if err != nil || validation.Valid {
			return ReconcileEvent{}, false, err
		}
		reason := summarizeValidationIssues(validation.Issues, 3)
		outcome, err := HandleTaskBlocked(root, taskID, "post-run validation failed after executor round: "+reason)
		if err != nil {
			return ReconcileEvent{}, false, err
		}
		title := "执行收尾校验失败，等待恢复"
		detail := fmt.Sprintf("任务 **%s** executor 回合结束后未通过状态校验，当前处于恢复等待态。\n\n这类问题通常可通过继续运行、修正合法交接状态并重新通过 self-check 解决，不需要人工加急介入。\n\n**问题**:\n%s", taskID, reason)
		if outcome.GuidanceRequested {
			title = "执行收尾校验失败，转评审指导"
			detail = fmt.Sprintf("任务 **%s** executor 回合结束后未通过状态校验，已转 reviewer 指导（第 %d/%d 次）。\n\n**问题**:\n%s", taskID, outcome.GuidanceAttempt, maxBlockedGuidanceRetries, reason)
		} else if outcome.TerminalBlocked {
			title = "执行收尾校验失败，等待恢复"
			detail = fmt.Sprintf("任务 **%s** executor 回合结束后未通过状态校验，且指导预算已耗尽，当前已被冻结等待恢复。\n\n这类问题通常仍应通过继续运行、补齐合法产物或重新通过 self-check 来解决，不需要人工加急介入。\n\n**问题**:\n%s", taskID, reason)
		}
		kind := EventTaskBlocked
		if outcome.GuidanceRequested {
			kind = EventTaskRetrying
		}
		return ReconcileEvent{
			Kind:       kind,
			CampaignID: campaignID,
			TaskID:     taskID,
			Title:      title,
			Detail:     detail,
			Severity:   "warning",
		}, true, nil
	case DispatchKindReviewer:
		validation, err := ValidateTaskPostRun(root, taskID, kind)
		if err != nil || validation.Valid {
			return ReconcileEvent{}, false, err
		}
		reason := summarizeValidationIssues(validation.Issues, 3)
		if err := MarkTaskBlocked(root, taskID, "post-run validation failed after reviewer round: "+reason); err != nil {
			return ReconcileEvent{}, false, err
		}
		return ReconcileEvent{
			Kind:       EventTaskBlocked,
			CampaignID: campaignID,
			TaskID:     taskID,
			Title:      "评审收尾校验失败，等待恢复",
			Detail:     fmt.Sprintf("任务 **%s** reviewer 回合结束后未通过状态校验，当前已冻结该非法交接，等待继续运行或补齐合法评审产物后恢复。\n\n这类问题通常不需要人工加急介入。\n\n**问题**:\n%s", taskID, reason),
			Severity:   "warning",
		}, true, nil
	default:
		return ReconcileEvent{}, false, nil
	}
}

func summarizeValidationIssues(issues []ValidationIssue, limit int) string {
	if len(issues) == 0 {
		return "- unknown validation failure"
	}
	if limit <= 0 || limit > len(issues) {
		limit = len(issues)
	}
	lines := make([]string, 0, limit+1)
	for _, issue := range issues[:limit] {
		lines = append(lines, "- "+strings.TrimSpace(issue.Message))
	}
	if extra := len(issues) - limit; extra > 0 {
		lines = append(lines, fmt.Sprintf("- 另外还有 %d 条校验问题", extra))
	}
	return strings.Join(lines, "\n")
}
