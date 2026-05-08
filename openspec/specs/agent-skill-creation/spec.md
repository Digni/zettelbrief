# agent-skill-creation Specification

## Purpose
TBD - created by syncing change add-agent-skill-creation. Update Purpose after archive.
## Requirements
### Requirement: Skill creation command validates explicit scope and target flags
The system SHALL provide a `zettelbrief skill create` command that requires exactly one scope flag and at least one agent target flag. The scope flags SHALL be `--user` and `--project`. The agent target flags SHALL be `--default` and `--claude`, where `--default` selects the `.agents` target.

#### Scenario: Missing scope is rejected
- **WHEN** `zettelbrief skill create --default` is run without `--user` or `--project`
- **THEN** the command fails with a clear validation error that exactly one scope is required
- **AND** no skill file is created

#### Scenario: Conflicting scopes are rejected
- **WHEN** `zettelbrief skill create --user --project --default` is run
- **THEN** the command fails with a clear validation error that exactly one scope is required
- **AND** no skill file is created

#### Scenario: Missing target is rejected
- **WHEN** `zettelbrief skill create --project` is run without `--default` or `--claude`
- **THEN** the command fails with a clear validation error that at least one agent target is required
- **AND** no skill file is created

#### Scenario: Multiple targets are created in one invocation
- **WHEN** `zettelbrief skill create --user --default --claude` is run
- **THEN** the command creates the user `.agents` zettelbrief skill
- **AND** the command creates the user Claude zettelbrief skill
- **AND** command output lists both created paths

#### Scenario: Skill create help is discoverable
- **WHEN** `zettelbrief skill create --help` is run
- **THEN** the help output lists `--user`, `--project`, `--default`, `--claude`, and `--force`
- **AND** the help output does not list `--general`

### Requirement: Skill creation writes to the selected skill locations
The system SHALL write generated skills as `zettelbrief/SKILL.md` directories under the selected user or project skill locations. For user scope, `.agents` output SHALL be written under `~/.agents/skills/` and Claude output SHALL be written under `~/.claude/skills/`. For project scope, `.agents` output SHALL be written under `<project-root>/.agents/skills/` and Claude output SHALL be written under `<project-root>/.claude/skills/`. The project root SHALL be the current git root when available and the current working directory otherwise. The system SHALL reject project-scope writes when the resolved project root is the user's home directory, an ancestor of the user's home directory, or the filesystem root.

#### Scenario: User default target writes to user agents directory
- **WHEN** `zettelbrief skill create --user --default` is run with `HOME=/tmp/home`
- **THEN** the generated skill file is written to `/tmp/home/.agents/skills/zettelbrief/SKILL.md`
- **AND** command output includes that path

#### Scenario: User Claude target writes to user Claude directory
- **WHEN** `zettelbrief skill create --user --claude` is run with `HOME=/tmp/home`
- **THEN** the generated skill file is written to `/tmp/home/.claude/skills/zettelbrief/SKILL.md`
- **AND** command output includes that path

#### Scenario: User scope requires home directory
- **WHEN** `zettelbrief skill create --user --default` is run and the user's home directory cannot be resolved
- **THEN** the command fails with a clear validation error explaining that the home directory is required for user-scope skill creation
- **AND** no skill file is created

#### Scenario: Project default target writes to git root agents directory
- **WHEN** `zettelbrief skill create --project --default` is run from a subdirectory of a git repository
- **THEN** the generated skill file is written to `<git-root>/.agents/skills/zettelbrief/SKILL.md`
- **AND** command output includes that path

#### Scenario: Project Claude target writes to git root Claude directory
- **WHEN** `zettelbrief skill create --project --claude` is run from a subdirectory of a git repository
- **THEN** the generated skill file is written to `<git-root>/.claude/skills/zettelbrief/SKILL.md`
- **AND** command output includes that path

#### Scenario: Project target outside git writes to current directory
- **WHEN** `zettelbrief skill create --project --default` is run from a directory below the user's home directory that is not inside a git repository
- **THEN** the generated skill file is written to `<cwd>/.agents/skills/zettelbrief/SKILL.md`
- **AND** command output includes that path

#### Scenario: Project target refuses home directory
- **WHEN** `zettelbrief skill create --project --default` is run from the user's home directory outside a git repository
- **THEN** the command fails with a clear validation error that project-scope skill creation refuses to write at the home directory
- **AND** no skill file is created

