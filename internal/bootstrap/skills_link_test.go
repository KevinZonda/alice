package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureBundledSkillsLinked_NewLinks(t *testing.T) {
	workspace := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	skillA := makeSkillDir(t, workspace, "alice-codebase-onboarding")
	_ = makeNonSkillDir(t, workspace, "notes")

	report, err := EnsureBundledSkillsLinked(workspace)
	if err != nil {
		t.Fatalf("link bundled skills failed: %v", err)
	}
	if report.Discovered != 1 || report.Linked != 1 || report.Failed != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}

	dst := filepath.Join(codexHome, "skills", "alice-codebase-onboarding")
	assertSymlinkTarget(t, dst, skillA)
}

func TestEnsureBundledSkillsLinked_UpdatesSymlink(t *testing.T) {
	workspace := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	skillA := makeSkillDir(t, workspace, "feishu-task")
	dst := filepath.Join(codexHome, "skills", "feishu-task")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("create destination dir failed: %v", err)
	}
	oldTarget := t.TempDir()
	if err := os.Symlink(oldTarget, dst); err != nil {
		t.Fatalf("seed old symlink failed: %v", err)
	}

	report, err := EnsureBundledSkillsLinked(workspace)
	if err != nil {
		t.Fatalf("link bundled skills failed: %v", err)
	}
	if report.Updated != 1 || report.Failed != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
	assertSymlinkTarget(t, dst, skillA)
}

func TestEnsureBundledSkillsLinked_BackupExistingDir(t *testing.T) {
	workspace := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	skillA := makeSkillDir(t, workspace, "alice-codebase-onboarding")
	dst := filepath.Join(codexHome, "skills", "alice-codebase-onboarding")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("seed conflicting dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dst, "SKILL.md"), []byte("legacy"), 0o644); err != nil {
		t.Fatalf("seed legacy skill failed: %v", err)
	}

	report, err := EnsureBundledSkillsLinked(workspace)
	if err != nil {
		t.Fatalf("link bundled skills failed: %v", err)
	}
	if report.BackedUp != 1 || report.Linked != 1 || report.Failed != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
	assertSymlinkTarget(t, dst, skillA)

	backups, err := filepath.Glob(dst + ".backup-*")
	if err != nil {
		t.Fatalf("glob backup failed: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected exactly one backup, got %d", len(backups))
	}
}

func makeSkillDir(t *testing.T, workspace, name string) string {
	t.Helper()
	path := filepath.Join(workspace, "skills", name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("create skill dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("name: test"), 0o644); err != nil {
		t.Fatalf("create SKILL.md failed: %v", err)
	}
	return path
}

func makeNonSkillDir(t *testing.T, workspace, name string) string {
	t.Helper()
	path := filepath.Join(workspace, "skills", name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("create non-skill dir failed: %v", err)
	}
	return path
}

func assertSymlinkTarget(t *testing.T, path, expectedTarget string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat symlink failed: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink, got mode=%v", info.Mode())
	}
	target, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("readlink failed: %v", err)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	target, _ = filepath.Abs(target)
	expectedTarget, _ = filepath.Abs(expectedTarget)
	if filepath.Clean(target) != filepath.Clean(expectedTarget) {
		t.Fatalf("unexpected symlink target got=%q want=%q", target, expectedTarget)
	}
}
