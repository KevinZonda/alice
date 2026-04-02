package campaignrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type RoundReceiptDocument struct {
	Path        string
	Body        string
	Frontmatter RoundReceiptFrontmatter
	CreatedAt   time.Time
}

type RoundReceiptFrontmatter struct {
	Kind              string   `yaml:"kind" json:"kind,omitempty"`
	TaskID            string   `yaml:"task_id" json:"task_id,omitempty"`
	PlanRound         int      `yaml:"plan_round" json:"plan_round,omitempty"`
	Round             int      `yaml:"round" json:"round,omitempty"`
	Role              string   `yaml:"role" json:"role,omitempty"`
	Status            string   `yaml:"status" json:"status,omitempty"`
	ArtifactPaths     []string `yaml:"artifact_paths" json:"artifact_paths,omitempty"`
	IssueCodes        []string `yaml:"issue_codes" json:"issue_codes,omitempty"`
	RequestedHandoff  string   `yaml:"requested_handoff" json:"requested_handoff,omitempty"`
	RequestedSignal   string   `yaml:"requested_signal" json:"requested_signal,omitempty"`
	RepairAttempts    int      `yaml:"repair_attempts" json:"repair_attempts,omitempty"`
	SelfCheckAttempts int      `yaml:"self_check_attempts" json:"self_check_attempts,omitempty"`
	CreatedAtRaw      string   `yaml:"created_at" json:"created_at,omitempty"`
}

func taskRoundReceiptPath(task TaskDocument, kind DispatchKind) string {
	round := maxInt(taskPostRunRound(task, kind), 1)
	switch kind {
	case DispatchKindReviewer:
		return filepath.ToSlash(filepath.Join(task.Dir, "reviews", "receipts", fmt.Sprintf("reviewer-round-%03d.md", round)))
	default:
		return filepath.ToSlash(filepath.Join(task.Dir, "results", "receipts", fmt.Sprintf("executor-round-%03d.md", round)))
	}
}

func planRoundReceiptPath(kind DispatchKind, round int) string {
	label := "planner"
	if kind == DispatchKindPlannerReviewer {
		label = "planner-reviewer"
	}
	return filepath.ToSlash(filepath.Join("plans", "receipts", fmt.Sprintf("%s-round-%03d.md", label, maxInt(round, 1))))
}

func loadTaskRoundReceipt(root string, task TaskDocument, kind DispatchKind) (RoundReceiptDocument, bool, error) {
	path := taskRoundReceiptPath(task, kind)
	if path == "" {
		return RoundReceiptDocument{}, false, nil
	}
	return loadRoundReceiptDocument(root, path)
}

func loadPlanRoundReceipt(root string, kind DispatchKind, round int) (RoundReceiptDocument, bool, error) {
	path := planRoundReceiptPath(kind, round)
	if path == "" {
		return RoundReceiptDocument{}, false, nil
	}
	return loadRoundReceiptDocument(root, path)
}

func loadRoundReceiptDocument(root, relPath string) (RoundReceiptDocument, bool, error) {
	relPath = filepath.ToSlash(strings.TrimSpace(relPath))
	if relPath == "" {
		return RoundReceiptDocument{}, false, nil
	}
	fullPath := filepath.Join(root, filepath.FromSlash(relPath))
	raw, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return RoundReceiptDocument{}, false, nil
		}
		return RoundReceiptDocument{}, false, err
	}
	parsed := parseMarkdownFrontmatter(string(raw))
	if !parsed.Found {
		return RoundReceiptDocument{}, false, fmt.Errorf("round receipt %s is missing frontmatter", relPath)
	}
	var frontmatter RoundReceiptFrontmatter
	if err := yaml.Unmarshal([]byte(parsed.Frontmatter), &frontmatter); err != nil {
		return RoundReceiptDocument{}, false, fmt.Errorf("parse round receipt frontmatter %s: %w", relPath, err)
	}
	doc := RoundReceiptDocument{
		Path: filepath.ToSlash(relPath),
		Body: parsed.Body,
		Frontmatter: RoundReceiptFrontmatter{
			Kind:              strings.ToLower(strings.TrimSpace(frontmatter.Kind)),
			TaskID:            strings.TrimSpace(frontmatter.TaskID),
			PlanRound:         maxInt(frontmatter.PlanRound, 0),
			Round:             maxInt(frontmatter.Round, 0),
			Role:              strings.TrimSpace(frontmatter.Role),
			Status:            strings.TrimSpace(frontmatter.Status),
			ArtifactPaths:     normalizeStringList(frontmatter.ArtifactPaths),
			IssueCodes:        normalizeStringList(frontmatter.IssueCodes),
			RequestedHandoff:  strings.TrimSpace(frontmatter.RequestedHandoff),
			RequestedSignal:   strings.TrimSpace(frontmatter.RequestedSignal),
			RepairAttempts:    maxInt(frontmatter.RepairAttempts, 0),
			SelfCheckAttempts: maxInt(frontmatter.SelfCheckAttempts, 0),
			CreatedAtRaw:      strings.TrimSpace(frontmatter.CreatedAtRaw),
		},
	}
	if createdAt, err := parseFlexibleTime(doc.Frontmatter.CreatedAtRaw); err == nil {
		doc.CreatedAt = createdAt
	}
	return doc, true, nil
}

func recordTaskRoundReceiptPath(task *TaskDocument, kind DispatchKind) {
	if task == nil {
		return
	}
	task.Frontmatter.LastReceiptPath = taskRoundReceiptPath(*task, kind)
}

func recordPlanRoundReceiptPath(frontmatter *CampaignFrontmatter, kind DispatchKind, round int) {
	if frontmatter == nil {
		return
	}
	path := planRoundReceiptPath(kind, round)
	switch kind {
	case DispatchKindPlanner:
		frontmatter.PlannerReceiptPath = path
	case DispatchKindPlannerReviewer:
		frontmatter.PlannerReviewerReceiptPath = path
	}
}