#### Scenario: Project target refuses filesystem root or home ancestor
- **WHEN** `zettelbrief skill create --project --default` would resolve the project root to the filesystem root or an ancestor of the user's home directory
- **THEN** the command fails with a clear validation error naming the unsafe project root
- **AND** no skill file is created

#### Scenario: Project target in submodule writes to submodule root
- **WHEN** `zettelbrief skill create --project --default` is run from a git submodule
- **THEN** the generated skill file is written under the submodule git root
- **AND** the command does not write the skill to the superproject root

#### Scenario: Project target in linked worktree writes to linked worktree root
- **WHEN** `zettelbrief skill create --project --default` is run from a linked git worktree
- **THEN** the generated skill file is written under the current linked worktree root
- **AND** the command does not write the skill to the main worktree root

### Requirement: Skill creation preflights selected targets and writes safely
The system SHALL validate every selected target before creating or modifying any selected target. If any selected target is invalid, the command SHALL fail without writing any selected target. The system SHALL NOT overwrite an existing generated `SKILL.md` file unless the user supplies `--force`. `--force` SHALL replace only `SKILL.md` and SHALL preserve sibling files and directories. The system SHALL reject unsafe path collisions and SHALL NOT follow symlinks for the final `zettelbrief` skill directory or `SKILL.md` path. On POSIX platforms, the system SHALL create parent skill directories with mode `0700` and write `SKILL.md` with mode `0600`; on platforms where these modes are unsupported, permission setting MAY be best-effort.

#### Scenario: Existing skill file is not overwritten by default
- **WHEN** the target `zettelbrief/SKILL.md` already exists
- **AND** `zettelbrief skill create --project --default` is run without `--force`
- **THEN** the command fails with a clear error naming the existing target
- **AND** the existing file contents are unchanged

#### Scenario: Existing skill file is overwritten with force
- **WHEN** the target `zettelbrief/SKILL.md` already exists
- **AND** `zettelbrief skill create --project --default --force` is run
- **THEN** the command replaces the existing `SKILL.md` with the generated zettelbrief skill
- **AND** command output includes the overwritten path

#### Scenario: Force preserves sibling files
- **WHEN** the target `zettelbrief/SKILL.md` already exists
- **AND** the target `zettelbrief/` directory contains sibling files or directories
- **AND** `zettelbrief skill create --project --default --force` is run
- **THEN** the command replaces only `SKILL.md`
- **AND** the sibling files and directories remain unchanged

#### Scenario: Existing skill directory without skill file is accepted
- **WHEN** the target `zettelbrief/` directory exists but `zettelbrief/SKILL.md` does not exist
- **AND** `zettelbrief skill create --project --default` is run without `--force`
- **THEN** the command writes `zettelbrief/SKILL.md`
- **AND** command output includes the created path

#### Scenario: Non-directory skill path is rejected
- **WHEN** the target `zettelbrief` path exists as a file instead of a directory
- **AND** `zettelbrief skill create --project --default --force` is run
- **THEN** the command fails with a clear unsafe path error
- **AND** the existing file is unchanged

#### Scenario: Symlink skill directory is rejected
- **WHEN** the target `zettelbrief` path exists as a symlink
- **AND** `zettelbrief skill create --project --default --force` is run
- **THEN** the command fails with a clear unsafe path error
- **AND** the symlink target is not modified

#### Scenario: Symlink skill file is rejected
- **WHEN** the target `zettelbrief/SKILL.md` path exists as a symlink
- **AND** `zettelbrief skill create --project --default --force` is run
- **THEN** the command fails with a clear unsafe path error
- **AND** the symlink target is not modified

#### Scenario: Multi-target failure writes nothing
- **WHEN** `zettelbrief skill create --user --default --claude` selects two targets
- **AND** one selected target is invalid or already exists without `--force`
- **THEN** the command fails before writing either target
- **AND** no selected target is created or modified

#### Scenario: Multi-target force permits mixed existing and new targets
- **WHEN** `zettelbrief skill create --user --default --claude --force` selects one existing `SKILL.md` target and one missing target
- **THEN** the command preflights both targets successfully
- **AND** the existing `SKILL.md` target is replaced
- **AND** the missing target is created

#### Scenario: Generated skill path is private on POSIX
- **WHEN** a skill target is created successfully on a POSIX platform
- **THEN** parent skill directories are created with mode `0700`
- **AND** `SKILL.md` is written with mode `0600`

