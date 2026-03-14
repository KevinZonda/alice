package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Alice-space/alice/internal/config"
)

func TestEnsureBundledSkillsLinked_InstallsEmbeddedSkills(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv(config.EnvCodexHome, codexHome)

	report, err := EnsureBundledSkillsLinked(t.TempDir())
	if err != nil {
		t.Fatalf("sync bundled skills failed: %v", err)
	}
	if report.Discovered <= 0 {
		t.Fatalf("expected discovered skills > 0, got %+v", report)
	}
	if report.Linked <= 0 {
		t.Fatalf("expected linked skills > 0 on first sync, got %+v", report)
	}

	skillDir := filepath.Join(codexHome, "skills", "alice-message")
	if isSymlink(t, skillDir) {
		t.Fatalf("embedded install should create regular directory, got symlink: %s", skillDir)
	}
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Fatalf("embedded skill manifest missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillDir, embeddedSkillMarkerFile)); err != nil {
		t.Fatalf("embedded skill marker missing: %v", err)
	}
}

func TestEnsureBundledSkillsLinked_ReplacesSymlink(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv(config.EnvCodexHome, codexHome)

	skillDir := filepath.Join(codexHome, "skills", "alice-message")
	if err := os.MkdirAll(filepath.Dir(skillDir), 0o755); err != nil {
		t.Fatalf("create skills dir failed: %v", err)
	}
	legacy := t.TempDir()
	if err := os.Symlink(legacy, skillDir); err != nil {
		t.Fatalf("seed legacy symlink failed: %v", err)
	}

	report, err := EnsureBundledSkillsLinked(t.TempDir())
	if err != nil {
		t.Fatalf("sync bundled skills failed: %v", err)
	}
	if report.Updated <= 0 {
		t.Fatalf("expected updated skills > 0 when replacing symlink, got %+v", report)
	}
	if isSymlink(t, skillDir) {
		t.Fatalf("symlink should be replaced by real directory: %s", skillDir)
	}
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Fatalf("embedded skill manifest missing after symlink replacement: %v", err)
	}
}

func TestEnsureBundledSkillsLinked_KeepCustomDirectory(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv(config.EnvCodexHome, codexHome)

	skillDir := filepath.Join(codexHome, "skills", "alice-message")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("create custom skill dir failed: %v", err)
	}
	custom := []byte("custom-skill\n")
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), custom, 0o644); err != nil {
		t.Fatalf("write custom skill file failed: %v", err)
	}

	report, err := EnsureBundledSkillsLinked(t.TempDir())
	if err != nil {
		t.Fatalf("sync bundled skills failed: %v", err)
	}
	if report.Unchanged <= 0 {
		t.Fatalf("expected unchanged skills > 0 for custom directory, got %+v", report)
	}

	raw, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read custom skill file failed: %v", err)
	}
	if string(raw) != string(custom) {
		t.Fatalf("custom skill should not be overwritten, got=%q want=%q", string(raw), string(custom))
	}
}

func isSymlink(t *testing.T, path string) bool {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat failed path=%s err=%v", path, err)
	}
	return info.Mode()&os.ModeSymlink != 0
}
