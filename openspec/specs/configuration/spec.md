# configuration Specification

## Purpose
TBD - created by archiving change foundation-and-ingest. Update Purpose after archive.
## Requirements
### Requirement: Global configuration file
The system SHALL load configuration from `~/.config/zettelbrief/config.yaml` if it exists. The file MUST be valid YAML. If the file does not exist, the system SHALL start with empty defaults and treat all settings as optional until a command requires them.

#### Scenario: Global config file exists
- **WHEN** `~/.config/zettelbrief/config.yaml` is present and valid
- **THEN** the system loads `vault_path`, `projects`, and any other defined settings

#### Scenario: Global config file is missing
- **WHEN** `~/.config/zettelbrief/config.yaml` does not exist
- **THEN** the system starts with empty defaults and does not error during config loading

#### Scenario: Global config file is invalid YAML
- **WHEN** `~/.config/zettelbrief/config.yaml` contains malformed YAML
- **THEN** the system SHALL exit with a non-zero status and print a parse error including the file path

### Requirement: Per-project configuration discovery and override
The system SHALL discover the nearest `.zettelbrief/config.yaml` by walking upward from the current working directory. If found, the system SHALL merge it on top of the global config. Per-project config MAY add or override `projects` entries. Per-project config MUST NOT override `vault_path`.

#### Scenario: Per-project config exists in working directory
- **WHEN** `.zettelbrief/config.yaml` is present in the working directory
- **THEN** its values are merged on top of the global config

#### Scenario: Per-project config exists in an ancestor directory
- **WHEN** the command is run from a repository subdirectory and an ancestor contains `.zettelbrief/config.yaml`
- **THEN** the nearest ancestor project config is discovered and merged

#### Scenario: Per-project config overrides vault_path
- **WHEN** `.zettelbrief/config.yaml` contains a `vault_path` field
- **THEN** the system SHALL warn that `vault_path` can only be set in global config and ignore the override

### Requirement: Project folder mapping
Each configured project SHALL have a name and one or more vault-relative folder paths. A project MAY define aliases used only for matching external folder names such as Granola `folders:` frontmatter. The system SHALL use project folder mappings to discover notes belonging to a project.

#### Scenario: Project with single folder
- **WHEN** a project `Acme` is configured with `folders: ["1.Projects/Acme"]`
- **THEN** scanning that project walks `{vault_path}/1.Projects/Acme` and all subdirectories

#### Scenario: Project with multiple folders
- **WHEN** a project `Acme` is configured with `folders: ["1.Projects/Acme", "1.Projects/Acme/Backend"]`
- **THEN** scanning that project walks both paths and deduplicates discovered files by canonical vault-relative path before extraction/storage

#### Scenario: Project with aliases
- **WHEN** a project `Acme` is configured with `aliases: ["Acme"]`
- **THEN** Granola frontmatter `folders: [Acme]` is matched to canonical project `Acme`

#### Scenario: Unknown project requested
- **WHEN** `zettelbrief scan --project UnknownProject` is run
- **THEN** the system SHALL exit with a non-zero status and list the configured project names

### Requirement: Vault path configuration
The system SHALL resolve the Obsidian vault path from the `vault_path` config field. The path SHALL support `~` expansion for the user's home directory. Commands that scan the vault SHALL require `vault_path` to be set and to point to an existing directory.

#### Scenario: Valid vault path
- **WHEN** `vault_path` is set to a valid existing directory
- **THEN** the system uses that directory as the vault root

#### Scenario: Missing vault path
- **WHEN** `vault_path` is empty or not set and a vault-dependent command is run
- **THEN** the system SHALL exit with an error message instructing the user to set `vault_path` in `~/.config/zettelbrief/config.yaml`

#### Scenario: Non-existent vault path
- **WHEN** `vault_path` points to a path that does not exist and a vault-dependent command is run
- **THEN** the system SHALL exit with an error message including the configured path

### Requirement: Project folder path validation
Configured project folders SHALL be relative to `vault_path` and SHALL NOT escape the vault root after path cleaning or symlink resolution.

#### Scenario: Absolute project folder rejected
- **WHEN** a project folder is configured as `/tmp/notes`
- **THEN** config validation SHALL fail with an error that project folders must be vault-relative

#### Scenario: Parent traversal rejected
- **WHEN** a project folder is configured as `../Secrets`
- **THEN** config validation SHALL fail with an error that the folder escapes the vault root

#### Scenario: Symlink escaping vault rejected
- **WHEN** a configured folder is or contains a symlink that resolves outside `vault_path`
- **THEN** the scanner SHALL NOT follow it and SHALL warn or error without ingesting files outside the vault

### Requirement: Configuration schema
The configuration SHALL use the following YAML structure:

```yaml
vault_path: /path/to/obsidian/vault
projects:
  ProjectName:
    folders:
      - 1.Projects/ProjectName
    aliases:
      - Project Name In Granola
  AnotherProject:
    folders:
      - 1.Projects/AnotherProject
      - 1.Projects/AnotherProject/Subfolder
```

#### Scenario: Valid configuration structure
- **WHEN** a config file matches the defined schema
- **THEN** the system loads it without errors

#### Scenario: Projects section is empty
- **WHEN** `projects:` is present but contains no entries
- **THEN** the system loads the config successfully (scanning will simply find no projects)

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

