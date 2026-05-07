## MODIFIED Requirements

### Requirement: Scan is idempotent and removes stale rows
Re-scanning the same files SHALL update existing logical note records in place or mark unchanged logical note records as seen in the current scan without duplicating rows. The `scanned_at` timestamp SHALL reflect the latest scan that processed or marked the row. A successful full project re-scan without active date filters SHALL run stale cleanup and remove database rows for files or sections that no longer exist in the vault for that project. A date-sliced scan SHALL be non-destructive for rows outside the active date slice: it limits which discovered logical notes are processed, SHALL skip stale cleanup for that scan run, and SHALL NOT delete previously indexed rows solely because their dates are outside the active slice or missing.

#### Scenario: Re-scan unchanged files
- **WHEN** a previously scanned project is scanned again with no file changes
- **THEN** all existing notes are either updated or marked seen for the current scan
- **AND** no duplicate rows are created
- **AND** stale cleanup does not remove unchanged rows

#### Scenario: New file added between scans
- **WHEN** a new `.md` file is added to a project folder and the project is re-scanned
- **THEN** the new file is inserted as a new note; existing notes are updated in place or marked seen

#### Scenario: File removed between scans
- **WHEN** a previously scanned `.md` file is removed from the project folder and the project is re-scanned successfully
- **THEN** the database rows for that file and project are removed

#### Scenario: Daily work section removed between scans
- **WHEN** a `## Section` with `- Repo:` is removed from a daily work note and the project is re-scanned successfully
- **THEN** the database row for that section's previous `(project, source_path, section_id)` key is removed

#### Scenario: Date-sliced scan preserves existing out-of-slice rows
- **WHEN** a previous scan indexed Acme notes dated `2026-04-01` and `2026-05-01`
- **AND** `zettelbrief scan --project Acme --since 2026-05-01` completes successfully
- **THEN** the `2026-05-01` note is processed for the scan
- **AND** the existing `2026-04-01` note remains indexed unless its source file or section is removed during a later non-date-sliced full scan

## ADDED Requirements

### Requirement: Scan applies inclusive date slicing
The `scan` subcommand SHALL accept optional `--since YYYY-MM-DD` and `--until YYYY-MM-DD` flags. When either flag is supplied, scan SHALL only process and upsert logical notes whose extracted note date is within the inclusive range. Notes with empty or missing dates SHALL be excluded from writes while an active date filter is supplied. Excluding a note from writes because of an active date filter SHALL NOT by itself delete an existing row for that note; stale cleanup SHALL NOT run during date-sliced scans.

#### Scenario: Scan since date is inclusive
- **WHEN** notes dated `2026-04-01` and `2026-03-31` are discovered
- **AND** `zettelbrief scan --project Acme --since 2026-04-01` is run
- **THEN** the `2026-04-01` note is processed and stored
- **AND** a newly discovered `2026-03-31` note is not stored by that scan
- **AND** any existing `2026-03-31` row is not deleted solely by the active date filter

#### Scenario: Scan until date is inclusive
- **WHEN** notes dated `2026-04-30` and `2026-05-01` are discovered
- **AND** `zettelbrief scan --project Acme --until 2026-04-30` is run
- **THEN** the `2026-04-30` note is processed and stored
- **AND** a newly discovered `2026-05-01` note is not stored by that scan
- **AND** any existing `2026-05-01` row is not deleted solely by the active date filter

#### Scenario: Scan date range rejects invalid date
- **WHEN** `zettelbrief scan --project Acme --since not-a-date` is run
- **THEN** the command fails with a clear date validation error
- **AND** no stale cleanup is applied

#### Scenario: Scan date range rejects reversed bounds
- **WHEN** `zettelbrief scan --project Acme --since 2026-05-01 --until 2026-04-01` is run
- **THEN** the command fails with a clear date range validation error
- **AND** no stale cleanup is applied

#### Scenario: Active scan date filter excludes undated notes
- **WHEN** a matching project note has no extracted date
- **AND** `zettelbrief scan --project Acme --since 2026-04-01` is run
- **THEN** the undated note is not stored for that scan
- **AND** any existing row for that undated note is not deleted solely by the active date filter

### Requirement: Scan can skip unchanged files safely
After measuring full-scan behavior, the system MAY use incremental scan behavior to avoid reparsing unchanged non-Granola project files by comparing content hashes only when a checked-in benchmark corpus is measured over at least 5 warm-cache iterations, reports total file count, Granola file share, and changed/unchanged ratio, and either the representative full scan median exceeds 2 seconds or a 90%-unchanged/10%-changed run demonstrates at least a 25% wall-clock improvement. If the threshold is not met, the system SHALL keep full-scan behavior and document the measurement. When used, incremental behavior SHALL preserve scan run accounting, stale cleanup, date-filter safety, and FTS consistency. Granola meeting files under `4.Granola/` SHALL always be parsed because project association depends on current configuration aliases as well as file content.

#### Scenario: Unchanged non-Granola file is marked seen without duplicate rows
- **WHEN** a previously scanned non-Granola project file has the same content hash during a later scan without active date filters
- **THEN** its existing logical note rows are marked seen in the current scan
- **AND** no duplicate note rows are created
- **AND** `scanned_at` reflects the current scan
- **AND** FTS search results for those rows remain available without a content reindex

#### Scenario: Unchanged Granola file is still parsed
- **WHEN** a previously scanned Granola meeting file has the same content hash during a later scan
- **THEN** the file is still parsed against the current configuration aliases
- **AND** project associations reflect the current configuration

#### Scenario: Changed file is reparsed and reindexed
- **WHEN** a previously scanned file content hash changes
- **THEN** the file is read, parsed, upserted, and reflected in FTS search results

#### Scenario: Removed file is still cleaned up
- **WHEN** a file that was indexed in the previous scan is no longer discovered
- **AND** the next scan completes successfully
- **THEN** rows for that file are deleted from `notes` and `notes_fts`

#### Scenario: Date-filtered scan skips cleanup and unsafe mark-seen
- **WHEN** a previously scanned file has both in-range and out-of-range rows
- **AND** a scan with active date filters runs
- **THEN** incremental mark-seen is disabled or date-aware for that run
- **AND** stale cleanup is skipped for that run
- **AND** out-of-range rows remain indexed

#### Scenario: Failed incremental scan does not delete existing rows
- **WHEN** an incremental scan fails before completion
- **THEN** the transaction is rolled back
- **AND** notes from the previous successful scan remain queryable

#### Scenario: Granola alias removal is cleaned when former project is scanned
- **WHEN** a Granola file previously matched project `Beta` through an alias
- **AND** that alias is removed from config while file content is unchanged
- **WHEN** project `Beta` is scanned without active date filters or `zettelbrief scan --all` is run
- **THEN** the Granola file is reparsed against current aliases
- **AND** stale cleanup removes the old `Beta` rows if the file no longer maps to `Beta`
