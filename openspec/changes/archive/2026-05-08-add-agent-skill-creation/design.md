## Context

zettelbrief's stated purpose is to give agents useful project context before planning or implementation by generating compact, cited context packs from Obsidian notes (`README.md:3-11`). The current manual workflow is to run `zettelbrief fetch --project <project> --repo <repo> "<query>"` and read `.zettelbrief/briefs/<timestamp>/brief.md` plus `sources.json` (`README.md:24-40`). The CLI currently wires top-level commands in one Cobra root (`cmd/zettelbrief/main.go:26-30`) and exposes fetch as a one-query command with required `--project` and optional `--repo` filters (`cmd/zettelbrief/main.go:124-153`, `internal/app/fetch.go:18-24`, `internal/app/fetch.go:57`).

Pi's skill documentation confirms that `.agents/skills/<name>/SKILL.md` is a shared global/project skill shape that Pi discovers, and that root `.md` files under `.agents/skills` are ignored while directories containing `SKILL.md` are discovered recursively (`docs/skills.md:24-39`, `docs/skills.md:92-105`). It also confirms Agent Skill frontmatter requires a lowercase kebab-case `name` and a `description` no longer than 1024 characters (`docs/skills.md:137-164`). The user explicitly chose: only `.agents` for the default target, require an explicit scope, and allow project-scoped generated skills to embed detected project/repo hints.

Existing config discovery already walks upward for `.zettelbrief/config.yaml` (`internal/config/config.go:73-101`), and config loading merges global config with the nearest project config (`internal/config/config.go:108-115`). zettelbrief does not currently have an explicit local-repo-to-Obsidian-project mapping beyond configured project names, folders, and aliases, so local project/repo inference must be heuristic and fail safe.

I ran `go test ./...` before drafting; all packages passed.

## Goals / Non-Goals

**Goals:**

- Add an explicit `zettelbrief skill create` command that installs a reusable zettelbrief Agent Skill.
- Support user and project scopes, but require the user to choose exactly one scope.
- Support `.agents` via `--default` and Claude via `--claude`; allow both target flags in one command.
- Generate valid Agent Skill directories named `zettelbrief` with `SKILL.md` content that tells agents how to determine project/repo, fetch a brief, read it, and ask the user when context is ambiguous.
- Preserve local privacy and filesystem safety by preflighting all selected targets, creating parent directories with restrictive permissions on POSIX, rejecting unsafe path collisions, and refusing to overwrite existing `SKILL.md` files unless explicitly forced.
- Keep detection heuristic-only for this change; no config schema expansion is required.

**Non-Goals:**

- No `.pi/skills` or `~/.pi/agent/skills` output.
- No `--general` alias.
- No MCP server, daemon, external agent API, or Obsidian writes.
- No database schema or retrieval/ranking changes.
- No explicit persistent repo mapping in `config.yaml`.
- No automatic scan or fetch execution during skill creation.

## Decisions

1. **Represent skill creation as a nested Cobra command.**
   - Decision: add `newSkillCommand()` to the root alongside existing commands (`cmd/zettelbrief/main.go:26-30`) and add `newSkillCreateCommand()` under it. This keeps command shape discoverable as `zettelbrief skill create` without changing existing `init`, `scan`, `status`, or `fetch` behavior.
   - Alternative considered: a top-level `create-skill` command. Rejected because the requested shape groups future skill-related actions naturally.
   - Verification checkpoint: `go run ./cmd/zettelbrief skill create --help` should list `--user`, `--project`, `--default`, `--claude`, and `--force`; `go test ./...` should still pass.

2. **Use explicit scope and target validation.**
   - Decision: require exactly one of `--user` or `--project`, and require at least one target flag among `--default` and `--claude`. `--default` selects the `.agents` target. Add `--force` for overwrite safety, following the existing config writer pattern that refuses existing files unless forced (`internal/config/config.go:48-70`). Do not add `--general`; the alias adds complexity without improving agent behavior.
   - Alternative considered: default to project scope and `.agents` target. Rejected because the user chose explicit scope and global skill writes are high-impact.
   - Verification checkpoint: CLI tests should cover missing scope, both scopes, missing target, multiple target selection, and existing target refusal without `--force`.