### Requirement: Generated skill follows Agent Skill structure
The generated skill SHALL be a valid Agent Skill directory named `zettelbrief` with a `SKILL.md` file. The generated `SKILL.md` SHALL include exactly the required frontmatter keys `name` and `description`, with `name: zettelbrief`. The description SHALL be non-empty, SHALL be no longer than 1024 characters, and SHALL describe using zettelbrief for Obsidian-backed project context before planning, implementing, debugging, reviewing, or continuing work.

#### Scenario: Generated skill has required frontmatter
- **WHEN** a skill is generated successfully
- **THEN** `SKILL.md` begins with YAML frontmatter
- **AND** the frontmatter contains `name: zettelbrief`
- **AND** the frontmatter contains a non-empty `description`
- **AND** the frontmatter contains no required target-specific keys beyond `name` and `description`

#### Scenario: Generated description stays within Agent Skill limit
- **WHEN** a skill is generated successfully
- **THEN** the frontmatter `description` is no longer than 1024 characters
- **AND** the description includes trigger phrasing for using zettelbrief before planning, implementing, debugging, reviewing, or continuing work

#### Scenario: Generated skill is placed in a directory skill shape
- **WHEN** a skill is generated successfully
- **THEN** the parent directory name is `zettelbrief`
- **AND** the generated file name is `SKILL.md`
- **AND** no root `.md` skill file is created directly under `.agents/skills`

#### Scenario: Generated skill avoids unsupported target references
- **WHEN** a skill is generated for the default target
- **THEN** the generated content does not instruct users or agents to use `.pi/skills` as the install location
- **AND** the generated content does not instruct users or agents to use `~/.pi/agent/skills` as the install location

### Requirement: Generated skill teaches agents how to query zettelbrief
The generated skill SHALL instruct agents to use zettelbrief before planning, implementation, debugging, review, or continuation tasks where local project history may matter. The generated skill SHALL instruct agents to determine project, repository, and query context; run `zettelbrief fetch`; read the generated `brief.md`; and treat the result as cited context rather than uncited memory. The generated skill SHALL describe `--repo` as an optional narrowing filter rather than a mandatory filter whenever a repository value is known.

#### Scenario: Skill instructs fetch workflow
- **WHEN** a skill is generated successfully
- **THEN** `SKILL.md` includes an instruction to run `zettelbrief fetch --project <project> "<task>"` for broad project context
- **AND** `SKILL.md` includes an instruction to run `zettelbrief fetch --project <project> --repo <repo> "<task>"` only for clearly repository-specific tasks with a reliable repository value
- **AND** `SKILL.md` instructs the agent to read the `brief.md` in the directory printed by the command

#### Scenario: Skill explains when to omit repo filter
- **WHEN** a skill is generated successfully
- **THEN** `SKILL.md` instructs the agent to omit `--repo` when the task asks for broad project decisions, meeting context, project status, or cross-repository history
- **AND** `SKILL.md` instructs the agent to omit `--repo` when no reliable repository value is known
- **AND** `SKILL.md` allows retrying with or without `--repo` if the first brief is too broad or too sparse

#### Scenario: Skill instructs ambiguity handling
- **WHEN** a skill is generated successfully
- **THEN** `SKILL.md` instructs the agent to ask the user for the zettelbrief project if no project can be determined
- **AND** `SKILL.md` instructs the agent to ask the user when multiple projects are plausible
- **AND** `SKILL.md` instructs the agent not to invent project memory when zettelbrief returns no matching sources

#### Scenario: Skill explains missing scan data
- **WHEN** a skill is generated successfully
- **THEN** `SKILL.md` explains that a missing database or stale context may require `zettelbrief scan --project <project>`
- **AND** `SKILL.md` does not instruct the agent to write to the Obsidian vault

### Requirement: Project-scoped skill generation may embed detected context hints
When creating a project-scoped skill, the system SHALL attempt to detect non-authoritative context hints from current git, folder, and configured project context. The system SHALL load global zettelbrief config merged with the nearest `.zettelbrief/config.yaml` discovered from the resolved project root. For project matching, the system SHALL compare configured project names and aliases against whole path components from the current working directory, resolved project root, and detectable main worktree path. Both configured values and path components SHALL be normalized by simple case-folding and removing all characters except ASCII letters and digits. Substring matches SHALL NOT count. If exactly one configured project matches after normalization, the generated skill SHALL include `project_hint`; otherwise it SHALL omit `project_hint` and instruct the agent to ask the user. For repository matching, the system SHALL use the main worktree basename when detectable, otherwise the current git root basename, and SHALL omit `repo_hint` outside git.

