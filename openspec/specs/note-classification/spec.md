# note-classification Specification

## Purpose
TBD - created by archiving change foundation-and-ingest. Update Purpose after archive.
## Requirements
### Requirement: Normalize note paths before classification
The system SHALL classify notes using clean, slash-separated, vault-relative paths. Absolute filesystem paths SHALL be converted to vault-relative paths only after validation that the path remains inside `vault_path`.

#### Scenario: Absolute source path normalized
- **WHEN** the walker discovers `{vault_path}/1.Projects/Acme/State.md`
- **THEN** the classifier receives `1.Projects/Acme/State.md`

#### Scenario: Path outside vault rejected
- **WHEN** a discovered path does not resolve inside `vault_path`
- **THEN** the file is rejected and is not classified or stored

### Requirement: Classify note by path pattern
The system SHALL detect note type from the file's vault-relative path using the following priority-ordered rules:

1. Path contains `/1. Daily Work/` or starts with `1. Daily Work/` → `daily_work`
2. Path starts with `4.Granola/` → `meeting`
3. Filename equals `State.md` → `project_state`
4. All other paths → `knowledge`

#### Scenario: Daily work note classified
- **WHEN** a file at `1.Projects/Acme/1. Daily Work/2026/04/2026-04-24.md` is processed
- **THEN** its note type is `daily_work`

#### Scenario: Granola meeting note classified
- **WHEN** a file at `4.Granola/2026-04/16/Daily Acme-2026-04-16.md` is processed
- **THEN** its note type is `meeting`

#### Scenario: State document classified
- **WHEN** a file at `1.Projects/Flive/State.md` is processed
- **THEN** its note type is `project_state`

#### Scenario: General knowledge note classified (fallback)
- **WHEN** a file at `1.Projects/Acme/Backend/architecture-overview.md` is processed
- **THEN** its note type is `knowledge`

#### Scenario: Daily work not in a project folder
- **WHEN** a file outside `1.Projects/` has `/1. Daily Work/` in its vault-relative path
- **THEN** its note type is still `daily_work` (path pattern takes precedence over project context)

### Requirement: Refine classification from frontmatter
When a file has YAML frontmatter, the system SHALL use normalized frontmatter fields to refine or confirm the classification. The `tags` field containing `state` after tag normalization SHALL set the type to `project_state`, overriding path-based detection.

#### Scenario: State tag overrides path classification
- **WHEN** `State.md` has frontmatter `tags: [project/flive, state]`
- **THEN** its note type is confirmed as `project_state` (path-based detection already set this)

#### Scenario: State tag with hash prefix overrides path classification
- **WHEN** a note has frontmatter `tags: "#state, #project/flive"`
- **THEN** its note type is `project_state`

#### Scenario: Meeting note confirmed by frontmatter
- **WHEN** a Granola note has frontmatter `type: note` and `granola_id: abc123`
- **THEN** its note type is confirmed as `meeting`

### Requirement: Normalize frontmatter list fields
The system SHALL normalize `tags` and `folders` frontmatter values from common YAML shapes, including scalar strings, comma-separated strings, YAML sequences, and hash-prefixed tags. Unsupported shapes SHALL produce a warning and an empty normalized list for that field.

#### Scenario: Tags as YAML sequence
- **WHEN** frontmatter contains `tags: [acme, backend]`
- **THEN** normalized tags are `['acme', 'backend']`

#### Scenario: Tags as comma-separated string
- **WHEN** frontmatter contains `tags: "#acme, #backend"`
- **THEN** normalized tags are `['acme', 'backend']`

#### Scenario: Folders as scalar string
- **WHEN** frontmatter contains `folders: Acme`
- **THEN** normalized folders are `['Acme']`

### Requirement: Resolve project from note path or frontmatter
For notes under `1.Projects/`, the system SHALL extract the project name from the first path segment after `1.Projects/`. For Granola notes, the system SHALL use normalized frontmatter `folders:` values matched against configured project names and aliases.

#### Scenario: Project resolved from path
- **WHEN** a file at `1.Projects/Acme/1. Daily Work/2026/04/2026-04-24.md` is processed
- **THEN** the project name is `Acme`

#### Scenario: Project resolved from Granola frontmatter
- **WHEN** a Granola note at `4.Granola/2026-04/16/Daily Acme-2026-04-16.md` has `folders: [Acme]`
- **THEN** the project name is `Acme`

#### Scenario: Project resolved from Granola alias
- **WHEN** a Granola note has `folders: [Acme]` and config project `Acme` has alias `Acme`
- **THEN** the note is associated with project `Acme`

#### Scenario: Multiple Granola folders map to multiple projects
- **WHEN** a Granola note has `folders: [Acme, IReckonu]` and both projects are configured
- **THEN** the scan produces one logical note record for `Acme` and one logical note record for `IReckonu`

#### Scenario: Ambiguous Granola folder alias
- **WHEN** a Granola folder value matches aliases for more than one configured project
- **THEN** the note is logged as ambiguous for that folder value and is not associated through that ambiguous value

### Requirement: Extract daily work metadata from bullet sections
For notes classified as `daily_work`, the system SHALL parse each `## Section` for structured bullet fields: `- Repo:`, `- Branch:`, `- Summary:`, `- Verification:`, `- Notes:`, and optional `- Commit:`, `- Ticket:`. Each `## Section` with a `- Repo:` field SHALL produce a separate logical note record with a stable `section_id` derived from the section heading index and slug.

#### Scenario: Daily work section with all fields
- **WHEN** a daily work note contains:
```

