package skill

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cyphant/zettelbrief/internal/config"
)

const (
	SkillName   = "zettelbrief"
	SkillFile   = "SKILL.md"
	Description = "Use when working in a code repository and you need Obsidian-backed project context from zettelbrief before planning, implementing, debugging, reviewing, or continuing work."
)

type Scope string

const (
	ScopeUser    Scope = "user"
	ScopeProject Scope = "project"
)

type Target string

const (
	TargetDefault Target = "default"
	TargetClaude  Target = "claude"
)

type CreateOptions struct {
	Scope       Scope
	Targets     []Target
	Force       bool
	CWD         string
	HomeDir     string
	ResolveHome func() (string, error)
}

type Result struct {
	Paths []string
}

type ResolvedTarget struct {
	Target Target
	Path   string
}

type Hints struct {
	ProjectHint      string
	RepoHint         string
	GitRoot          string
	MainWorktreeHint string
}

type resolvedContext struct {
	cwd          string
	home         string
	projectRoot  string
	gitRoot      string
	mainWorktree string
	hints        Hints
}

func Create(opts CreateOptions) (Result, error) {
	ctx, targets, err := ResolveTargets(opts)
	if err != nil {
		return Result{}, err
	}
	content, err := Render(RenderOptions{Scope: opts.Scope, Hints: ctx.hints})
	if err != nil {
		return Result{}, err
	}
	if err := Preflight(targets, opts.Force); err != nil {
		return Result{}, err
	}
	paths := make([]string, 0, len(targets))
	for _, target := range targets {
		if err := writeSkill(target.Path, []byte(content)); err != nil {
			return Result{}, err
		}
		paths = append(paths, target.Path)
	}
	return Result{Paths: paths}, nil
}

func ResolveTargets(opts CreateOptions) (resolvedContext, []ResolvedTarget, error) {
	if opts.Scope != ScopeUser && opts.Scope != ScopeProject {
		return resolvedContext{}, nil, fmt.Errorf("exactly one scope is required (--user or --project)")
	}
	if len(opts.Targets) == 0 {
		return resolvedContext{}, nil, fmt.Errorf("at least one agent target is required (--default or --claude)")
	}
	cwd := opts.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return resolvedContext{}, nil, err
		}
	}
	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return resolvedContext{}, nil, err
	}
	ctx := resolvedContext{cwd: cwdAbs}
	var base string
	if opts.Scope == ScopeUser {
		home, err := resolveHome(opts)
		if err != nil {
			return resolvedContext{}, nil, err
		}
		ctx.home = home
		base = home
	} else {
		home, err := resolveHome(opts)
		if err != nil {
			return resolvedContext{}, nil, fmt.Errorf("home directory is required to validate project-scope skill creation: %w", err)
		}
		ctx.home = home
		gitRoot, inGit, err := gitOutput(cwdAbs, "rev-parse", "--show-toplevel")
		if err != nil {
			return resolvedContext{}, nil, err
		}
		if inGit {
			ctx.gitRoot = filepath.Clean(gitRoot)
			ctx.projectRoot = ctx.gitRoot
		} else {
			ctx.projectRoot = cwdAbs
		}
		if err := validateProjectRoot(ctx.projectRoot, home); err != nil {
			return resolvedContext{}, nil, err
		}
		if inGit {
			ctx.mainWorktree = detectMainWorktree(cwdAbs)
		}
		ctx.hints = DetectHints(ctx.cwd, ctx.projectRoot, ctx.gitRoot, ctx.mainWorktree)
		base = ctx.projectRoot
	}
	targets := make([]ResolvedTarget, 0, len(opts.Targets))
	seen := map[Target]bool{}
	for _, target := range opts.Targets {
		if seen[target] {
			continue
		}
		seen[target] = true
		switch target {
		case TargetDefault:
			targets = append(targets, ResolvedTarget{Target: target, Path: filepath.Join(base, ".agents", "skills", SkillName, SkillFile)})
		case TargetClaude:
			targets = append(targets, ResolvedTarget{Target: target, Path: filepath.Join(base, ".claude", "skills", SkillName, SkillFile)})
		default:
			return resolvedContext{}, nil, fmt.Errorf("unsupported agent target %q", target)
		}
	}
	return ctx, targets, nil
}

