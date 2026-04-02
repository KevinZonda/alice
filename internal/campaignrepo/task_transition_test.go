package campaignrepo

import (
	"testing"
	"time"
)

func TestApplyTaskTerminalBlockedTransition(t *testing.T) {
	task := &TaskDocument{
		Body: "## Goal\n\nFinish the task.",
		Frontmatter: TaskFrontmatter{
			TaskID:        "T001",
			Status:        TaskStatusExecuting,
			DispatchState: dispatchStateExecutorDispatched,
			ReviewStatus:  "pending",
			OwnerAgent:    "executor.codex",
			WakePrompt:    "resume later",
		},
		LeaseUntil: time.Date(2026, 4, 2, 18, 0, 0, 0, time.FixedZone("CST", 8*3600)),
		WakeAt:     time.Date(2026, 4, 2, 19, 0, 0, 0, time.FixedZone("CST", 8*3600)),
	}

	applyTaskTerminalBlockedTransition(task, "missing remote artifact")

	if got := normalizeTaskStatus(task.Frontmatter.Status); got != TaskStatusBlocked {
		t.Fatalf("expected blocked status, got %s", got)
	}
	if task.Frontmatter.DispatchState != dispatchStateSignalBlockedTerminal {
		t.Fatalf("expected %q dispatch state, got %q", dispatchStateSignalBlockedTerminal, task.Frontmatter.DispatchState)
	}
	if task.Frontmatter.ReviewStatus != "blocked" {
		t.Fatalf("expected blocked review status, got %q", task.Frontmatter.ReviewStatus)
	}
	if task.Frontmatter.LastBlockedReason != "missing remote artifact" {
		t.Fatalf("expected blocked reason to be recorded, got %q", task.Frontmatter.LastBlockedReason)
	}
	if task.Frontmatter.OwnerAgent != "" || !task.LeaseUntil.IsZero() || !task.WakeAt.IsZero() || task.Frontmatter.WakePrompt != "" {
		t.Fatalf("expected assignment and wake fields cleared, got owner=%q lease=%v wake=%v prompt=%q", task.Frontmatter.OwnerAgent, task.LeaseUntil, task.WakeAt, task.Frontmatter.WakePrompt)
	}
	if !containsAll(task.Body, "## Blocked", "missing remote artifact") {
		t.Fatalf("expected blocked section appended to body, got %q", task.Body)
	}
}

func TestApplyTaskReviewPendingTransition(t *testing.T) {
	tests := []struct {
		name               string
		clearBlockedReason bool
		wantBlockedReason  string
	}{
		{
			name:               "keeps blocker for guidance handoff",
			clearBlockedReason: false,
			wantBlockedReason:  "cluster handoff missing",
		},
		{
			name:               "clears blocker for recovered handoff",
			clearBlockedReason: true,
			wantBlockedReason:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &TaskDocument{
				Frontmatter: TaskFrontmatter{
					TaskID:            "T002",
					Status:            TaskStatusBlocked,
					DispatchState:     dispatchStateSignalBlockedTerminal,
					ReviewStatus:      "blocked",
					LastBlockedReason: "cluster handoff missing",
					OwnerAgent:        "executor.codex",
					WakePrompt:        "resume later",
				},
				LeaseUntil: time.Date(2026, 4, 2, 18, 0, 0, 0, time.FixedZone("CST", 8*3600)),
				WakeAt:     time.Date(2026, 4, 2, 19, 0, 0, 0, time.FixedZone("CST", 8*3600)),
			}

			applyTaskReviewPendingTransition(task, dispatchStateExecutorCompleted, tt.clearBlockedReason)

			if got := normalizeTaskStatus(task.Frontmatter.Status); got != TaskStatusReviewPending {
				t.Fatalf("expected review_pending status, got %s", got)
			}
			if task.Frontmatter.DispatchState != dispatchStateExecutorCompleted {
				t.Fatalf("expected %q dispatch state, got %q", dispatchStateExecutorCompleted, task.Frontmatter.DispatchState)
			}
			if task.Frontmatter.ReviewStatus != "pending" {
				t.Fatalf("expected pending review status, got %q", task.Frontmatter.ReviewStatus)
			}
			if task.Frontmatter.LastBlockedReason != tt.wantBlockedReason {
				t.Fatalf("unexpected blocked reason: got %q want %q", task.Frontmatter.LastBlockedReason, tt.wantBlockedReason)
			}
			if task.Frontmatter.OwnerAgent != "" || !task.LeaseUntil.IsZero() || !task.WakeAt.IsZero() || task.Frontmatter.WakePrompt != "" {
				t.Fatalf("expected assignment and wake fields cleared, got owner=%q lease=%v wake=%v prompt=%q", task.Frontmatter.OwnerAgent, task.LeaseUntil, task.WakeAt, task.Frontmatter.WakePrompt)
			}
		})
	}
}

