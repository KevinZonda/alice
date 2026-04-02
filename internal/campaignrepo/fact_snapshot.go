package campaignrepo

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type FactSnapshot struct {
	Kind                      DispatchKind
	TaskID                    string
	PlanRound                 int
	Round                     int
	Status                    string
	ReviewStatus              string
	DispatchDepth             string
	ExpectedReceiptPath       string
	LastReceiptPath           string
	LastRunPath               string
	LastReviewPath            string
	LastBlockedCode           string
	LastBlockedReason         string
	LastHumanGuidancePath     string
	ArtifactRepairOnly        bool
	IntegrationConflictRepair bool
}

func buildTaskFactSnapshot(repo Repository, task TaskDocument, kind DispatchKind) FactSnapshot {
	return FactSnapshot{
		Kind:                      kind,
		TaskID:                    strings.TrimSpace(task.Frontmatter.TaskID),
		Round:                     taskPostRunRound(task, kind),
		Status:                    normalizeTaskStatus(task.Frontmatter.Status),
		ReviewStatus:              normalizeReviewStatus(task.Frontmatter.ReviewStatus),
		DispatchDepth:             effectiveTaskDispatchDepth(repo, task),
		ExpectedReceiptPath:       taskRoundReceiptPath(task, kind),
		LastReceiptPath:           filepathToSlash(task.Frontmatter.LastReceiptPath),
		LastRunPath:               filepathToSlash(task.Frontmatter.LastRunPath),
		LastReviewPath:            filepathToSlash(task.Frontmatter.LastReviewPath),
		LastBlockedCode:           strings.TrimSpace(task.Frontmatter.BlockedCode),
		LastBlockedReason:         strings.TrimSpace(task.Frontmatter.LastBlockedReason),
		LastHumanGuidancePath:     filepathToSlash(task.Frontmatter.LastHumanGuidancePath),
		ArtifactRepairOnly:        taskDispatchesArtifactRepair(task),
		IntegrationConflictRepair: taskNeedsIntegrationConflictRecovery(task),
	}
}

func buildPlanFactSnapshot(repo Repository, kind DispatchKind) FactSnapshot {
	receiptPath := repo.Campaign.Frontmatter.PlannerReceiptPath
	if kind == DispatchKindPlannerReviewer {
		receiptPath = repo.Campaign.Frontmatter.PlannerReviewerReceiptPath
	}
	return FactSnapshot{
		Kind:                kind,
		PlanRound:           repo.Campaign.Frontmatter.PlanRound,
		Round:               repo.Campaign.Frontmatter.PlanRound,
		Status:              normalizePlanStatus(repo.Campaign.Frontmatter.PlanStatus),
		DispatchDepth:       effectiveCampaignDispatchDepth(repo),
		ExpectedReceiptPath: planRoundReceiptPath(kind, repo.Campaign.Frontmatter.PlanRound),
		LastReceiptPath:     filepathToSlash(receiptPath),
	}
}

func formatFactSnapshot(snapshot FactSnapshot) string {
	lines := []string{
		fmt.Sprintf("kind=%s", snapshot.Kind),
		fmt.Sprintf("round=%d", snapshot.Round),
		fmt.Sprintf("status=%s", blankForSummary(snapshot.Status)),
		fmt.Sprintf("review_status=%s", blankForSummary(snapshot.ReviewStatus)),
		fmt.Sprintf("dispatch_depth=%s", blankForSummary(snapshot.DispatchDepth)),
		fmt.Sprintf("expected_receipt=%s", blankForSummary(snapshot.ExpectedReceiptPath)),
		fmt.Sprintf("last_receipt=%s", blankForSummary(snapshot.LastReceiptPath)),
		fmt.Sprintf("last_run=%s", blankForSummary(snapshot.LastRunPath)),
		fmt.Sprintf("last_review=%s", blankForSummary(snapshot.LastReviewPath)),
		fmt.Sprintf("blocked_code=%s", blankForSummary(snapshot.LastBlockedCode)),
		fmt.Sprintf("blocked_reason=%s", blankForSummary(snapshot.LastBlockedReason)),
		fmt.Sprintf("human_guidance=%s", blankForSummary(snapshot.LastHumanGuidancePath)),
	}
	if snapshot.PlanRound > 0 {
		lines = append(lines, fmt.Sprintf("plan_round=%d", snapshot.PlanRound))
	}
	if snapshot.TaskID != "" {
		lines = append(lines, fmt.Sprintf("task_id=%s", snapshot.TaskID))
	}
	if snapshot.ArtifactRepairOnly {
		lines = append(lines, "artifact_repair_only=yes")
	}
	if snapshot.IntegrationConflictRepair {
		lines = append(lines, "integration_conflict_recovery=yes")
	}
	return strings.Join(lines, "\n")
}

func factSnapshotDigest(snapshot FactSnapshot) string {
	sum := sha256.Sum256([]byte(formatFactSnapshot(snapshot)))
	return hex.EncodeToString(sum[:])
}

func filepathToSlash(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
}