func resolveHome(opts CreateOptions) (string, error) {
	if opts.HomeDir != "" {
		return filepath.Abs(opts.HomeDir)
	}
	resolver := opts.ResolveHome
	if resolver == nil {
		resolver = os.UserHomeDir
	}
	home, err := resolver()
	if err != nil || strings.TrimSpace(home) == "" {
		if err == nil {
			err = errors.New("empty home directory")
		}
		return "", fmt.Errorf("home directory is required for user-scope skill creation: %w", err)
	}
	return filepath.Abs(home)
}

func validateProjectRoot(root, home string) error {
	root = filepath.Clean(root)
	if root == filepath.Dir(root) {
		return fmt.Errorf("project-scope skill creation refuses unsafe project root %s", root)
	}
	if home == "" {
		return nil
	}
	rootAbs, err := canonicalPath(root)
	if err != nil {
		return err
	}
	homeAbs, err := canonicalPath(home)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, homeAbs)
	if err != nil {
		return err
	}
	if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return fmt.Errorf("project-scope skill creation refuses unsafe project root %s", rootAbs)
	}
	return nil
}

func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs, nil
	}
	return resolved, nil
}

func Preflight(targets []ResolvedTarget, force bool) error {
	for _, target := range targets {
		skillDir := filepath.Dir(target.Path)
		for _, dir := range []string{filepath.Dir(filepath.Dir(skillDir)), filepath.Dir(skillDir), skillDir} {
			if err := preflightDirectory(dir); err != nil {
				return err
			}
		}
		if info, err := os.Lstat(target.Path); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("unsafe skill path %s: SKILL.md is a symlink", target.Path)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("unsafe skill path %s: SKILL.md is not a regular file", target.Path)
			}
			if !force {
				return fmt.Errorf("skill %s already exists (use --force to overwrite)", target.Path)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func preflightDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe skill path %s: directory is a symlink", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("unsafe skill path %s: path is not a directory", path)
	}
	return nil
}

func writeSkill(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	_ = os.Chmod(filepath.Dir(filepath.Dir(dir)), 0o700)
	_ = os.Chmod(filepath.Dir(dir), 0o700)
	_ = os.Chmod(dir, 0o700)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	_ = os.Chmod(path, 0o600)
	return nil
}

type RenderOptions struct {
	Scope Scope
	Hints Hints
}

func Render(opts RenderOptions) (string, error) {
	if len(Description) > 1024 {
		return "", fmt.Errorf("skill description exceeds 1024 characters")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "---\nname: %s\ndescription: %s\n---\n\n", SkillName, Description)
	b.WriteString("# zettelbrief\n\n")
	b.WriteString("Use zettelbrief before planning, implementing, debugging, reviewing, or continuing work when local project history, decisions, meetings, or prior attempts may matter.\n\n")
	if opts.Scope == ScopeProject && opts.Hints.hasAny() {
		b.WriteString("## Detected Context Hints\n\n")
		b.WriteString("These hints are non-authoritative. Verify them against the user's task, repository, and working directory before using them. Ask the user if anything is missing or ambiguous.\n\n")
		b.WriteString("```yaml\n")
		writeYAMLHint(&b, "project_hint", opts.Hints.ProjectHint)
		writeYAMLHint(&b, "repo_hint", opts.Hints.RepoHint)
		writeYAMLHint(&b, "git_root", opts.Hints.GitRoot)
		writeYAMLHint(&b, "main_worktree_hint", opts.Hints.MainWorktreeHint)
		b.WriteString("```\n\n")
	}
	b.WriteString("## Workflow\n\n")
	b.WriteString("1. Determine the zettelbrief project, repository, and query context from the user's task, current directory, git remote/root names, branch names, and any detected hints. Treat detected hints as clues, not facts.\n")
	b.WriteString("2. If no project can be determined, or multiple projects are plausible, ask the user which zettelbrief project to use before running zettelbrief.\n")
	b.WriteString("3. For broad project context, run `zettelbrief fetch --project <project> \"<task>\"`.\n")
	b.WriteString("4. Only for clearly repository-specific code or architecture tasks with a reliable repository value, run `zettelbrief fetch --project <project> --repo <repo> \"<task>\"`.\n")
	b.WriteString("5. Omit `--repo` for broad project decisions, meeting context, project status, cross-repository history, or when no reliable repository value is known.\n")
	b.WriteString("6. Read the `brief.md` in the directory printed by `zettelbrief fetch`; use `sources.json` if you need source metadata.\n")
	b.WriteString("7. Treat the brief as cited context. Do not invent uncited project memory, and do not claim facts that are not supported by the brief or repository files.\n")
	b.WriteString("8. If results are too broad or too sparse, retry with a sharper query, add `--repo` for a reliable repo-specific task, or retry without `--repo` to recover broader project context.\n\n")
	b.WriteString("## Missing Or Stale Scan Data\n\n")
	b.WriteString("If zettelbrief reports missing database data, stale context, or no matching sources, tell the user that the project may need `zettelbrief scan --project <project>`. Do not run scan or fetch during skill creation, and do not write to the Obsidian vault.\n")
	return b.String(), nil
}