func TestApplyTaskHumanGuidanceTransition(t *testing.T) {
	tests := []struct {
		name             string
		action           string
		wantStatus       string
		wantReviewStatus string
		wantDispatch     string
	}{
		{
			name:             "accept",
			action:           TaskHumanGuidanceActionAccept,
			wantStatus:       TaskStatusAccepted,
			wantReviewStatus: "approved",
			wantDispatch:     dispatchStateHumanGuidanceApplied,
		},
		{
			name:             "resume",
			action:           TaskHumanGuidanceActionResume,
			wantStatus:       TaskStatusRework,
			wantReviewStatus: "changes_requested",
			wantDispatch:     dispatchStateHumanGuidanceRequested,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &TaskDocument{
				Frontmatter: TaskFrontmatter{
					TaskID:                   "T003",
					Status:                   TaskStatusBlocked,
					DispatchState:            dispatchStateNeedsHuman,
					ReviewStatus:             "blocked",
					LastBlockedReason:        "needs human decision",
					OwnerAgent:               "executor.codex",
					WakePrompt:               "resume later",
					HumanGuidanceRound:       2,
					LastHumanGuidancePath:    "old/path.md",
					LastHumanGuidanceSummary: "old summary",
				},
				LeaseUntil: time.Date(2026, 4, 2, 18, 0, 0, 0, time.FixedZone("CST", 8*3600)),
				WakeAt:     time.Date(2026, 4, 2, 19, 0, 0, 0, time.FixedZone("CST", 8*3600)),
			}

			err := applyTaskHumanGuidanceTransition(task, tt.action, "phases/P01/tasks/T003/guidance/G003.md", "follow the new operator decision", 3)
			if err != nil {
				t.Fatalf("apply task human guidance transition failed: %v", err)
			}
			if got := normalizeTaskStatus(task.Frontmatter.Status); got != tt.wantStatus {
				t.Fatalf("unexpected status: got %s want %s", got, tt.wantStatus)
			}
			if task.Frontmatter.ReviewStatus != tt.wantReviewStatus {
				t.Fatalf("unexpected review status: got %q want %q", task.Frontmatter.ReviewStatus, tt.wantReviewStatus)
			}
			if task.Frontmatter.DispatchState != tt.wantDispatch {
				t.Fatalf("unexpected dispatch state: got %q want %q", task.Frontmatter.DispatchState, tt.wantDispatch)
			}
			if task.Frontmatter.HumanGuidanceRound != 3 {
				t.Fatalf("expected human guidance round 3, got %d", task.Frontmatter.HumanGuidanceRound)
			}
			if task.Frontmatter.HumanGuidanceStatus != tt.action {
				t.Fatalf("expected human guidance status %q, got %q", tt.action, task.Frontmatter.HumanGuidanceStatus)
			}
			if task.Frontmatter.LastHumanGuidancePath != "phases/P01/tasks/T003/guidance/G003.md" {
				t.Fatalf("unexpected guidance path %q", task.Frontmatter.LastHumanGuidancePath)
			}
			if task.Frontmatter.LastHumanGuidanceSummary != "follow the new operator decision" {
				t.Fatalf("unexpected guidance summary %q", task.Frontmatter.LastHumanGuidanceSummary)
			}
			if task.Frontmatter.LastBlockedReason != "" {
				t.Fatalf("expected blocked reason cleared, got %q", task.Frontmatter.LastBlockedReason)
			}
			if task.Frontmatter.OwnerAgent != "" || !task.LeaseUntil.IsZero() || !task.WakeAt.IsZero() || task.Frontmatter.WakePrompt != "" {
				t.Fatalf("expected assignment and wake fields cleared, got owner=%q lease=%v wake=%v prompt=%q", task.Frontmatter.OwnerAgent, task.LeaseUntil, task.WakeAt, task.Frontmatter.WakePrompt)
			}
		})
	}
}

