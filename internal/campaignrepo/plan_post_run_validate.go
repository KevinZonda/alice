package campaignrepo

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func ValidatePlanPostRun(root string, kind DispatchKind, round int) (ValidationResult, error) {
	repo, err := Load(root)
	if err != nil {
		return ValidationResult{}, err
	}
	return validatePlanPostRun(repo, kind, round, true), nil
}

func RunPlanSelfCheck(root string, kind DispatchKind, round int, checkedAt time.Time) (ValidationResult, error) {
	repo, err := Load(root)
	if err != nil {
		return ValidationResult{}, err
	}
	validation := validatePlanPostRun(repo, kind, round, false)
	if checkedAt.IsZero() {
		checkedAt = time.Now().Local()
	}
	recordPlanSelfCheck(&repo.Campaign.Frontmatter, kind, round, validation.Valid, checkedAt)
	if _, err := persistCampaignDocument(&repo); err != nil {
		return ValidationResult{}, err
	}
	return validation, nil
}

func validatePlanPostRun(repo Repository, kind DispatchKind, round int, requireSelfCheckProof bool) ValidationResult {
	var issues []ValidationIssue
	switch kind {
	case DispatchKindPlanner, DispatchKindPlannerReviewer:
	default:
		return ValidationResult{Valid: true}
	}
	if round <= 0 {
		issues = append(issues, ValidationIssue{
			Code:    "plan_round_invalid",
			Path:    repo.Campaign.Path,
			Message: fmt.Sprintf("plan post-run validation requires round > 0 for %s", kind),
		})
		return ValidationResult{Valid: false, Issues: issues}
	}

	core := validateRepository(repo, false, false)
	issues = append(issues, core.Issues...)

	switch kind {
	case DispatchKindPlanner:
		validatePlannerRoundArtifacts(repo, round, &issues)
	case DispatchKindPlannerReviewer:
		validatePlannerReviewerRoundArtifacts(repo, round, &issues)
	}

	if requireSelfCheckProof {
		validatePlanSelfCheckProof(repo, kind, round, &issues)
	}

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Code != issues[j].Code {
			return issues[i].Code < issues[j].Code
		}
		if issues[i].Path != issues[j].Path {
			return issues[i].Path < issues[j].Path
		}
		return issues[i].Message < issues[j].Message
	})
	return ValidationResult{
		Valid:  len(issues) == 0,
		Issues: issues,
	}
}

func planningSelfCheckIssues(repo Repository) []ValidationIssue {
	round := repo.Campaign.Frontmatter.PlanRound
	if round <= 0 {
		return nil
	}
	var issues []ValidationIssue
	switch normalizePlanStatus(repo.Campaign.Frontmatter.PlanStatus) {
	case PlanStatusPlanning:
		if hasSubmittedProposal(repo.PlanProposals, round) {
			validatePlanSelfCheckProof(repo, DispatchKindPlanner, round, &issues)
		}
	case PlanStatusPlanReviewPending, PlanStatusPlanReviewing:
		if _, ok := latestPlanReviewForRound(repo.PlanReviews, round); ok {
			validatePlanSelfCheckProof(repo, DispatchKindPlannerReviewer, round, &issues)
		}
	}
	return issues
}

func validatePlannerRoundArtifacts(repo Repository, round int, issues *[]ValidationIssue) {
	if repo.Campaign.Frontmatter.PlanRound > 0 && repo.Campaign.Frontmatter.PlanRound != round {
		*issues = append(*issues, ValidationIssue{
			Code:    "planner_round_mismatch",
			Path:    repo.Campaign.Path,
			Message: fmt.Sprintf("planner self-check ran for round %d, but campaign is on round %d", round, repo.Campaign.Frontmatter.PlanRound),
		})
	}
	proposal, ok := latestProposalForRound(repo.PlanProposals, round)
	if !ok {
		*issues = append(*issues, ValidationIssue{
			Code:    "planner_current_round_proposal_missing",
			Path:    filepath.ToSlash(filepath.Join("plans", "proposals")),
			Message: fmt.Sprintf("planner round %d must write a submitted proposal", round),
		})
	} else if proposal.Frontmatter.Status != "submitted" {
		*issues = append(*issues, ValidationIssue{
			Code:    "planner_current_round_proposal_not_submitted",
			Path:    proposal.Path,
			Message: fmt.Sprintf("planner round %d proposal must have status submitted, got %s", round, blankForSummary(proposal.Frontmatter.Status)),
		})
	}
	validateMasterPlanArtifact(repo, issues)
	if len(repo.Phases) == 0 {
		*issues = append(*issues, ValidationIssue{
			Code:    "planner_phase_tree_empty",
			Path:    filepath.ToSlash(filepath.Join("phases")),
			Message: "planner must expand at least one phase document",
		})
	}
	if len(repo.Tasks) == 0 {
		*issues = append(*issues, ValidationIssue{
			Code:    "planner_task_tree_empty",
			Path:    filepath.ToSlash(filepath.Join("phases")),
			Message: "planner must expand at least one task package",
		})
	}
}

