package zettelbrief_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEndToEndScanStatusAndStale(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repo := filepath.Dir(file)
	tmp := t.TempDir()
	vault := filepath.Join(tmp, "vault")
	copyDir(t, filepath.Join(repo, "testdata", "vault"), vault)
	home := filepath.Join(tmp, "home")
	cfgDir := filepath.Join(home, ".config", "zettelbrief")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "vault_path: " + vault + "\nprojects:\n  VetZ:\n    folders:\n      - 1.Projects/VetZ\n      - 1.Projects/VetZ/Backend\n    aliases: [Vetz]\n  Flive:\n    folders: []\n"
	if err := os.MkdirAll(filepath.Join(vault, "1.Projects", "Flive"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg = strings.Replace(cfg, "folders: []", "folders:\n      - 1.Projects/Flive", 1)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(tmp, "zettelbrief")
	build := exec.Command("go", "build", "-o", bin, "./cmd/zettelbrief")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	out := runCLI(t, tmp, home, bin, "scan", "--project", "VetZ")
	if !strings.Contains(out, "Records inserted/updated: 5") {
		t.Fatalf("scan output = %s", out)
	}
	out = runCLI(t, tmp, home, bin, "status")
	if !strings.Contains(out, "VetZ") || !strings.Contains(out, "notes: 5") || !strings.Contains(out, "Flive") || !strings.Contains(out, "not yet scanned") {
		t.Fatalf("status output = %s", out)
	}
	if err := os.Remove(filepath.Join(vault, "1.Projects", "VetZ", "Backend", "architecture-overview.md")); err != nil {
		t.Fatal(err)
	}
	out = runCLI(t, tmp, home, bin, "scan", "--project", "VetZ")
	if !strings.Contains(out, "Stale records removed: 1") {
		t.Fatalf("stale output = %s", out)
	}
}

func runCLI(t *testing.T, cwd, home, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	if err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		to := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(to, data, 0o600)
	}); err != nil {
		t.Fatal(err)
	}
}