3. **Resolve target paths deterministically from scope and target.**
   - Decision: write these paths:
     - `--user --default`: `~/.agents/skills/zettelbrief/SKILL.md`
     - `--project --default`: `<project-root>/.agents/skills/zettelbrief/SKILL.md`
     - `--user --claude`: `~/.claude/skills/zettelbrief/SKILL.md`
     - `--project --claude`: `<project-root>/.claude/skills/zettelbrief/SKILL.md`
   - For project scope, use `git rev-parse --show-toplevel` when available; otherwise use the current working directory. Refuse project-scope writes if the resolved project root is the user's home directory, an ancestor of the home directory, or the filesystem root. This avoids accidental project skills in global-ish directories while still supporting non-git project folders below home. Submodules use their own git root. Linked worktrees write to the linked worktree root, not the main worktree, because that is the active project root for the command; the main worktree may still be used as a non-authoritative repo hint.
   - Alternative considered: write exactly to cwd for `--project`. Rejected because agents discover project-level skills from ancestor directories and a repository root is the stable project boundary.
   - Verification checkpoint: filesystem tests using temporary HOME and git/non-git directories should assert exact target paths, refusal at HOME/root, subdirectory-to-git-root behavior, and POSIX `0700`/`0600` best-effort permissions.

4. **Preflight all selected targets before writing anything.**
   - Decision: compute and validate every selected target before creating directories or writing files. If any selected target fails validation, write nothing. A selected target is invalid if the `zettelbrief` path exists as a non-directory or symlink, if `SKILL.md` exists as a symlink, if `SKILL.md` exists and `--force` is absent, or if any required user/project root cannot be resolved. Existing `zettelbrief/` directories without `SKILL.md` are valid. `--force` overwrites only `SKILL.md`; it does not delete or replace sibling files such as `references/` or `assets/`.
   - Alternative considered: write targets one by one and report partial success. Rejected because partial failure makes retries surprising and can require unnecessary `--force` on already-created targets.
   - Verification checkpoint: writer tests should cover existing file refusal, force overwriting only `SKILL.md`, preserving siblings, existing directory without `SKILL.md`, non-directory collisions, symlink refusal, and multi-target all-or-nothing preflight.

5. **Generate one exact Agent Skill template for both targets.**
   - Decision: generate one `zettelbrief` Agent Skill with this frontmatter key set and no target-specific additions:
     ```yaml
     ---
     name: zettelbrief
     description: Use when working in a code repository and you need Obsidian-backed project context from zettelbrief before planning, implementing, debugging, reviewing, or continuing work.
     ---
     ```
     The description is intentionally below the 1024-character Agent Skills cap. The body should include: when to use it; how to determine `project`, `repo`, and query; how to run `zettelbrief fetch`; how to open the printed `brief.md`; and strict instruction to ask the user when the project cannot be determined or multiple projects match. The skill should explain that `--repo` is a narrowing filter: use it for clearly repo-specific code/architecture tasks with a reliable repo value, omit it for broad decisions, meeting context, project status, or when unsure, and retry with/without it if results are too broad or too sparse.
   - Alternative considered: generate per-agent customized content. Rejected for this change because `.agents` and Claude use the same `SKILL.md` structure, and divergence would create needless maintenance risk.
   - Verification checkpoint: golden-content tests should assert exact frontmatter keys, description length, fetch command guidance, nuanced repo-filter guidance, scan-missing guidance, ambiguity/ask-user instruction, and absence of `.pi/skills` and `~/.pi/agent/skills` install guidance.

6. **Render project-scoped hints in a stable, machine-readable block.**
   - Decision: project-scoped output may include a `## Detected Context Hints` section containing a fenced YAML block with only non-empty keys from `project_hint`, `repo_hint`, `git_root`, and `main_worktree_hint`. The text immediately before the block must say the hints are non-authoritative and must be verified against the user's task. User-scoped output omits this section because it is not tied to one project.
   - Alternative considered: prose-only hints. Rejected because a fixed shape is easier for agents and tests to consume.
   - Verification checkpoint: generated-content tests should assert the exact section shape and that uncertain hints are omitted rather than rendered empty.

