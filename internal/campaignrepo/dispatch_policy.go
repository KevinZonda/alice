package campaignrepo

import "strings"

const (
	DispatchDepthFast   = "fast"
	DispatchDepthNormal = "normal"
	DispatchDepthFull   = "full"

	defaultRoundRepairBudget    = 3
	defaultRoundSelfCheckBudget = 5
)

func normalizeDispatchDepth(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case DispatchDepthFast:
		return DispatchDepthFast
	case DispatchDepthFull:
		return DispatchDepthFull
	case "", DispatchDepthNormal:
		return DispatchDepthNormal
	default:
		return DispatchDepthNormal
	}
}

func effectiveCampaignDispatchDepth(repo Repository) string {
	return normalizeDispatchDepth(repo.Campaign.Frontmatter.DispatchDepth)
}

func effectiveTaskDispatchDepth(repo Repository, task TaskDocument) string {
	if depth := normalizeDispatchDepth(task.Frontmatter.DispatchDepth); depth != DispatchDepthNormal || strings.TrimSpace(task.Frontmatter.DispatchDepth) != "" {
		return depth
	}
	return effectiveCampaignDispatchDepth(repo)
}

func taskUsesFastDispatchDepth(repo Repository, task TaskDocument) bool {
	return effectiveTaskDispatchDepth(repo, task) == DispatchDepthFast
}

func dispatchRepairBudget(kind DispatchKind) int {
	switch kind {
	case DispatchKindPlannerReviewer:
		return 2
	default:
		return defaultRoundRepairBudget
	}
}

func dispatchSelfCheckBudget(kind DispatchKind) int {
	switch kind {
	case DispatchKindPlannerReviewer:
		return 3
	default:
		return defaultRoundSelfCheckBudget
	}
}
