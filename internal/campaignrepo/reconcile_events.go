package campaignrepo

// ReconcileEventKind is the type of a reconcile event.
type ReconcileEventKind string

const (
	EventPlanningStarted      ReconcileEventKind = "planning_started"
	EventProposalSubmitted    ReconcileEventKind = "proposal_submitted"
	EventPlanReviewVerdict    ReconcileEventKind = "plan_review_verdict"
	EventHumanApprovalNeeded  ReconcileEventKind = "human_approval_needed"
	EventPlanApproved         ReconcileEventKind = "plan_approved"
	EventPlanningBlocked      ReconcileEventKind = "planning_blocked"
	EventTaskDispatched       ReconcileEventKind = "task_dispatched"
	EventTaskIntegrated       ReconcileEventKind = "task_integrated"
	EventTaskRetrying         ReconcileEventKind = "task_retrying"
	EventTaskRecovered        ReconcileEventKind = "task_recovered"
	EventReviewVerdictApplied ReconcileEventKind = "review_verdict_applied"
	EventJudgeDeferred        ReconcileEventKind = "judge_deferred"
	EventReplanRequested      ReconcileEventKind = "replan_requested"
	EventTaskBlocked          ReconcileEventKind = "task_blocked"
	EventTaskNeedsHuman       ReconcileEventKind = "task_needs_human"
	EventAutomationFailed     ReconcileEventKind = "automation_failed"
	EventDiscoveryReported    ReconcileEventKind = "discovery_reported"
	EventCampaignCompleted    ReconcileEventKind = "campaign_completed"
)

// ReconcileEvent represents a state change event produced during reconciliation.
type ReconcileEvent struct {
	Kind       ReconcileEventKind
	CampaignID string
	PlanRound  int
	TaskID     string // empty for campaign-level events
	Title      string
	Detail     string
	Severity   string // "info" | "success" | "warning" | "error"
}