func validatePlannerReviewerRoundArtifacts(repo Repository, round int, issues *[]ValidationIssue) {
	if repo.Campaign.Frontmatter.PlanRound > 0 && repo.Campaign.Frontmatter.PlanRound != round {
		*issues = append(*issues, ValidationIssue{
			Code:    "planner_reviewer_round_mismatch",
			Path:    repo.Campaign.Path,
			Message: fmt.Sprintf("planner reviewer self-check ran for round %d, but campaign is on round %d", round, repo.Campaign.Frontmatter.PlanRound),
		})
	}
	proposal, ok := latestProposalForRound(repo.PlanProposals, round)
	if !ok || proposal.Frontmatter.Status != "submitted" {
		*issues = append(*issues, ValidationIssue{
			Code:    "planner_reviewer_current_round_proposal_missing",
			Path:    filepath.ToSlash(filepath.Join("plans", "proposals")),
			Message: fmt.Sprintf("planner reviewer round %d requires a submitted proposal", round),
		})
	}
	review, ok := latestPlanReviewForRound(repo.PlanReviews, round)
	if !ok {
		*issues = append(*issues, ValidationIssue{
			Code:    "planner_reviewer_current_round_review_missing",
			Path:    filepath.ToSlash(filepath.Join("plans", "reviews")),
			Message: fmt.Sprintf("planner reviewer round %d must write a plan review", round),
		})
		return
	}
	validatePlanReviewApproveStrictness(review, issues)
}

func validatePlanProposalDocument(proposal PlanProposalDocument, issues *[]ValidationIssue) {
	if !strings.HasPrefix(filepath.ToSlash(proposal.Path), "plans/proposals/") {
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_proposal_path_mismatch",
			Path:    proposal.Path,
			Message: fmt.Sprintf("plan proposal must live under plans/proposals/, got %s", proposal.Path),
		})
	}
	if strings.TrimSpace(proposal.Frontmatter.ProposalID) == "" {
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_proposal_id_missing",
			Path:    proposal.Path,
			Message: "plan proposal frontmatter.proposal_id is empty",
		})
	}
	if proposal.Frontmatter.PlanRound <= 0 {
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_proposal_round_missing",
			Path:    proposal.Path,
			Message: "plan proposal frontmatter.plan_round must be > 0",
		})
	}
	switch proposal.Frontmatter.Status {
	case "draft", "submitted", "superseded":
	default:
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_proposal_status_invalid",
			Path:    proposal.Path,
			Message: fmt.Sprintf("plan proposal status must be draft/submitted/superseded, got %s", blankForSummary(proposal.Frontmatter.Status)),
		})
	}
	requireMarkdownSection(proposal.Path, proposal.Body, "Analysis", issues, "plan_proposal_analysis_missing")
	requireMarkdownSection(proposal.Path, proposal.Body, "Phases", issues, "plan_proposal_phases_missing")
	requireAnyMarkdownSection(proposal.Path, proposal.Body, []string{"Task Breakdown", "Tasks"}, issues, "plan_proposal_tasks_missing")
	requireMarkdownSection(proposal.Path, proposal.Body, "Risks", issues, "plan_proposal_risks_missing")
}

