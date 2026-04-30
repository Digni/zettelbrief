package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadGlobalMissingAndInvalidYAML(t *testing.T) {
	cfg, err := LoadGlobal(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("missing global should not error: %v", err)
	}
	if len(cfg.Projects) != 0 {
		t.Fatalf("expected empty projects")
	}
	bad := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(bad, []byte("vault_path: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = LoadGlobal(bad)
	if err == nil || !strings.Contains(err.Error(), bad) {
		t.Fatalf("expected parse error with path, got %v", err)
	}
}

func TestMergeDiscoveryValidationAndSortedNames(t *testing.T) {
	tmp := t.TempDir()
	vault := filepath.Join(tmp, "vault")
	if err := os.MkdirAll(filepath.Join(vault, "1.Projects", "VetZ"), 0o755); err != nil {
		t.Fatal(err)
	}
	global := &Config{VaultPath: vault, Projects: map[string]ProjectConfig{"VetZ": {Folders: []string{"1.Projects/VetZ"}}}}
	project := &Config{VaultPath: "/ignored", Projects: map[string]ProjectConfig{"Flive": {Folders: []string{"1.Projects/Flive"}, Aliases: []string{"F Live"}}}}
	if err := os.MkdirAll(filepath.Join(vault, "1.Projects", "Flive"), 0o755); err != nil {
		t.Fatal(err)
	}
	merged := Merge(global, project)
	if merged.VaultPath != vault || len(merged.Warnings) != 1 {
		t.Fatalf("vault override should be ignored with warning: %#v", merged)
	}
	if got := merged.SortedProjectNames(); !reflect.DeepEqual(got, []string{"Flive", "VetZ"}) {
		t.Fatalf("sorted names = %#v", got)
	}
	if err := merged.ValidateForScan(""); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := ValidateVaultRelativeFolder(vault, "/tmp/notes"); err == nil {
		t.Fatalf("absolute folder should fail")
	}
	if err := ValidateVaultRelativeFolder(vault, "../Secrets"); err == nil {
		t.Fatalf("traversal should fail")
	}
	cwd := filepath.Join(tmp, "repo", "sub")
	if err := os.MkdirAll(filepath.Join(tmp, "repo", ".zettelbrief"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(tmp, "repo", ".zettelbrief", "config.yaml")
	if err := os.WriteFile(want, []byte("projects: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, ok, err := DiscoverProjectConfig(cwd)
	if err != nil || !ok || got != want {
		t.Fatalf("discover got %q ok=%v err=%v, want %q", got, ok, err, want)
	}
}
