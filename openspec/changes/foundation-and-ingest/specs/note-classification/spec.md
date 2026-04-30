## ADDED Requirements

### Requirement: Normalize note paths before classification
The system SHALL classify notes using clean, slash-separated, vault-relative paths. Absolute filesystem paths SHALL be converted to vault-relative paths only after validation that the path remains inside `vault_path`.

#### Scenario: Absolute source path normalized
- **WHEN** the walker discovers `{vault_path}/1.Projects/VetZ/State.md`
- **THEN** the classifier receives `1.Projects/VetZ/State.md`

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
- **WHEN** a file at `1.Projects/VetZ/1. Daily Work/2026/04/2026-04-24.md` is processed
- **THEN** its note type is `daily_work`

#### Scenario: Granola meeting note classified
- **WHEN** a file at `4.Granola/2026-04/16/Daily Vetz-2026-04-16.md` is processed
- **THEN** its note type is `meeting`

#### Scenario: State document classified
- **WHEN** a file at `1.Projects/Flive/State.md` is processed
- **THEN** its note type is `project_state`

#### Scenario: General knowledge note classified (fallback)
- **WHEN** a file at `1.Projects/VetZ/Backend/architecture-overview.md` is processed
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
- **WHEN** frontmatter contains `tags: [vetz, backend]`
- **THEN** normalized tags are `['vetz', 'backend']`

#### Scenario: Tags as comma-separated string
- **WHEN** frontmatter contains `tags: "#vetz, #backend"`
- **THEN** normalized tags are `['vetz', 'backend']`

#### Scenario: Folders as scalar string
- **WHEN** frontmatter contains `folders: VetZ`
- **THEN** normalized folders are `['VetZ']`

### Requirement: Resolve project from note path or frontmatter
For notes under `1.Projects/`, the system SHALL extract the project name from the first path segment after `1.Projects/`. For Granola notes, the system SHALL use normalized frontmatter `folders:` values matched against configured project names and aliases.

#### Scenario: Project resolved from path
- **WHEN** a file at `1.Projects/VetZ/1. Daily Work/2026/04/2026-04-24.md` is processed
- **THEN** the project name is `VetZ`

#### Scenario: Project resolved from Granola frontmatter
- **WHEN** a Granola note at `4.Granola/2026-04/16/Daily Vetz-2026-04-16.md` has `folders: [VetZ]`
- **THEN** the project name is `VetZ`

#### Scenario: Project resolved from Granola alias
- **WHEN** a Granola note has `folders: [Vetz]` and config project `VetZ` has alias `Vetz`
- **THEN** the note is associated with project `VetZ`

#### Scenario: Multiple Granola folders map to multiple projects
- **WHEN** a Granola note has `folders: [VetZ, IReckonu]` and both projects are configured
- **THEN** the scan produces one logical note record for `VetZ` and one logical note record for `IReckonu`

#### Scenario: Ambiguous Granola folder alias
- **WHEN** a Granola folder value matches aliases for more than one configured project
- **THEN** the note is logged as ambiguous for that folder value and is not associated through that ambiguous value

### Requirement: Extract daily work metadata from bullet sections
For notes classified as `daily_work`, the system SHALL parse each `## Section` for structured bullet fields: `- Repo:`, `- Branch:`, `- Summary:`, `- Verification:`, `- Notes:`, and optional `- Commit:`, `- Ticket:`. Each `## Section` with a `- Repo:` field SHALL produce a separate logical note record with a stable `section_id` derived from the section heading index and slug.

#### Scenario: Daily work section with all fields
- **WHEN** a daily work note contains:
```
## One.Backend
- Repo: One.Backend
- Branch: feature/ff/lma
- Summary: Added vet_service_id to billable items
- Verification: dotnet test passed
```
- **THEN** the extracted record includes `repo: "One.Backend"`, `branch: "feature/ff/lma"`, `summary: "Added vet_service_id to billable items"`, `verification: "dotnet test passed"`, and a non-empty `section_id`

#### Scenario: Daily work section missing optional fields
- **WHEN** a daily work note section has `- Repo: One.Backend` but no `- Branch:` or `- Notes:` fields
- **THEN** the extracted record includes `repo: "One.Backend"` with NULL or empty values for missing optional fields (no error)

#### Scenario: Daily work section without Repo field
- **WHEN** a `## Section` in a daily work note has no `- Repo:` line
- **THEN** the section is logged as a warning and skipped (cannot determine repo)

#### Scenario: Daily work file has multiple repo sections
- **WHEN** one daily work file has `## One.Backend` and `## One.Frontend` sections with `- Repo:` fields
- **THEN** two logical note records are produced with the same `source_path` and different `section_id` values

#### Scenario: Daily work section is a follow-up
- **WHEN** a daily work note contains `### Follow-up` subsections
- **THEN** the subsection content is included in the overall note content but not parsed as a separate structured metadata record

### Requirement: Extract meeting metadata from frontmatter
For notes classified as `meeting`, the system SHALL extract metadata from the YAML frontmatter fields: `granola_id`, `title`, `created`, `updated`, `folders:`, and `type`. One logical note record SHALL be produced per matched project.

#### Scenario: Granola meeting with complete frontmatter
- **WHEN** a Granola note has:
```yaml
granola_id: aa93d0c8-6353-4e01-a5b6-5d16dfd6b95a
title: Daily Vetz
created: 2026-04-16T08:45:29.243Z
folders: [VetZ]
```
- **THEN** the extracted metadata includes `title: "Daily Vetz"`, `date: "2026-04-16"`, `granola_id: "aa93d0c8-6353-4e01-a5b6-5d16dfd6b95a"`, and `tags: ["granola"]`

#### Scenario: Granola meeting without optional fields
- **WHEN** a Granola note lacks `created` in frontmatter
- **THEN** `date` is derived from the filename date pattern if present, otherwise empty

### Requirement: Extract generic metadata from freeform notes
For notes classified as `knowledge` or `project_state`, the system SHALL extract the title from the first `# Heading` in the content and tags from normalized frontmatter `tags:` if present. Raw frontmatter `type` values such as `decision` or `plan` SHALL be preserved in `metadata_json` for future routing while the v1 `type` remains `knowledge` unless another classification rule applies.

#### Scenario: Knowledge note with heading
- **WHEN** a knowledge note's first line is `# One.Backend Architecture Overview`
- **THEN** the title is `One.Backend Architecture Overview`

#### Scenario: Knowledge note with tags
- **WHEN** a knowledge note has frontmatter `tags: [vetz, backend]`
- **THEN** the tags are stored as `["vetz", "backend"]`

#### Scenario: Knowledge note with raw decision type
- **WHEN** a knowledge note has frontmatter `type: decision`
- **THEN** its note type remains `knowledge` and `metadata_json` preserves `raw_type: "decision"`

#### Scenario: Knowledge note without heading
- **WHEN** a knowledge note has no `#` heading
- **THEN** the title is the filename without extension

#### Scenario: State note extracts title from heading
- **WHEN** `State.md` has `# Flive — Project State`
- **THEN** the title is `Flive — Project State`
