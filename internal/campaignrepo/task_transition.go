package campaignrepo

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	dispatchStateExecutorDispatched         = "executor_dispatched"
	dispatchStateSignalBlockedTerminal      = "signal_blocked_terminal"
	dispatchStateBlockedGuidanceRequested   = "blocked_guidance_requested"
	dispatchStateBlockedGuidanceApplied     = "blocked_guidance_applied"
	dispatchStateJudgeApplied               = "judge_applied"
	dispatchStateExecutorCompleted          = "executor_completed"
	dispatchStateReviewerCompletedRecovered = "reviewer_completed_recovered"
	dispatchStateWorkspaceSetupFailed       = "workspace_setup_failed"
	dispatchStateFastTrackAccepted          = "fast_track_accepted"
)

// clearTaskAssignment removes active executor/reviewer ownership from a task.
func clearTaskAssignment(task *TaskDocument) {
	if task == nil {
		return
	}
	task.Frontmatter.OwnerAgent = ""
	task.LeaseUntil = time.Time{}
	task.WakeAt = time.Time{}
	task.Frontmatter.WakePrompt = ""
}

func applyTaskTerminalBlockedTransition(task *TaskDocument, reason string) {
	applyTaskBlockedTransition(task, dispatchStateSignalBlockedTerminal, reason)
}

func applyTaskBlockedTransition(task *TaskDocument, dispatchState, reason string) {
	if task == nil {
		return
	}
	recordBlockedReason(task, reason)
	clearTaskAssignment(task)
	task.Frontmatter.Status = TaskStatusBlocked
	task.Frontmatter.DispatchState = strings.TrimSpace(dispatchState)
	task.Frontmatter.ReviewStatus = "blocked"
}

func applyTaskReviewPendingTransition(task *TaskDocument, dispatchState string, clearBlockedReason bool) {
	if task == nil {
		return
	}
	clearTaskAssignment(task)
	task.Frontmatter.Status = TaskStatusReviewPending
	task.Frontmatter.DispatchState = strings.TrimSpace(dispatchState)
	task.Frontmatter.ReviewStatus = "pending"
	if clearBlockedReason {
		clearBlockedReasonMetadata(task)
	}
}

func applyBlockedGuidanceTransition(task *TaskDocument, reason string) BlockedTaskOutcome {
	if task == nil {
		return BlockedTaskOutcome{}
	}
	recordBlockedReason(task, reason)
	clearTaskAssignment(task)
	if task.Frontmatter.BlockGuidanceCount < maxBlockedGuidanceRetries {
		task.Frontmatter.BlockGuidanceCount++
		applyTaskReviewPendingTransition(task, dispatchStateBlockedGuidanceRequested, false)
		return BlockedTaskOutcome{
			GuidanceRequested: true,
			GuidanceAttempt:   task.Frontmatter.BlockGuidanceCount,
			Reason:            task.Frontmatter.LastBlockedReason,
		}
	}
	applyTaskTerminalBlockedTransition(task, task.Frontmatter.LastBlockedReason)
	return BlockedTaskOutcome{
		TerminalBlocked: true,
		GuidanceAttempt: task.Frontmatter.BlockGuidanceCount,
		Reason:          task.Frontmatter.LastBlockedReason,
	}
}

func applyTaskNeedsHumanTransition(task *TaskDocument, reason string) {
	if task == nil {
		return
	}
	clearTaskAssignment(task)
	task.Frontmatter.Status = TaskStatusBlocked
	task.Frontmatter.DispatchState = dispatchStateNeedsHuman
	task.Frontmatter.ReviewStatus = "blocked"
	applyBlockedReasonMetadata(task, reason)
}

func applyTaskHumanGuidanceTransition(task *TaskDocument, action, guidancePath, guidance string, round int) error {
	if task == nil {
		return nil
	}
	action = normalizeTaskHumanGuidanceAction(action)
	switch action {
	case TaskHumanGuidanceActionAccept, TaskHumanGuidanceActionResume:
	default:
		return fmt.Errorf("unsupported task guidance action %q", action)
	}

	clearTaskAssignment(task)
	task.Frontmatter.HumanGuidanceRound = round
	task.Frontmatter.HumanGuidanceStatus = action
	task.Frontmatter.LastHumanGuidancePath = strings.TrimSpace(guidancePath)
	task.Frontmatter.LastHumanGuidanceSummary = strings.TrimSpace(guidance)
	clearBlockedReasonMetadata(task)

	switch action {
	case TaskHumanGuidanceActionAccept:
		task.Frontmatter.Status = TaskStatusAccepted
		task.Frontmatter.ReviewStatus = "approved"
		task.Frontmatter.DispatchState = dispatchStateHumanGuidanceApplied
	case TaskHumanGuidanceActionResume:
		task.Frontmatter.Status = TaskStatusRework
		task.Frontmatter.ReviewStatus = "changes_requested"
		task.Frontmatter.DispatchState = dispatchStateHumanGuidanceRequested
	}
	return nil
}

func applyTaskJudgeDeferredTransition(task *TaskDocument, reason string) {
	if task == nil {
		return
	}
	applyBlockedReasonMetadata(task, reason)
	task.Frontmatter.DispatchState = dispatchStateJudgeWaitingReviewer
}

func applyTaskReviewVerdictTransition(task *TaskDocument, review ReviewDocument, verdict string, blockedGuidanceLoop bool) {
	if task == nil {
		return
	}
	switch verdict {
	case "approve":
		task.Frontmatter.Status = TaskStatusAccepted
		task.Frontmatter.ReviewStatus = "approved"
	case "blocking":
		if blockedGuidanceLoop {
			task.Frontmatter.Status = TaskStatusRework
			task.Frontmatter.ReviewStatus = "changes_requested"
		} else {
			task.Frontmatter.Status = TaskStatusBlocked
			task.Frontmatter.ReviewStatus = "blocked"
		}
	case "reject":
		task.Frontmatter.Status = TaskStatusRejected
		task.Frontmatter.ReviewStatus = "blocked"
	default:
		task.Frontmatter.Status = TaskStatusRework
		task.Frontmatter.ReviewStatus = "changes_requested"
	}

	task.Frontmatter.DispatchState = dispatchStateJudgeApplied
	if verdict == "blocking" && blockedGuidanceLoop {
		task.Frontmatter.DispatchState = dispatchStateBlockedGuidanceApplied
	}
	clearBlockedReasonMetadata(task)
	task.Frontmatter.LastReviewPath = filepath.ToSlash(strings.TrimSpace(review.Path))
	clearTaskAssignment(task)
	if taskRequiresSourceRepoEvidence(*task) {
		if commit := strings.TrimSpace(review.Frontmatter.TargetCommit); commit != "" {
			task.Frontmatter.HeadCommit = commit
		}
	} else {
		task.Frontmatter.HeadCommit = ""
	}
}

func applyTaskFastTrackAcceptedTransition(task *TaskDocument) {
	if task == nil {
		return
	}
	clearTaskAssignment(task)
	task.Frontmatter.Status = TaskStatusAccepted
	task.Frontmatter.DispatchState = dispatchStateFastTrackAccepted
	task.Frontmatter.ReviewStatus = "approved"
	clearBlockedReasonMetadata(task)
}