7. **Use a defined heuristic for project/repo hints only.**
   - Decision: for project-scoped hints, load global config merged with the nearest project config discovered from the resolved project root, not from the arbitrary invocation subdirectory. Build path components from cwd, resolved project root, and detected main worktree path if present. Normalize both path components and configured project names/aliases by Unicode simple case-folding and removing all characters except ASCII letters and digits. A project matches only when a normalized project name or alias equals a normalized whole path component; substrings do not match. If exactly one configured project matches, emit `project_hint`; if zero or more than one match, omit it. Multiple matches for the same project still count as one project. Determine `repo_hint` from the basename of the main worktree path when detectable; otherwise from the current git root basename; outside git, omit it.
   - Alternative considered: add `repos:` or `local_paths:` mappings to config. Rejected as out of scope until heuristic failure becomes common.
   - Verification checkpoint: helper tests should cover configured project name match, alias-with-spaces normalization, whole-component-only matching, ambiguous multi-project matches, repeated same-project matches, missing config, stable subdirectory behavior, non-git folders, and worktree basename fallback behavior.

8. **Keep skill creation side-effect limited to file creation.**
   - Decision: do not run `zettelbrief scan`, `zettelbrief fetch`, or any agent command during `skill create`. The generated skill explains what agents should run during future sessions. This keeps installation deterministic and avoids surprising vault/database reads.
   - Alternative considered: run a validation fetch immediately. Rejected because a user may install before scanning and because installation should not require a task query.
   - Verification checkpoint: e2e tests should ensure command output reports created paths only and no `.zettelbrief/briefs` directory is created by skill creation.

## Risks / Trade-offs

- **Heuristic project detection can be wrong** → Only embed hints in project-scoped skills, use exact whole-component matching, and explicitly instruct agents to ask the user if the project is missing or ambiguous.
- **Repo filtering can hide useful project-level notes** → Generated skill explains that `--repo` is optional and should be omitted for broad decisions, meetings, project status, or uncertainty.
- **Skill installation can overwrite customized local instructions** → Refuse to overwrite existing `SKILL.md` by default; with `--force`, overwrite only `SKILL.md` and preserve siblings.
- **Multi-target writes can leave partial state** → Preflight all targets before writing any files.
- **Different agents may interpret skills slightly differently** → Keep content in the Agent Skills standard shape and avoid agent-specific behavior except install path selection.
- **Worktree main-root detection varies by git version and layout** → Treat main worktree as a hint only; write project skills to the current worktree root and require agents to verify hints.
- **Global install has broad behavioral impact** → Require explicit `--user` and print the exact created path.
- **Agent may not load the new skill until restart/reload** → Documentation and command output should tell users to restart or reload their agent if the skill is not detected.

## Migration Plan

- No database or config migration is required.
- Existing users are unaffected until they run `zettelbrief skill create`.
- Rollback is manual deletion of the generated `zettelbrief/SKILL.md` file or directory at the printed target path.

## Open Questions

- Should a future change add explicit `repos:`/`local_paths:` project mappings if heuristic detection proves too noisy?
- Should a future change add additional agent-specific targets beyond `.agents` and Claude?

## Planning Verification

- [x] Every file/line reference was read directly by me (not from subagent summary alone)
- [x] I ran diagnostic commands myself for facts in the plan (`go test ./...`, `openspec list --json`, git root/worktree commands)
- [x] Each step has a verification checkpoint with concrete command and expected outcome
- [x] I searched for existing patterns before proposing new ones (`cmd/zettelbrief/main.go`, `internal/config/config.go`, e2e tests, existing OpenSpec artifacts, Pi skill docs)
- [x] I checked current filesystem state for counts, paths, and names (`openspec/specs`, `.pi/.codex/.opencode`, user skill directories)
- [x] Blast radius listed if shared code is touched (CLI command wiring, config loading, filesystem writes, generated skill content, tests)
- [x] Edge cases documented for every integration point and data transformation

## Pre-Mortem

1. Agents still fail to fetch useful briefs because generated skills guess the wrong project from a folder name. Mitigation: do not make hints authoritative; require asking the user when no single configured project/alias matches.
2. Users lose customized skill edits because rerunning `skill create --force` overwrites more than intended. Mitigation: force overwrites only `SKILL.md`, preserves siblings, and preflight rejects symlink/non-directory collisions.
3. Project-scoped skills are written in a global-ish folder such as `$HOME` and trigger too broadly. Mitigation: refuse project-scope writes when the resolved project root is `$HOME`, an ancestor of `$HOME`, or the filesystem root.
4. Generated skills cause sparse briefs by overusing `--repo`. Mitigation: explain repo filters as optional narrowing and recommend omitting them for broad decisions, meetings, status context, or uncertainty.
