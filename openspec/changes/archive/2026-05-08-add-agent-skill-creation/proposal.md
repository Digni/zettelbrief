## Why

Agents can only benefit from zettelbrief if they know when and how to invoke it before planning or implementation. A `zettelbrief skill create` command turns that workflow into an installable Agent Skill so supported agents can consistently fetch cited Obsidian-backed project context from the current repository.

## What Changes

- Add a `zettelbrief skill create` CLI command that writes a zettelbrief Agent Skill to explicit user or project scope.
- Require exactly one scope flag: `--user` writes to the user skill directory and `--project` writes under the current project tree.
- Support agent target flags where `--default` writes to the widely used `.agents/skills` location and `--claude` writes to Claude's skill location.
- Allow multiple agent target flags in one invocation, while requiring at least one target flag.
- Generated skills describe how agents should use zettelbrief: determine project/repo from git and folder context, ask the user if ambiguous, run `zettelbrief fetch`, read `brief.md`, and treat output as cited context rather than invented memory.
- Generated skills instruct agents to use `--repo` selectively for repository-specific tasks, and to omit it when seeking broader project decisions, meetings, or status context.
- Project-scoped skills may embed detected hints for the current project/repo while still instructing agents to verify and ask when uncertain.
- Skill creation preflights all selected targets, refuses unsafe path collisions, and does not overwrite existing `SKILL.md` files unless `--force` is supplied.
- Do not introduce `.pi` skill output, Obsidian writes, MCP/server behavior, config schema changes, or new note retrieval semantics.

## Capabilities

### New Capabilities
- `agent-skill-creation`: Creating installable Agent Skills that teach agents how to query zettelbrief from user or project contexts.

### Modified Capabilities
- None.

## Impact

- Affected CLI/API surface: `zettelbrief skill create` with `--user`, `--project`, `--default`, `--claude`, and `--force` flags.
- Affected code areas: Cobra command wiring in `cmd/zettelbrief/main.go`; new app/domain code for skill target resolution, project/repo hint detection, path preflight, and skill file generation; tests for CLI validation and generated content.
- Affected local filesystem paths: `~/.agents/skills/zettelbrief/SKILL.md`, `.agents/skills/zettelbrief/SKILL.md`, `~/.claude/skills/zettelbrief/SKILL.md`, and `.claude/skills/zettelbrief/SKILL.md`.
- No database schema, scan logic, fetch ranking, note classification, or Obsidian vault content changes are introduced.
