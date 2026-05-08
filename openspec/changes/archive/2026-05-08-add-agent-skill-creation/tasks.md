## 1. CLI Surface and Validation

- [x] 1.1 Add `zettelbrief skill` and `zettelbrief skill create` Cobra commands under the existing root command wiring.
- [x] 1.2 Add `--user`, `--project`, `--default`, `--claude`, and `--force` flags with validation for exactly one scope and at least one target.
- [x] 1.3 Add command tests for missing scope, conflicting scopes, missing target, multiple target selection, and absence of unsupported `--general` behavior.
- [x] 1.4 Add a help/discoverability test or e2e assertion that `zettelbrief skill create --help` lists `--user`, `--project`, `--default`, `--claude`, and `--force`.
- [x] 1.5 Verify with `go run ./cmd/zettelbrief skill create --help` and `go test ./...` that existing CLI behavior remains unchanged.

## 2. Target Resolution and File Safety

- [x] 2.1 Implement target resolution for user `.agents`, user Claude, project `.agents`, and project Claude skill paths.
- [x] 2.2 Resolve project scope to git root when available and current working directory otherwise, with refusal for `$HOME`, ancestors of `$HOME`, and filesystem root.
- [x] 2.3 Hard-fail user-scope skill creation when the home directory cannot be resolved.
- [x] 2.4 Implement all-target preflight so multi-target invocations write nothing if any selected target is invalid or would require `--force`.
- [x] 2.5 Implement safe skill writes that create parent directories, refuse existing `SKILL.md` unless `--force` is supplied, overwrite only `SKILL.md` with `--force`, preserve siblings, and use restrictive POSIX permissions where supported.
- [x] 2.6 Reject unsafe path collisions: non-directory `zettelbrief` path, symlink `zettelbrief` path, and symlink `SKILL.md` path, even with `--force`.
- [x] 2.7 Add filesystem tests with temporary HOME, git-root, subdirectory, non-git cwd, `$HOME` refusal, submodule root, linked worktree root, existing dir without `SKILL.md`, force-preserves-siblings, symlink refusal, and multi-target all-or-nothing cases.
- [x] 2.8 Verify with focused tests for the resolver/writer package plus `go test ./...`.

## 3. Context Hint Detection

- [x] 3.1 Implement project-scoped hint detection using global config merged with nearest project config discovered from the resolved project root.
- [x] 3.2 Implement project matching by normalizing configured project names/aliases and whole path components with case-folding plus removal of non-ASCII letters/digits; do not use substring matching.
- [x] 3.3 Implement repo hint detection from detectable main worktree basename when available, otherwise current git root basename, and omit repo hint outside git.
- [x] 3.4 Ensure ambiguous or missing project matches produce no project hint rather than guessing; multiple matches for the same configured project are not ambiguous.
- [x] 3.5 Add tests for exact project match, alias-with-spaces normalization, whole-component-only matching, ambiguous multi-project matches, repeated same-project matches, missing config, stable subdirectory behavior, non-git directories, and worktree fallback behavior.
- [x] 3.6 Verify with focused tests for the hint detection package plus `go test ./...`.

## 4. Generated Skill Content

- [x] 4.1 Create the zettelbrief `SKILL.md` template with exact Agent Skill frontmatter keys `name` and `description`, including a description under 1024 characters.
- [x] 4.2 Include instructions for when to use zettelbrief and how to determine project, repository, and query context.
- [x] 4.3 Include fetch workflow guidance for both broad project context (`zettelbrief fetch --project <project> "<task>"`) and clearly repo-specific context (`zettelbrief fetch --project <project> --repo <repo> "<task>"`).
- [x] 4.4 Include nuanced repo-filter guidance: omit `--repo` for broad decisions, meetings, project status, cross-repository history, or uncertainty; retry with or without `--repo` if results are too broad or too sparse.
- [x] 4.5 Include instructions to read the printed `brief.md`, treat it as cited context, and avoid inventing uncited project memory.
- [x] 4.6 Include missing-scan guidance for `zettelbrief scan --project <project>` without running scan or fetch during skill creation.
- [x] 4.7 Include strict ambiguity handling: ask the user when project context is missing or multiple projects are plausible.
- [x] 4.8 Render optional project-scoped detected context hints in a `## Detected Context Hints` section with a fenced YAML block containing only non-empty hint keys and marking hints non-authoritative.
- [x] 4.9 Add golden-content tests for exact frontmatter, description length, broad/repo-specific fetch workflow, repo-filter nuance, missing-scan guidance, ambiguity handling, detected-hints shape, absence of `.pi/skills` and `~/.pi/agent/skills` install guidance, and user-scoped omission of hints.
- [x] 4.10 Verify with focused tests for the generation package plus `go test ./...`.

## 5. End-to-End Verification and Documentation

- [x] 5.1 Extend end-to-end CLI tests to create user/project default and Claude skills, assert printed paths, and assert no `.zettelbrief/briefs` directory is created.
- [x] 5.2 Extend end-to-end CLI tests for git-subdirectory-to-root behavior and non-git-cwd behavior.
- [x] 5.3 Add README documentation for `zettelbrief skill create` usage, target paths, `--force`, and agent reload/restart expectations.
- [x] 5.4 Ensure `zettelbrief skill create --help` text documents target paths and required scope/target choices.
- [x] 5.5 Run `go test ./...` and ensure all packages pass.
- [x] 5.6 Run `openspec status --change add-agent-skill-creation` and confirm all artifacts remain complete before implementation review.
