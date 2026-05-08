package skill

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveTargets(t *testing.T) {
	t.Run("user targets", func(t *testing.T) {
		home := filepath.Join(t.TempDir(), "home")
		_, targets, err := ResolveTargets(CreateOptions{Scope: ScopeUser, Targets: []Target{TargetDefault, TargetClaude}, HomeDir: home})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{
			filepath.Join(home, ".agents", "skills", "zettelbrief", "SKILL.md"),
			filepath.Join(home, ".claude", "skills", "zettelbrief", "SKILL.md"),
		}
		assertTargetPaths(t, targets, want)
	})

	t.Run("user scope requires home", func(t *testing.T) {
		_, _, err := ResolveTargets(CreateOptions{Scope: ScopeUser, Targets: []Target{TargetDefault}, ResolveHome: func() (string, error) {
			return "", errors.New("no home")
		}})
		if err == nil || !strings.Contains(err.Error(), "home directory is required") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("project targets use git root from subdirectory", func(t *testing.T) {
		repo := initGitRepo(t, filepath.Join(t.TempDir(), "repo"))
		sub := filepath.Join(repo, "a", "b")
		mustMkdir(t, sub)
		_, targets, err := ResolveTargets(CreateOptions{Scope: ScopeProject, Targets: []Target{TargetDefault, TargetClaude}, CWD: sub, HomeDir: filepath.Join(t.TempDir(), "home")})
		if err != nil {
			t.Fatal(err)
		}
		repo = realPath(t, repo)
		assertTargetPaths(t, targets, []string{
			filepath.Join(repo, ".agents", "skills", "zettelbrief", "SKILL.md"),
			filepath.Join(repo, ".claude", "skills", "zettelbrief", "SKILL.md"),
		})
	})

	t.Run("project target outside git uses cwd", func(t *testing.T) {
		tmp := t.TempDir()
		cwd := filepath.Join(tmp, "project")
		mustMkdir(t, cwd)
		_, targets, err := ResolveTargets(CreateOptions{Scope: ScopeProject, Targets: []Target{TargetDefault}, CWD: cwd, HomeDir: filepath.Join(tmp, "home")})
		if err != nil {
			t.Fatal(err)
		}
		assertTargetPaths(t, targets, []string{filepath.Join(cwd, ".agents", "skills", "zettelbrief", "SKILL.md")})
	})

	t.Run("project target refuses home and ancestors", func(t *testing.T) {
		tmp := t.TempDir()
		home := filepath.Join(tmp, "home")
		mustMkdir(t, home)
		for _, root := range []string{home, tmp} {
			_, _, err := ResolveTargets(CreateOptions{Scope: ScopeProject, Targets: []Target{TargetDefault}, CWD: root, HomeDir: home})
			if err == nil || !strings.Contains(err.Error(), "refuses unsafe project root") {
				t.Fatalf("root=%s err=%v", root, err)
			}
		}
		if runtime.GOOS != "windows" {
			link := filepath.Join(tmp, "home-link")
			if err := os.Symlink(home, link); err != nil {
				t.Fatal(err)
			}
			_, _, err := ResolveTargets(CreateOptions{Scope: ScopeProject, Targets: []Target{TargetDefault}, CWD: link, HomeDir: home})
			if err == nil || !strings.Contains(err.Error(), "refuses unsafe project root") {
				t.Fatalf("symlinked home err=%v", err)
			}
		}
	})

	t.Run("project target in submodule uses submodule root", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping git submodule test in short mode")
		}
		tmp := t.TempDir()
		subRepo := initGitRepo(t, filepath.Join(tmp, "subrepo"))
		writeFile(t, filepath.Join(subRepo, "README.md"), "sub\n")
		git(t, subRepo, "add", ".")
		git(t, subRepo, "commit", "-m", "sub")
		super := initGitRepo(t, filepath.Join(tmp, "super"))
		git(t, super, "-c", "protocol.file.allow=always", "submodule", "add", subRepo, "modules/sub")
		subdir := filepath.Join(super, "modules", "sub", "nested")
		mustMkdir(t, subdir)
		_, targets, err := ResolveTargets(CreateOptions{Scope: ScopeProject, Targets: []Target{TargetDefault}, CWD: subdir, HomeDir: filepath.Join(tmp, "home")})
		if err != nil {
			t.Fatal(err)
		}
		assertTargetPaths(t, targets, []string{filepath.Join(realPath(t, filepath.Join(super, "modules", "sub")), ".agents", "skills", "zettelbrief", "SKILL.md")})
	})

	t.Run("project target in linked worktree writes linked root", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping git worktree test in short mode")
		}
		tmp := t.TempDir()
		main := initGitRepo(t, filepath.Join(tmp, "main-repo"))
		writeFile(t, filepath.Join(main, "README.md"), "main\n")
		git(t, main, "add", ".")
		git(t, main, "commit", "-m", "main")
		linked := filepath.Join(tmp, "linked-repo")
		git(t, main, "worktree", "add", linked)
		_, targets, err := ResolveTargets(CreateOptions{Scope: ScopeProject, Targets: []Target{TargetDefault}, CWD: linked, HomeDir: filepath.Join(tmp, "home")})
		if err != nil {
			t.Fatal(err)
		}
		assertTargetPaths(t, targets, []string{filepath.Join(realPath(t, linked), ".agents", "skills", "zettelbrief", "SKILL.md")})
	})
}

