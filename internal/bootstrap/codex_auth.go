package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Alice-space/alice/internal/config"
)

type CodexAuthSyncReport struct {
	Target string
	Source string
	Copied bool
}

func EnsureCodexAuthForCodexHome(targetCodexHome string, sourceCodexHomes ...string) (CodexAuthSyncReport, error) {
	targetCodexHome = strings.TrimSpace(targetCodexHome)
	if targetCodexHome == "" {
		return CodexAuthSyncReport{}, fmt.Errorf("codex home is empty")
	}

	targetPath := filepath.Join(targetCodexHome, "auth.json")
	report := CodexAuthSyncReport{Target: targetPath}

	info, err := os.Stat(targetPath)
	switch {
	case err == nil:
		if info.IsDir() {
			return report, fmt.Errorf("target auth path is a directory: %s", targetPath)
		}
		return report, nil
	case !os.IsNotExist(err):
		return report, fmt.Errorf("stat target auth failed: %w", err)
	}

	if err := os.MkdirAll(targetCodexHome, 0o755); err != nil {
		return report, fmt.Errorf("create codex home failed: %w", err)
	}

	for _, sourcePath := range codexAuthSourceCandidates(targetPath, sourceCodexHomes...) {
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return report, fmt.Errorf("read source auth %q failed: %w", sourcePath, err)
		}
		if err := os.WriteFile(targetPath, content, 0o600); err != nil {
			return report, fmt.Errorf("write target auth failed: %w", err)
		}
		report.Source = sourcePath
		report.Copied = true
		return report, nil
	}

	return report, nil
}

func codexAuthSourceCandidates(targetPath string, sourceCodexHomes ...string) []string {
	candidates := make([]string, 0, len(sourceCodexHomes)+4)
	seen := make(map[string]struct{}, len(sourceCodexHomes)+4)

	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if filepath.Base(path) != "auth.json" {
			path = filepath.Join(path, "auth.json")
		}
		path = filepath.Clean(path)
		if path == targetPath {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		candidates = append(candidates, path)
	}

	for _, sourceCodexHome := range sourceCodexHomes {
		add(sourceCodexHome)
	}
	add(os.Getenv(config.EnvCodexHome))
	add(config.DefaultCodexHome())
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		add(filepath.Join(home, ".codex", "auth.json"))
		add(filepath.Join(home, ".config", "codex", "auth.json"))
	}

	return candidates
}