#### Scenario: Project hint is embedded for a single configured name match
- **WHEN** `zettelbrief skill create --project --default` is run from a path whose normalized components match exactly one configured zettelbrief project name
- **THEN** the generated `SKILL.md` includes that value as `project_hint`
- **AND** the generated `SKILL.md` states that hints are non-authoritative and should be verified against the task context

#### Scenario: Project hint is embedded for a normalized alias match
- **WHEN** a configured project has alias `F Live`
- **AND** `zettelbrief skill create --project --default` is run from a path containing component `flive`
- **THEN** the generated `SKILL.md` includes the corresponding configured project as `project_hint`

#### Scenario: Substring project match is ignored
- **WHEN** a configured project is named `Acme`
- **AND** `zettelbrief skill create --project --default` is run from a path containing component `AcmeBackend` but no whole component matching `Acme`
- **THEN** the generated `SKILL.md` does not include `Acme` as `project_hint` solely because of that substring

#### Scenario: Ambiguous project hint is omitted
- **WHEN** normalized path components match multiple configured zettelbrief projects or aliases
- **AND** `zettelbrief skill create --project --default` is run
- **THEN** the generated `SKILL.md` does not choose a project hint
- **AND** the generated `SKILL.md` instructs the agent to ask the user for the project

#### Scenario: Multiple matches for same project are not ambiguous
- **WHEN** normalized path components match both a configured project name and an alias for the same project
- **AND** no other configured project matches
- **THEN** the generated `SKILL.md` includes that project as `project_hint`

#### Scenario: Repo hint comes from git context
- **WHEN** `zettelbrief skill create --project --default` is run inside a git repository
- **THEN** the generated `SKILL.md` includes a repository hint derived from the current git root or detectable main worktree location
- **AND** the generated `SKILL.md` instructs the agent to verify the repo hint before using it as `--repo`

#### Scenario: Repo hint prefers detectable main worktree basename
- **WHEN** `zettelbrief skill create --project --default` is run inside a linked worktree
- **AND** the main worktree path is detectable
- **THEN** the generated `SKILL.md` includes a `repo_hint` derived from the main worktree basename
- **AND** the generated `SKILL.md` still writes the skill file under the linked worktree root

#### Scenario: Non-git project omits repo hint
- **WHEN** `zettelbrief skill create --project --default` is run outside a git repository
- **THEN** the generated `SKILL.md` omits the repository hint
- **AND** the generated `SKILL.md` instructs the agent to ask the user or omit `--repo` when no reliable repository value exists

### Requirement: Detected context hints use a stable rendered shape
When project-scoped skill generation includes detected context hints, the generated skill SHALL render them in a `## Detected Context Hints` section with a fenced YAML block. The block SHALL include only non-empty hint keys from `project_hint`, `repo_hint`, `git_root`, and `main_worktree_hint`. User-scoped skill generation SHALL omit the detected context hints section.

#### Scenario: Project-scoped hints render as YAML block
- **WHEN** a project-scoped skill is generated with `project_hint` and `repo_hint`
- **THEN** `SKILL.md` contains a `## Detected Context Hints` section
- **AND** that section contains a fenced YAML block with `project_hint` and `repo_hint`
- **AND** the section states that hints are non-authoritative

#### Scenario: Empty hints are omitted
- **WHEN** a project-scoped skill is generated without a project hint
- **THEN** the fenced YAML block does not contain an empty `project_hint` key

#### Scenario: User-scoped skill omits context hints
- **WHEN** a user-scoped skill is generated successfully
- **THEN** `SKILL.md` does not contain a `## Detected Context Hints` section

### Requirement: Skill creation does not query notes during installation
The system SHALL limit `zettelbrief skill create` side effects to creating the selected skill files and directories. It MUST NOT run scan, fetch, write briefing outputs, modify the SQLite database, or write to the Obsidian vault during installation.

#### Scenario: Skill creation does not create a brief
- **WHEN** `zettelbrief skill create --project --default` succeeds
- **THEN** no `.zettelbrief/briefs/<timestamp>` directory is created by that command
- **AND** the command output lists generated skill paths rather than a brief output directory

#### Scenario: Skill creation does not require an existing scan database
- **WHEN** `zettelbrief skill create --project --default` is run before `.zettelbrief/zettelbrief.db` exists
- **THEN** the command can still create the selected skill file
- **AND** the generated skill explains how agents should handle missing scan data later
