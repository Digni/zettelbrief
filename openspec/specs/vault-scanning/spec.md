# vault-scanning Specification

## Purpose
TBD - created by archiving change foundation-and-ingest. Update Purpose after archive.
## Requirements
### Requirement: Walk project folders safely
The system SHALL discover all Markdown (`.md`) files within configured project folders, recursively walking all subdirectories. Non-Markdown files SHALL be ignored. Walking SHALL use validated vault-relative folders, SHALL deduplicate files by canonical vault-relative path, and SHALL NOT follow symlinks that escape the vault root.

#### Scenario: Project folder contains daily work notes
- **WHEN** a project folder contains `1. Daily Work/2026/04/2026-04-24.md`
- **THEN** the file is discovered and included in the scan

#### Scenario: Project folder contains non-Markdown files
- **WHEN** a project folder contains `.DS_Store`, `.gitkeep`, or image files
- **THEN** those files are skipped silently

#### Scenario: Nested subdirectories
- **WHEN** a project folder contains nested paths like `Backend/services/tenant-catalog-service.md`
- **THEN** the file is discovered regardless of nesting depth

#### Scenario: Empty project folder
- **WHEN** a configured project folder exists but contains no `.md` files
- **THEN** the system completes the scan with zero notes discovered and prints a warning

#### Scenario: Overlapping project folders
- **WHEN** project `VetZ` is configured with both `1.Projects/VetZ` and `1.Projects/VetZ/Backend`
- **THEN** a file under `Backend` is parsed and counted once for that project scan

#### Scenario: Symlink escapes vault
- **WHEN** a walked directory contains a symlink to a path outside `vault_path`
- **THEN** the scanner does not follow the symlink and logs a warning without ingesting files outside the vault

### Requirement: Read and parse note files defensively
The system SHALL read Markdown files as UTF-8 text with a configurable/default maximum file size. Files that exceed the size limit or cannot be read due to iCloud/sync/filesystem errors SHALL be warned and skipped. Invalid frontmatter SHALL be warned and skipped for that file.

#### Scenario: File exceeds size limit
- **WHEN** a Markdown file is larger than the configured maximum note size
- **THEN** the file is skipped with a warning including its vault-relative path

#### Scenario: iCloud placeholder cannot be read
- **WHEN** a Markdown file exists but cannot be read because the file is unavailable or mid-sync
- **THEN** the file is skipped with a warning and the scan continues

#### Scenario: Invalid YAML frontmatter
- **WHEN** a Markdown file contains malformed YAML frontmatter
- **THEN** the file is skipped with a warning including its vault-relative path

### Requirement: Discover Granola meeting notes by project
The system SHALL scan `4.Granola/` for Markdown files whose normalized frontmatter `folders:` field matches a configured project name or alias. Granola notes with `folders:` that match no configured project SHALL be skipped for that project scan.

#### Scenario: Granola note matches scanned project
- **WHEN** scanning project `VetZ` and a Granola note at `4.Granola/2026-04/16/Daily Vetz-2026-04-16.md` has `folders: [VetZ]`
- **THEN** the note is discovered and processed as a meeting note for `VetZ`

#### Scenario: Granola note matches scanned project alias
- **WHEN** scanning project `VetZ`, config has alias `Vetz`, and a Granola note has `folders: [Vetz]`
- **THEN** the note is discovered and processed as a meeting note for canonical project `VetZ`

#### Scenario: Granola note matches multiple configured projects
- **WHEN** scanning all projects and a Granola note has `folders: [VetZ, IReckonu]`
- **THEN** the note is processed once for `VetZ` and once for `IReckonu`

#### Scenario: Granola note does not match scanned project
- **WHEN** scanning project `VetZ` and a Granola note has `folders: [Flive]`
- **THEN** the note is skipped for this scan

#### Scenario: Granola note has no folders field
- **WHEN** a Granola note lacks a `folders:` frontmatter field
- **THEN** the note is logged as a warning and skipped (cannot determine project association)

#### Scenario: Granola folder name does not match config or aliases
- **WHEN** a Granola note has `folders: [Vetz]` but no configured project name or alias matches it
- **THEN** the note is logged as a warning with the unmatched folder name and skipped

### Requirement: Scan command accepts project or all flag
The `scan` subcommand SHALL require either `--project <name>` or `--all`. The `--all` flag SHALL scan every configured project sequentially in sorted project-name order.

#### Scenario: Scan a specific project
- **WHEN** `zettelbrief scan --project VetZ` is run
- **THEN** only notes belonging to project `VetZ` are processed

#### Scenario: Scan all projects
- **WHEN** `zettelbrief scan --all` is run
- **THEN** all configured projects are scanned sequentially in deterministic sorted project-name order

#### Scenario: Missing required project flag
- **WHEN** `zettelbrief scan` is run without `--project` or `--all`
- **THEN** the system SHALL exit with a non-zero status and show usage

### Requirement: Scan is idempotent and removes stale rows
Re-scanning the same files SHALL update existing logical note records in place. The `scanned_at` timestamp SHALL be updated on every successful scan, even if no content changed. A successful full project re-scan SHALL remove database rows for files or sections that no longer exist in the vault for that project.

#### Scenario: Re-scan unchanged files
- **WHEN** a previously scanned project is scanned again with no file changes
- **THEN** all existing notes are updated with the current `scanned_at` timestamp
- **AND** no duplicate rows are created

#### Scenario: New file added between scans
- **WHEN** a new `.md` file is added to a project folder and the project is re-scanned
- **THEN** the new file is inserted as a new note; existing notes are updated in place

#### Scenario: File removed between scans
- **WHEN** a previously scanned `.md` file is removed from the project folder and the project is re-scanned successfully
- **THEN** the database rows for that file and project are removed

#### Scenario: Daily work section removed between scans
- **WHEN** a `## Section` with `- Repo:` is removed from a daily work note and the project is re-scanned successfully
- **THEN** the database row for that section's previous `(project, source_path, section_id)` key is removed

### Requirement: Scan summary output
After each project scan, the system SHALL print a summary containing files discovered, records inserted/updated, stale records removed, warnings count, and project name. The summary SHALL NOT include note content or full frontmatter values.

#### Scenario: Scan completes with warnings
- **WHEN** a project scan skips two unreadable files and completes
- **THEN** the summary reports the warning count and successful scan status without printing private note content