func TestPreflightAndWriteSafety(t *testing.T) {
	t.Run("existing directory without skill file is accepted", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, ".agents", "skills", "zettelbrief", "SKILL.md")
		mustMkdir(t, filepath.Dir(target))
		if err := Preflight([]ResolvedTarget{{Target: TargetDefault, Path: target}}, false); err != nil {
			t.Fatal(err)
		}
		if err := writeSkill(target, []byte("content")); err != nil {
			t.Fatal(err)
		}
		assertFileContent(t, target, "content")
	})

	t.Run("existing skill requires force and force preserves siblings", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, ".agents", "skills", "zettelbrief", "SKILL.md")
		sibling := filepath.Join(filepath.Dir(target), "notes.md")
		mustMkdir(t, filepath.Dir(target))
		writeFile(t, target, "old")
		if runtime.GOOS != "windows" {
			if err := os.Chmod(target, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		writeFile(t, sibling, "sibling")
		if err := Preflight([]ResolvedTarget{{Target: TargetDefault, Path: target}}, false); err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("err=%v", err)
		}
		if err := Preflight([]ResolvedTarget{{Target: TargetDefault, Path: target}}, true); err != nil {
			t.Fatal(err)
		}
		if err := writeSkill(target, []byte("new")); err != nil {
			t.Fatal(err)
		}
		assertFileContent(t, target, "new")
		if runtime.GOOS != "windows" {
			info, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != 0o600 {
				t.Fatalf("forced file mode=%#o, want 0600", got)
			}
		}
		assertFileContent(t, sibling, "sibling")
	})

	t.Run("unsafe path collisions are rejected", func(t *testing.T) {
		root := t.TempDir()
		nonDirTarget := filepath.Join(root, ".agents", "skills", "zettelbrief", "SKILL.md")
		mustMkdir(t, filepath.Dir(filepath.Dir(nonDirTarget)))
		writeFile(t, filepath.Dir(nonDirTarget), "not a dir")
		if err := Preflight([]ResolvedTarget{{Target: TargetDefault, Path: nonDirTarget}}, true); err == nil || !strings.Contains(err.Error(), "not a directory") {
			t.Fatalf("non-dir err=%v", err)
		}

		dirSkillTarget := filepath.Join(t.TempDir(), ".agents", "skills", "zettelbrief", "SKILL.md")
		mustMkdir(t, dirSkillTarget)
		if err := Preflight([]ResolvedTarget{{Target: TargetDefault, Path: dirSkillTarget}}, true); err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("skill dir err=%v", err)
		}

		if runtime.GOOS == "windows" {
			t.Skip("symlink checks require symlink support")
		}
		symlinkRoot := filepath.Join(t.TempDir(), "root")
		actual := filepath.Join(symlinkRoot, "actual")
		mustMkdir(t, actual)
		dirLink := filepath.Join(symlinkRoot, ".agents", "skills", "zettelbrief")
		mustMkdir(t, filepath.Dir(dirLink))
		if err := os.Symlink(actual, dirLink); err != nil {
			t.Fatal(err)
		}
		if err := Preflight([]ResolvedTarget{{Target: TargetDefault, Path: filepath.Join(dirLink, "SKILL.md")}}, true); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("dir symlink err=%v", err)
		}

		parentSymlinkRoot := t.TempDir()
		parentActual := filepath.Join(parentSymlinkRoot, "actual")
		mustMkdir(t, parentActual)
		agentsLink := filepath.Join(parentSymlinkRoot, ".agents")
		if err := os.Symlink(parentActual, agentsLink); err != nil {
			t.Fatal(err)
		}
		if err := Preflight([]ResolvedTarget{{Target: TargetDefault, Path: filepath.Join(agentsLink, "skills", "zettelbrief", "SKILL.md")}}, true); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("parent symlink err=%v", err)
		}

		fileRoot := t.TempDir()
		fileTarget := filepath.Join(fileRoot, ".agents", "skills", "zettelbrief", "SKILL.md")
		mustMkdir(t, filepath.Dir(fileTarget))
		linkedFile := filepath.Join(fileRoot, "linked.md")
		writeFile(t, linkedFile, "linked")
		if err := os.Symlink(linkedFile, fileTarget); err != nil {
			t.Fatal(err)
		}
		if err := Preflight([]ResolvedTarget{{Target: TargetDefault, Path: fileTarget}}, true); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("file symlink err=%v", err)
		}
		assertFileContent(t, linkedFile, "linked")
	})

	t.Run("multi-target failure writes nothing", func(t *testing.T) {
		home := filepath.Join(t.TempDir(), "home")
		existing := filepath.Join(home, ".agents", "skills", "zettelbrief", "SKILL.md")
		missing := filepath.Join(home, ".claude", "skills", "zettelbrief", "SKILL.md")
		mustMkdir(t, filepath.Dir(existing))
		writeFile(t, existing, "old")
		_, err := Create(CreateOptions{Scope: ScopeUser, Targets: []Target{TargetDefault, TargetClaude}, HomeDir: home})
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("err=%v", err)
		}
		assertFileContent(t, existing, "old")
		if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("missing target stat err=%v", err)
		}

		laterInvalidHome := filepath.Join(t.TempDir(), "home")
		first := filepath.Join(laterInvalidHome, ".agents", "skills", "zettelbrief", "SKILL.md")
		second := filepath.Join(laterInvalidHome, ".claude", "skills", "zettelbrief", "SKILL.md")
		mustMkdir(t, filepath.Dir(second))
		writeFile(t, second, "old")
		_, err = Create(CreateOptions{Scope: ScopeUser, Targets: []Target{TargetDefault, TargetClaude}, HomeDir: laterInvalidHome})
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("later invalid err=%v", err)
		}
		if _, err := os.Stat(first); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("first target stat err=%v", err)
		}
		assertFileContent(t, second, "old")
	})

	t.Run("multi-target force overwrites and creates", func(t *testing.T) {
		home := filepath.Join(t.TempDir(), "home")
		existing := filepath.Join(home, ".agents", "skills", "zettelbrief", "SKILL.md")
		missing := filepath.Join(home, ".claude", "skills", "zettelbrief", "SKILL.md")
		mustMkdir(t, filepath.Dir(existing))
		writeFile(t, existing, "old")
		result, err := Create(CreateOptions{Scope: ScopeUser, Targets: []Target{TargetDefault, TargetClaude}, HomeDir: home, Force: true})
		if err != nil {
			t.Fatal(err)
		}
		assertTargetPaths(t, pathsToTargets(result.Paths), []string{existing, missing})
		assertFileContains(t, existing, "name: zettelbrief")
		assertFileContains(t, missing, "name: zettelbrief")
	})

	t.Run("created files are private on posix", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("POSIX permissions do not apply on Windows")
		}
		root := t.TempDir()
		target := filepath.Join(root, ".agents", "skills", "zettelbrief", "SKILL.md")
		if err := writeSkill(target, []byte("content")); err != nil {
			t.Fatal(err)
		}
		for _, dir := range []string{filepath.Join(root, ".agents"), filepath.Join(root, ".agents", "skills"), filepath.Dir(target)} {
			info, err := os.Stat(dir)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != 0o700 {
				t.Fatalf("%s mode=%#o, want 0700", dir, got)
			}
		}
		info, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("file mode=%#o, want 0600", got)
		}
	})
}

