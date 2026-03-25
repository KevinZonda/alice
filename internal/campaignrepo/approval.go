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
