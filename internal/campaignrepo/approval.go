package campaignrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

func ApprovePlan(root string) (Repository, ValidationResult, error) {
	repo, validation, err := ValidateForApproval(root)
	if err != nil {
		return Repository{}, ValidationResult{}, err
	}
	if !validation.Valid {
		return repo, validation, nil
	}
	repo.Campaign.Frontmatter.PlanStatus = PlanStatusHumanApproved
	if _, err := persistCampaignDocument(&repo); err != nil {
		return repo, validation, err
	}
	if err := markMasterPlanHumanApproved(repo.Root); err != nil {
		return repo, validation, err
	}
	updated, err := Load(repo.Root)
	if err != nil {
		return repo, validation, err
	}
	return updated, validation, nil
}

func RejectPlan(root string) (Repository, error) {
	repo, err := Load(root)
	if err != nil {
		return Repository{}, err
	}
	if normalizePlanStatus(repo.Campaign.Frontmatter.PlanStatus) != PlanStatusPlanApproved {
		return Repository{}, fmt.Errorf("reject-plan requires %s, got %s", PlanStatusPlanApproved, normalizePlanStatus(repo.Campaign.Frontmatter.PlanStatus))
	}
	if err := markCurrentProposalSuperseded(&repo); err != nil {
		return Repository{}, err
	}
	repo.Campaign.Frontmatter.PlanRound++
	repo.Campaign.Frontmatter.PlanStatus = PlanStatusPlanning
	if _, err := persistCampaignDocument(&repo); err != nil {
		return Repository{}, err
	}
	if err := markMasterPlanHumanRejected(repo.Root); err != nil {
		return Repository{}, err
	}
	return Load(repo.Root)
}

func markMasterPlanHumanApproved(root string) error {
	path := filepath.Join(root, "plans", "merged", "master-plan.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read master plan: %w", err)
	}
	parsed := parseMarkdownFrontmatter(string(raw))
	meta := map[string]any{}
	if parsed.Found {
		if err := yaml.Unmarshal([]byte(parsed.Frontmatter), &meta); err != nil {
			return fmt.Errorf("parse master plan frontmatter: %w", err)
		}
	}
	meta["status"] = "approved"
	meta["human_approved"] = true
	frontmatter, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	rendered := "---\n" + strings.TrimRight(string(frontmatter), "\n") + "\n---\n"
	body := strings.TrimSpace(parsed.Body)
	if body != "" {
		rendered += "\n" + body + "\n"
	}
	return writeFileIfChanged(path, []byte(rendered))
}

func markMasterPlanHumanRejected(root string) error {
	path := filepath.Join(root, "plans", "merged", "master-plan.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read master plan: %w", err)
	}
	parsed := parseMarkdownFrontmatter(string(raw))
	meta := map[string]any{}
	if parsed.Found {
		if err := yaml.Unmarshal([]byte(parsed.Frontmatter), &meta); err != nil {
			return fmt.Errorf("parse master plan frontmatter: %w", err)
		}
	}
	meta["status"] = "rejected"
	meta["human_approved"] = false
	meta["human_rejected"] = true
	frontmatter, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	rendered := "---\n" + strings.TrimRight(string(frontmatter), "\n") + "\n---\n"
	body := strings.TrimSpace(parsed.Body)
	if body != "" {
		rendered += "\n" + body + "\n"
	}
	return writeFileIfChanged(path, []byte(rendered))
}