func validatePlanReviewDocument(review PlanReviewDocument, issues *[]ValidationIssue) {
	if !strings.HasPrefix(filepath.ToSlash(review.Path), "plans/reviews/") {
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_review_path_mismatch",
			Path:    review.Path,
			Message: fmt.Sprintf("plan review must live under plans/reviews/, got %s", review.Path),
		})
	}
	if strings.TrimSpace(review.Frontmatter.ReviewID) == "" {
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_review_id_missing",
			Path:    review.Path,
			Message: "plan review frontmatter.review_id is empty",
		})
	}
	if review.Frontmatter.PlanRound <= 0 {
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_review_round_missing",
			Path:    review.Path,
			Message: "plan review frontmatter.plan_round must be > 0",
		})
	}
	if strings.TrimSpace(review.Frontmatter.Reviewer.Role) == "" {
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_review_role_missing",
			Path:    review.Path,
			Message: "plan review frontmatter.reviewer.role is empty",
		})
	}
	validateRoleWorkflow(review.Path, "plan review", fmt.Sprintf("round-%03d", maxInt(review.Frontmatter.PlanRound, 1)), "planner_reviewer", review.Frontmatter.Reviewer.Workflow, issues)
	if normalizeReviewVerdict(review.Frontmatter.Verdict, review.Frontmatter.Blocking) == "" {
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_review_verdict_missing",
			Path:    review.Path,
			Message: "plan review must set verdict to approve/concern/blocking",
		})
	}
	if strings.TrimSpace(review.Frontmatter.CreatedAtRaw) == "" {
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_review_created_at_missing",
			Path:    review.Path,
			Message: "plan review must set created_at",
		})
	}
	verdict := normalizeReviewVerdict(review.Frontmatter.Verdict, review.Frontmatter.Blocking)
	if verdict == "blocking" && !review.Frontmatter.Blocking {
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_review_blocking_flag_mismatch",
			Path:    review.Path,
			Message: "plan review with verdict blocking must set blocking: true",
		})
	}
	if verdict != "" && verdict != "blocking" && review.Frontmatter.Blocking {
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_review_blocking_flag_unexpected",
			Path:    review.Path,
			Message: fmt.Sprintf("plan review with verdict %s must set blocking: false", verdict),
		})
	}
	requireMarkdownSection(review.Path, review.Body, "Summary", issues, "plan_review_summary_missing")
	requireMarkdownSection(review.Path, review.Body, "Findings", issues, "plan_review_findings_missing")
	requireAnyMarkdownSection(review.Path, review.Body, []string{"Verdict", "Conclusion"}, issues, "plan_review_conclusion_missing")
	validatePlanReviewApproveStrictness(review, issues)
}

func validatePlanReviewApproveStrictness(review PlanReviewDocument, issues *[]ValidationIssue) {
	if normalizeReviewVerdict(review.Frontmatter.Verdict, review.Frontmatter.Blocking) != "approve" {
		return
	}
	if !isPlaceholderText(markdownSectionContent(review.Body, "Concerns")) {
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_review_approve_has_concerns",
			Path:    review.Path,
			Message: "plan review verdict approve cannot keep a non-empty Concerns section; any remaining issue must be concern/blocking",
		})
	}
}

func validateMasterPlanArtifact(repo Repository, issues *[]ValidationIssue) {
	path := filepath.Join(repo.Root, "plans", "merged", "master-plan.md")
	body, err := loadMarkdownBodyIfExists(path)
	if err != nil {
		*issues = append(*issues, ValidationIssue{
			Code:    "master_plan_read_failed",
			Path:    filepath.ToSlash(relativePath(repo.Root, path)),
			Message: fmt.Sprintf("read master plan failed: %v", err),
		})
		return
	}
	if isPlaceholderText(body) {
		*issues = append(*issues, ValidationIssue{
			Code:    "master_plan_missing",
			Path:    filepath.ToSlash(relativePath(repo.Root, path)),
			Message: "master-plan.md must exist and contain a concrete refined plan",
		})
		return
	}
	requireMarkdownSection(filepath.ToSlash(relativePath(repo.Root, path)), body, "Analysis", issues, "master_plan_analysis_missing")
	requireMarkdownSection(filepath.ToSlash(relativePath(repo.Root, path)), body, "Phases", issues, "master_plan_phases_missing")
	requireAnyMarkdownSection(filepath.ToSlash(relativePath(repo.Root, path)), body, []string{"Task Breakdown", "Tasks"}, issues, "master_plan_tasks_missing")
	requireMarkdownSection(filepath.ToSlash(relativePath(repo.Root, path)), body, "Risks", issues, "master_plan_risks_missing")
}

