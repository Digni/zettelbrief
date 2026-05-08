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
	cfg := "vault_path: " + vault + "\nprojects:\n  Acme:\n    folders:\n      - 1.Projects/Acme\n      - 1.Projects/Acme/Backend\n    aliases: [Acme]\n  Flive:\n    folders: []\n"
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
	help := runCLI(t, tmp, home, bin, "scan", "--help")
	if !strings.Contains(help, "--since") || !strings.Contains(help, "--until") {
		t.Fatalf("scan help = %s", help)
	}
	out := runCLI(t, tmp, home, bin, "scan", "--project", "Acme")
	if !strings.Contains(out, "Records inserted/updated: 5") {
		t.Fatalf("scan output = %s", out)
	}
	out = runCLI(t, tmp, home, bin, "status")
	if !strings.Contains(out, "Acme") || !strings.Contains(out, "notes: 5") || !strings.Contains(out, "Flive") || !strings.Contains(out, "not yet scanned") {
		t.Fatalf("status output = %s", out)
	}
	briefDir := strings.TrimSpace(runCLI(t, tmp, home, bin, "fetch", "--project", "Acme", "--repo", "One.Backend", "persistence"))
	if !strings.Contains(briefDir, filepath.Join(".zettelbrief", "briefs")) {
		t.Fatalf("fetch output = %s", briefDir)
	}
	briefData, err := os.ReadFile(filepath.Join(tmp, briefDir, "brief.md"))
	if err != nil {
		t.Fatal(err)
	}
	briefText := string(briefData)
	if !strings.Contains(briefText, "## Relevant Prior Work") || !strings.Contains(briefText, "1.Projects/Acme/Backend/architecture-overview.md") {
		t.Fatalf("brief.md = %s", briefText)
	}
	sourcesData, err := os.ReadFile(filepath.Join(tmp, briefDir, "sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	sourcesText := string(sourcesData)
	for _, field := range []string{"source_path", "row_id", "classification", "confidence", "match_reason", "excerpt", "score", "recency_factor"} {
		if !strings.Contains(sourcesText, field) {
			t.Fatalf("sources.json missing %s = %s", field, sourcesData)
		}
	}
	for _, args := range [][]string{{"fetch", "--project", "Acme"}, {"fetch", "persistence"}, {"fetch", "--project", "Acme", "--since", "bad-date", "persistence"}, {"fetch", "--project", "Acme", "--until", "bad-date", "persistence"}, {"fetch", "--project", "Acme", "--since", "2026-05-01", "--until", "2026-04-01", "persistence"}, {"fetch", "--project", "Acme", "--type", "unsupported", "persistence"}, {"scan", "--project", "Acme", "--since", "bad-date"}, {"scan", "--project", "Acme", "--since", "2026-05-01", "--until", "2026-04-01"}} {
		if out, err := runCLIErr(t, tmp, home, bin, args...); err == nil || strings.Contains(out, "Usage:") {
			t.Fatalf("expected clear failure without usage for %v: err=%v out=%s", args, err, out)
		}
	}
	if err := os.Remove(filepath.Join(vault, "1.Projects", "Acme", "Backend", "architecture-overview.md")); err != nil {
		t.Fatal(err)
	}
	out = runCLI(t, tmp, home, bin, "scan", "--project", "Acme")
	if !strings.Contains(out, "Stale records removed: 1") {
		t.Fatalf("stale output = %s", out)
	}
}

func TestEndToEndSkillCreate(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repo := filepath.Dir(file)
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	bin := filepath.Join(tmp, "zettelbrief")
	build := exec.Command("go", "build", "-o", bin, "./cmd/zettelbrief")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	userOut := runCLI(t, tmp, home, bin, "skill", "create", "--user", "--default", "--claude")
	for _, path := range []string{
		filepath.Join(home, ".agents", "skills", "zettelbrief", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "zettelbrief", "SKILL.md"),
	} {
		if !strings.Contains(userOut, path) {
			t.Fatalf("user output missing %s:\n%s", path, userOut)
		}
		assertExists(t, path)
	}
	assertNotExists(t, filepath.Join(tmp, ".zettelbrief", "briefs"))

	project := filepath.Join(tmp, "project")
	if err := os.MkdirAll(filepath.Join(project, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitE2E(t, project, "init")
	projectRoot := realPathE2E(t, project)
	projectOut := runCLI(t, filepath.Join(project, "sub"), home, bin, "skill", "create", "--project", "--default", "--claude")
	for _, path := range []string{
		filepath.Join(projectRoot, ".agents", "skills", "zettelbrief", "SKILL.md"),
		filepath.Join(projectRoot, ".claude", "skills", "zettelbrief", "SKILL.md"),
	} {
		if !strings.Contains(projectOut, path) {
			t.Fatalf("project output missing %s:\n%s", path, projectOut)
		}
		assertExists(t, path)
	}
	assertNotExists(t, filepath.Join(project, ".zettelbrief", "briefs"))

	nonGit := filepath.Join(tmp, "non-git")
	if err := os.MkdirAll(nonGit, 0o755); err != nil {
		t.Fatal(err)
	}
	nonGitOut := runCLI(t, nonGit, home, bin, "skill", "create", "--project", "--default")
	nonGitPath := filepath.Join(nonGit, ".agents", "skills", "zettelbrief", "SKILL.md")
	if !strings.Contains(nonGitOut, nonGitPath) {
		t.Fatalf("non-git output missing %s:\n%s", nonGitPath, nonGitOut)
	}
	assertExists(t, nonGitPath)
	assertNotExists(t, filepath.Join(nonGit, ".zettelbrief", "briefs"))
}

func runCLI(t *testing.T, cwd, home, name string, args ...string) string {
	t.Helper()
	out, err := runCLIErr(t, cwd, home, name, args...)
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return out
}

func runCLIErr(t *testing.T, cwd, home, name string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to not exist, stat err=%v", path, err)
	}
}

func gitE2E(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

func realPathE2E(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
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
