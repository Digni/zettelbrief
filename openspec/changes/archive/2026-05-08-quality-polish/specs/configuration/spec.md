## ADDED Requirements

### Requirement: Validation errors are graceful and actionable
The system SHALL report configuration and scan-input validation failures with clear, actionable errors that identify the relevant project, folder, vault path, flag, or note path without printing private note content or full frontmatter values.

#### Scenario: Missing vault path explains how to configure it
- **WHEN** a vault-dependent command is run and `vault_path` is not configured
- **THEN** the command fails with a message instructing the user to set `vault_path` in the global config

#### Scenario: Missing vault directory includes configured path
- **WHEN** `vault_path` points to a directory that does not exist
- **THEN** the command fails with an error including the configured path
- **AND** the error does not include note content

#### Scenario: Empty project folder list is rejected
- **WHEN** project `Acme` is configured with no folders
- **AND** `zettelbrief scan --project Acme` is run
- **THEN** configuration validation fails from the project validation path before scanning starts
- **AND** the message says project `Acme` must define at least one folder

#### Scenario: Empty existing folder warns without private content
- **WHEN** a configured project folder exists but contains no Markdown files
- **THEN** scan completes with a warning naming the folder
- **AND** the warning does not include note content or frontmatter values

### Requirement: Malformed note metadata produces privacy-safe warnings
The system SHALL continue scanning other files when a note has malformed frontmatter, malformed supported date fields, or unsupported normalized metadata shapes, and SHALL emit privacy-safe warnings that identify the vault-relative source path and problem category.

#### Scenario: Malformed frontmatter warning is path-scoped
- **WHEN** a Markdown file has invalid YAML frontmatter
- **THEN** the file is skipped with a warning including its vault-relative path
- **AND** the warning does not print the full frontmatter body


#### Scenario: Malformed frontmatter date warning is path-scoped
- **WHEN** a Markdown file has a non-empty `created` or `date` frontmatter value that cannot be parsed as `YYYY-MM-DD` or RFC3339
- **THEN** the scan emits a warning including the vault-relative path and field name
- **AND** the warning does not print the full frontmatter body
- **AND** an active scan date filter does not silently treat the malformed value as an in-range date

#### Scenario: Unsupported tags shape warning is path-scoped
- **WHEN** a Markdown file has unsupported `tags` frontmatter shape
- **THEN** the scan emits a warning including the vault-relative path and field name
- **AND** the scan continues processing other files

#### Scenario: Unsupported Granola folders shape warning is path-scoped
- **WHEN** a Granola note has unsupported `folders` frontmatter shape
- **THEN** the scan emits a warning including the vault-relative path and field name
- **AND** the note is not associated with a project through that invalid field

#### Scenario: Date extractor reports parse status
- **WHEN** date extraction sees a non-empty unsupported date value
- **THEN** the extractor returns enough status for scan code to warn instead of silently treating the value as missing