func validatePlanSelfCheckProof(repo Repository, kind DispatchKind, round int, issues *[]ValidationIssue) {
	recordedRound, status, atRaw := planSelfCheckProof(repo.Campaign.Frontmatter, kind)
	label := string(kind)
	if recordedRound <= 0 {
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_post_run_self_check_missing",
			Path:    repo.Campaign.Path,
			Message: fmt.Sprintf("%s round %d has no recorded self-check proof", label, round),
		})
		return
	}
	if recordedRound != round {
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_post_run_self_check_round_mismatch",
			Path:    repo.Campaign.Path,
			Message: fmt.Sprintf("%s round %d has self-check proof for round %d", label, round, recordedRound),
		})
	}
	if status != taskSelfCheckStatusPassed {
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_post_run_self_check_not_passed",
			Path:    repo.Campaign.Path,
			Message: fmt.Sprintf("%s round %d self-check status is %q instead of %q", label, round, blankForSummary(status), taskSelfCheckStatusPassed),
		})
	}
	if strings.TrimSpace(atRaw) == "" {
		*issues = append(*issues, ValidationIssue{
			Code:    "plan_post_run_self_check_timestamp_missing",
			Path:    repo.Campaign.Path,
			Message: fmt.Sprintf("%s round %d self_check_at is empty", label, round),
		})
	}
}

func planSelfCheckProof(frontmatter CampaignFrontmatter, kind DispatchKind) (int, string, string) {
	switch kind {
	case DispatchKindPlanner:
		return frontmatter.PlannerSelfCheckRound, normalizeTaskSelfCheckStatus(frontmatter.PlannerSelfCheckStatus), strings.TrimSpace(frontmatter.PlannerSelfCheckAtRaw)
	case DispatchKindPlannerReviewer:
		return frontmatter.PlannerReviewerSelfCheckRound, normalizeTaskSelfCheckStatus(frontmatter.PlannerReviewerSelfCheckStatus), strings.TrimSpace(frontmatter.PlannerReviewerSelfCheckAtRaw)
	default:
		return 0, "", ""
	}
}

func recordPlanSelfCheck(frontmatter *CampaignFrontmatter, kind DispatchKind, round int, passed bool, checkedAt time.Time) {
	if frontmatter == nil {
		return
	}
	status := taskSelfCheckStatusFailed
	if passed {
		status = taskSelfCheckStatusPassed
	}
	switch kind {
	case DispatchKindPlanner:
		frontmatter.PlannerSelfCheckRound = round
		frontmatter.PlannerSelfCheckStatus = status
		frontmatter.PlannerSelfCheckAtRaw = checkedAt.Format(time.RFC3339)
	case DispatchKindPlannerReviewer:
		frontmatter.PlannerReviewerSelfCheckRound = round
		frontmatter.PlannerReviewerSelfCheckStatus = status
		frontmatter.PlannerReviewerSelfCheckAtRaw = checkedAt.Format(time.RFC3339)
	}
}

func clearPlanSelfCheck(frontmatter *CampaignFrontmatter, kind DispatchKind) {
	if frontmatter == nil {
		return
	}
	switch kind {
	case DispatchKindPlanner:
		frontmatter.PlannerSelfCheckRound = 0
		frontmatter.PlannerSelfCheckStatus = ""
		frontmatter.PlannerSelfCheckAtRaw = ""
	case DispatchKindPlannerReviewer:
		frontmatter.PlannerReviewerSelfCheckRound = 0
		frontmatter.PlannerReviewerSelfCheckStatus = ""
		frontmatter.PlannerReviewerSelfCheckAtRaw = ""
	}
}

func clearPlanningSelfCheckProofs(frontmatter *CampaignFrontmatter) {
	clearPlanSelfCheck(frontmatter, DispatchKindPlanner)
	clearPlanSelfCheck(frontmatter, DispatchKindPlannerReviewer)
}
