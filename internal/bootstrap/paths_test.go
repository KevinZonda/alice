package bootstrap

import (
	"path/filepath"
	"testing"

	"github.com/Alice-space/alice/internal/config"
)

func TestResolveRuntimeStateRoot(t *testing.T) {
	home := t.TempDir()
	got := ResolveRuntimeStateRoot(home)
	want := filepath.Join(config.RunDirForAliceHome(home), "connector")
	if got != want {
		t.Fatalf("unexpected runtime state root, got=%q want=%q", got, want)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("runtime state root should be absolute, got=%q", got)
	}
}

func TestResolveConfigPath(t *testing.T) {
	if got := ResolveConfigPath(""); got != config.DefaultConfigPath() {
		t.Fatalf("empty config path should fallback to default config path, got=%q", got)
	}

	got := ResolveConfigPath("config.yaml")
	if !filepath.IsAbs(got) {
		t.Fatalf("relative config path should resolve absolute path, got=%q", got)
	}
	if filepath.Base(got) != "config.yaml" {
		t.Fatalf("unexpected resolved config base, got=%q", filepath.Base(got))
	}
}