func TestRenderSkillContent(t *testing.T) {
	content, err := Render(RenderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := "---\nname: zettelbrief\ndescription: " + Description + "\n---\n"
	if !strings.HasPrefix(content, wantPrefix) {
		t.Fatalf("frontmatter prefix mismatch:\n%s", content)
	}
	if len(Description) > 1024 {
		t.Fatalf("description length=%d", len(Description))
	}
	for _, snippet := range []string{
		"before planning, implementing, debugging, reviewing, or continuing work",
		"Determine the zettelbrief project, repository, and query context",
		"zettelbrief fetch --project <project> \"<task>\"",
		"zettelbrief fetch --project <project> --repo <repo> \"<task>\"",
		"Omit `--repo` for broad project decisions, meeting context, project status, cross-repository history",
		"retry with a sharper query, add `--repo`",
		"Read the `brief.md` in the directory printed",
		"Do not invent uncited project memory",
		"zettelbrief scan --project <project>",
		"do not write to the Obsidian vault",
	} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("content missing %q:\n%s", snippet, content)
		}
	}
	for _, unsupported := range []string{".pi/skills", "~/.pi/agent/skills"} {
		if strings.Contains(content, unsupported) {
			t.Fatalf("content contains unsupported install guidance %q", unsupported)
		}
	}
	if strings.Contains(content, "## Detected Context Hints") {
		t.Fatalf("user/default render should omit hints section")
	}

	withHints, err := Render(RenderOptions{Scope: ScopeProject, Hints: Hints{ProjectHint: "Acme", RepoHint: "one-backend", GitRoot: "/tmp/repo"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, snippet := range []string{"## Detected Context Hints", "non-authoritative", "```yaml", "project_hint: \"Acme\"", "repo_hint: \"one-backend\"", "git_root: \"/tmp/repo\""} {
		if !strings.Contains(withHints, snippet) {
			t.Fatalf("hints content missing %q:\n%s", snippet, withHints)
		}
	}
	if strings.Contains(withHints, "main_worktree_hint:") {
		t.Fatalf("empty hint rendered:\n%s", withHints)
	}
}

func TestDetectHints(t *testing.T) {
	t.Run("exact project match", func(t *testing.T) {
		root, home := configFixture(t, "projects:\n  Acme:\n    folders: []\n")
		t.Setenv("HOME", home)
		hints := DetectHints(filepath.Join(root, "Acme"), root, root, "")
		if hints.ProjectHint != "Acme" {
			t.Fatalf("project_hint=%q", hints.ProjectHint)
		}
	})

	t.Run("alias normalization and repeated same project", func(t *testing.T) {
		root, home := configFixture(t, "projects:\n  Flive:\n    folders: []\n    aliases: [F Live]\n")
		t.Setenv("HOME", home)
		hints := DetectHints(filepath.Join(root, "flive", "F Live"), root, root, "")
		if hints.ProjectHint != "Flive" {
			t.Fatalf("project_hint=%q", hints.ProjectHint)
		}
	})

	t.Run("whole component only and ambiguous matches omitted", func(t *testing.T) {
		root, home := configFixture(t, "projects:\n  Acme:\n    folders: []\n  Other:\n    folders: []\n")
		t.Setenv("HOME", home)
		hints := DetectHints(filepath.Join(root, "AcmeBackend"), root, root, "")
		if hints.ProjectHint != "" {
			t.Fatalf("substring produced project_hint=%q", hints.ProjectHint)
		}
		hints = DetectHints(filepath.Join(root, "Acme", "Other"), root, root, "")
		if hints.ProjectHint != "" {
			t.Fatalf("ambiguous project_hint=%q", hints.ProjectHint)
		}
	})

	t.Run("missing config and non-git omit hints", func(t *testing.T) {
		tmp := t.TempDir()
		home := filepath.Join(tmp, "home")
		mustMkdir(t, home)
		t.Setenv("HOME", home)
		hints := DetectHints(filepath.Join(tmp, "nogit"), filepath.Join(tmp, "nogit"), "", "")
		if hints.ProjectHint != "" || hints.RepoHint != "" || hints.GitRoot != "" || hints.MainWorktreeHint != "" {
			t.Fatalf("hints=%#v", hints)
		}
	})

	t.Run("project config discovered from root keeps subdirectory behavior stable", func(t *testing.T) {
		tmp := t.TempDir()
		home := filepath.Join(tmp, "home")
		mustMkdir(t, home)
		root := filepath.Join(tmp, "RepoProject")
		mustMkdir(t, filepath.Join(root, ".zettelbrief"))
		writeFile(t, filepath.Join(root, ".zettelbrief", "config.yaml"), "projects:\n  RepoProject:\n    folders: []\n")
		t.Setenv("HOME", home)
		hints := DetectHints(filepath.Join(root, "src"), root, root, "")
		if hints.ProjectHint != "RepoProject" {
			t.Fatalf("project_hint=%q", hints.ProjectHint)
		}
	})

	t.Run("repo hint prefers main worktree", func(t *testing.T) {
		tmp := t.TempDir()
		home := filepath.Join(tmp, "home")
		mustMkdir(t, home)
		t.Setenv("HOME", home)
		hints := DetectHints(filepath.Join(tmp, "linked"), filepath.Join(tmp, "linked"), filepath.Join(tmp, "linked"), filepath.Join(tmp, "main-repo"))
		if hints.RepoHint != "main-repo" || hints.MainWorktreeHint != filepath.Join(tmp, "main-repo") {
			t.Fatalf("hints=%#v", hints)
		}
	})
}

func assertTargetPaths(t *testing.T, targets []ResolvedTarget, want []string) {
	t.Helper()
	got := make([]string, len(targets))
	for i, target := range targets {
		got[i] = filepath.Clean(target.Path)
	}
	want = cleanPaths(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("paths=\n%s\nwant=\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func pathsToTargets(paths []string) []ResolvedTarget {
	targets := make([]ResolvedTarget, len(paths))
	for i, path := range paths {
		targets[i] = ResolvedTarget{Path: path}
	}
	return targets
}

func cleanPaths(paths []string) []string {
	out := make([]string, len(paths))
	for i, path := range paths {
		out[i] = filepath.Clean(path)
	}
	return out
}

func configFixture(t *testing.T, globalProjects string) (root, home string) {
	t.Helper()
	tmp := t.TempDir()
	home = filepath.Join(tmp, "home")
	cfgDir := filepath.Join(home, ".config", "zettelbrief")
	mustMkdir(t, cfgDir)
	writeFile(t, filepath.Join(cfgDir, "config.yaml"), globalProjects)
	root = filepath.Join(tmp, "repo")
	mustMkdir(t, root)
	return root, home
}

func initGitRepo(t *testing.T, path string) string {
	t.Helper()
	mustMkdir(t, path)
	git(t, path, "init")
	git(t, path, "config", "user.email", "test@example.com")
	git(t, path, "config", "user.name", "Test User")
	return path
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func realPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("%s=%q, want %q", path, data, want)
	}
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s missing %q:\n%s", path, want, data)
	}
}
