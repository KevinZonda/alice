package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SkillLinkReport struct {
	CodexHome  string
	Discovered int
	Linked     int
	Updated    int
	BackedUp   int
	Unchanged  int
	Failed     int
}

func EnsureBundledSkillsLinked(workspaceDir string) (SkillLinkReport, error) {
	report := SkillLinkReport{}

	workspaceDir = strings.TrimSpace(workspaceDir)
	if workspaceDir == "" {
		workspaceDir = "."
	}
	workspaceAbs, err := filepath.Abs(workspaceDir)
	if err != nil {
		return report, fmt.Errorf("resolve workspace dir failed: %w", err)
	}

	repoSkillsRoot := filepath.Join(workspaceAbs, "skills")
	entries, err := os.ReadDir(repoSkillsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return report, nil
		}
		return report, fmt.Errorf("read bundled skills dir failed: %w", err)
	}

	codexHome, err := resolveCodexHome()
	if err != nil {
		return report, err
	}
	report.CodexHome = codexHome
	dstRoot := filepath.Join(codexHome, "skills")
	if err := os.MkdirAll(dstRoot, 0o755); err != nil {
		return report, fmt.Errorf("create codex skills dir failed: %w", err)
	}

	for _, entry := range entries {
		if entry == nil || !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" || strings.HasPrefix(name, ".") {
			continue
		}
		src := filepath.Join(repoSkillsRoot, name)
		if !hasSkillManifest(src) {
			continue
		}
		report.Discovered++

		dst := filepath.Join(dstRoot, name)
		changed, backedUp, failed := ensureSkillSymlink(src, dst)
		if failed {
			report.Failed++
			continue
		}
		if backedUp {
			report.BackedUp++
		}
		switch changed {
		case "linked":
			report.Linked++
		case "updated":
			report.Updated++
		default:
			report.Unchanged++
		}
	}

	return report, nil
}

func resolveCodexHome() (string, error) {
	codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME"))
	if codexHome != "" {
		return codexHome, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home failed: %w", err)
	}
	return filepath.Join(home, ".codex"), nil
}

func hasSkillManifest(skillDir string) bool {
	info, err := os.Stat(filepath.Join(skillDir, "SKILL.md"))
	return err == nil && !info.IsDir()
}

func ensureSkillSymlink(src, dst string) (changed string, backedUp bool, failed bool) {
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return "", false, true
	}

	info, err := os.Lstat(dst)
	if err != nil {
		if os.IsNotExist(err) {
			if linkErr := os.Symlink(srcAbs, dst); linkErr != nil {
				return "", false, true
			}
			return "linked", false, false
		}
		return "", false, true
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, readErr := os.Readlink(dst)
		if readErr != nil {
			return "", false, true
		}
		targetAbs := target
		if !filepath.IsAbs(targetAbs) {
			targetAbs = filepath.Join(filepath.Dir(dst), targetAbs)
		}
		targetAbs, _ = filepath.Abs(targetAbs)
		if filepath.Clean(targetAbs) == filepath.Clean(srcAbs) {
			return "unchanged", false, false
		}
		if removeErr := os.Remove(dst); removeErr != nil {
			return "", false, true
		}
		if linkErr := os.Symlink(srcAbs, dst); linkErr != nil {
			return "", false, true
		}
		return "updated", false, false
	}

	backupPath := fmt.Sprintf("%s.backup-%s", dst, time.Now().Format("20060102150405"))
	if renameErr := os.Rename(dst, backupPath); renameErr != nil {
		return "", false, true
	}
	if linkErr := os.Symlink(srcAbs, dst); linkErr != nil {
		_ = os.Rename(backupPath, dst)
		return "", false, true
	}
	return "linked", true, false
}