func (h Hints) hasAny() bool {
	return h.ProjectHint != "" || h.RepoHint != "" || h.GitRoot != "" || h.MainWorktreeHint != ""
}

func writeYAMLHint(b *strings.Builder, key, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(b, "%s: %q\n", key, value)
}

func DetectHints(cwd, projectRoot, gitRoot, mainWorktree string) Hints {
	hints := Hints{GitRoot: gitRoot, MainWorktreeHint: mainWorktree}
	if mainWorktree != "" {
		hints.RepoHint = filepath.Base(mainWorktree)
	} else if gitRoot != "" {
		hints.RepoHint = filepath.Base(gitRoot)
	}
	cfg, err := loadConfigForRoot(projectRoot)
	if err != nil || cfg == nil || len(cfg.Projects) == 0 {
		return hints
	}
	components := normalizedComponents(cwd, projectRoot, mainWorktree)
	matched := map[string]bool{}
	for name, project := range cfg.Projects {
		values := append([]string{name}, project.Aliases...)
		for _, value := range values {
			if components[normalize(value)] {
				matched[name] = true
			}
		}
	}
	if len(matched) == 1 {
		for name := range matched {
			hints.ProjectHint = name
		}
	}
	return hints
}

func loadConfigForRoot(root string) (*config.Config, error) {
	global, err := config.LoadGlobal("")
	if err != nil {
		return nil, err
	}
	projectPath, ok, err := config.DiscoverProjectConfig(root)
	if err != nil {
		return nil, err
	}
	if !ok {
		return global, nil
	}
	project, err := config.LoadProject(projectPath)
	if err != nil {
		return nil, err
	}
	return config.Merge(global, project), nil
}

func normalizedComponents(paths ...string) map[string]bool {
	components := map[string]bool{}
	for _, path := range paths {
		if path == "" {
			continue
		}
		clean := filepath.Clean(path)
		for {
			base := filepath.Base(clean)
			if n := normalize(base); n != "" {
				components[n] = true
			}
			parent := filepath.Dir(clean)
			if parent == clean {
				break
			}
			clean = parent
		}
	}
	return components
}

func normalize(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func gitOutput(cwd string, args ...string) (string, bool, error) {
	cmd := exec.Command("git", append([]string{"-C", cwd}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok || errors.Is(err, exec.ErrNotFound) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("running git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), true, nil
}

func detectMainWorktree(cwd string) string {
	out, ok, err := gitOutput(cwd, "worktree", "list", "--porcelain")
	if err != nil || !ok || out == "" {
		return ""
	}
	for line := range strings.SplitSeq(out, "\n") {
		if worktree, ok := strings.CutPrefix(line, "worktree "); ok {
			return strings.TrimSpace(worktree)
		}
	}
	return ""
}