func TestApplyTaskReviewVerdictTransition(t *testing.T) {
	tests := []struct {
		name              string
		task              TaskDocument
		review            ReviewDocument
		verdict           string
		blockedGuidance   bool
		wantStatus        string
		wantReviewStatus  string
		wantDispatchState string
		wantHeadCommit    string
	}{
		{
			name: "approve source repo task",
			task: TaskDocument{
				Frontmatter: TaskFrontmatter{
					TaskID:            "T004",
					Status:            TaskStatusReviewing,
					ReviewStatus:      "reviewing",
					OwnerAgent:        "reviewer.claude",
					LastBlockedReason: "stale blocker",
					TargetRepos:       []string{"repo-a"},
					WriteScope:        []string{"repo-a:src/lib.rs"},
				},
				LeaseUntil: time.Date(2026, 4, 2, 18, 0, 0, 0, time.FixedZone("CST", 8*3600)),
				WakeAt:     time.Date(2026, 4, 2, 19, 0, 0, 0, time.FixedZone("CST", 8*3600)),
			},
			review: ReviewDocument{
				Path: "phases/P01/tasks/T004/reviews/R001.md",
				Frontmatter: ReviewFrontmatter{
					TargetCommit: "abc123",
				},
			},
			verdict:           "approve",
			wantStatus:        TaskStatusAccepted,
			wantReviewStatus:  "approved",
			wantDispatchState: dispatchStateJudgeApplied,
			wantHeadCommit:    "abc123",
		},
		{
			name: "blocking during guidance loop returns to rework",
			task: TaskDocument{
				Frontmatter: TaskFrontmatter{
					TaskID:            "T005",
					Status:            TaskStatusReviewing,
					ReviewStatus:      "reviewing",
					OwnerAgent:        "reviewer.claude",
					LastBlockedReason: "stale blocker",
				},
				LeaseUntil: time.Date(2026, 4, 2, 18, 0, 0, 0, time.FixedZone("CST", 8*3600)),
			},
			review: ReviewDocument{
				Path: "phases/P01/tasks/T005/reviews/R001.md",
			},
			verdict:           "blocking",
			blockedGuidance:   true,
			wantStatus:        TaskStatusRework,
			wantReviewStatus:  "changes_requested",
			wantDispatchState: dispatchStateBlockedGuidanceApplied,
			wantHeadCommit:    "",
		},
		{
			name: "campaign only concern clears head commit",
			task: TaskDocument{
				Frontmatter: TaskFrontmatter{
					TaskID:            "T006",
					Status:            TaskStatusReviewing,
					ReviewStatus:      "reviewing",
					OwnerAgent:        "reviewer.claude",
					HeadCommit:        "stale-commit",
					LastBlockedReason: "stale blocker",
					WriteScope:        []string{"campaign:reports/live-report.md"},
				},
				LeaseUntil: time.Date(2026, 4, 2, 18, 0, 0, 0, time.FixedZone("CST", 8*3600)),
			},
			review: ReviewDocument{
				Path: "phases/P01/tasks/T006/reviews/R001.md",
				Frontmatter: ReviewFrontmatter{
					TargetCommit: "ignored-for-campaign-only",
				},
			},
			verdict:           "concern",
			wantStatus:        TaskStatusRework,
			wantReviewStatus:  "changes_requested",
			wantDispatchState: dispatchStateJudgeApplied,
			wantHeadCommit:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := tt.task
			applyTaskReviewVerdictTransition(&task, tt.review, tt.verdict, tt.blockedGuidance)

			if got := normalizeTaskStatus(task.Frontmatter.Status); got != tt.wantStatus {
				t.Fatalf("unexpected status: got %s want %s", got, tt.wantStatus)
			}
			if task.Frontmatter.ReviewStatus != tt.wantReviewStatus {
				t.Fatalf("unexpected review status: got %q want %q", task.Frontmatter.ReviewStatus, tt.wantReviewStatus)
			}
			if task.Frontmatter.DispatchState != tt.wantDispatchState {
				t.Fatalf("unexpected dispatch state: got %q want %q", task.Frontmatter.DispatchState, tt.wantDispatchState)
			}
			if task.Frontmatter.LastReviewPath != tt.review.Path {
				t.Fatalf("expected last review path %q, got %q", tt.review.Path, task.Frontmatter.LastReviewPath)
			}
			if task.Frontmatter.LastBlockedReason != "" {
				t.Fatalf("expected blocked reason cleared, got %q", task.Frontmatter.LastBlockedReason)
			}
			if task.Frontmatter.HeadCommit != tt.wantHeadCommit {
				t.Fatalf("unexpected head commit: got %q want %q", task.Frontmatter.HeadCommit, tt.wantHeadCommit)
			}
			if task.Frontmatter.OwnerAgent != "" || !task.LeaseUntil.IsZero() || !task.WakeAt.IsZero() || task.Frontmatter.WakePrompt != "" {
				t.Fatalf("expected assignment and wake fields cleared, got owner=%q lease=%v wake=%v prompt=%q", task.Frontmatter.OwnerAgent, task.LeaseUntil, task.WakeAt, task.Frontmatter.WakePrompt)
			}
		})
	}
}
