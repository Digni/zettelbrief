## MODIFIED Requirements

### Requirement: FTS5 full-text index
The system SHALL create a FTS5 virtual table `notes_fts` indexing `title`, `summary`, `content`, and `tags` columns from the `notes` table. Triggers SHALL keep the FTS5 index in sync on INSERT, DELETE, and updates to indexed text columns. Visibility-only updates such as `seen_in_scan_id` or `scanned_at` SHALL NOT delete and rebuild FTS rows; the preferred implementation SHALL narrow the update trigger to indexed columns (`title`, `summary`, `notes_text`, `content`, `tags`) or use an equivalent visibility side table.

#### Scenario: Inserted note is searchable via FTS5
- **WHEN** a note with content containing "billable service update persistence" is inserted
- **THEN** an FTS5 query for `billable service` returns that note

#### Scenario: Updated note content is re-indexed
- **WHEN** a note's content is updated with new text
- **THEN** FTS5 queries reflect the updated content immediately

#### Scenario: Deleted note is removed from FTS5
- **WHEN** a note row is deleted from the `notes` table
- **THEN** the corresponding FTS5 entry is removed

#### Scenario: Visibility-only update does not re-index FTS5
- **WHEN** a note row is marked seen by updating `seen_in_scan_id` and `scanned_at` only
- **THEN** the FTS5 entry remains queryable
- **AND** the FTS5 delete/insert update trigger is not invoked for that visibility-only update

### Requirement: Search result data supports scoring and citation
The store layer SHALL return enough data for the application layer to score, route, render, excerpt, and cite fetched notes, including note identifier, project, type, section ID, metadata fields, source path, content, any available FTS rank signal, and any available FTS snippet text needed for evidence excerpts.

#### Scenario: Search result includes citation fields
- **WHEN** a matching logical note row is returned by search
- **THEN** the result includes `source_path`
- **AND** the result includes `section_id`
- **AND** the result includes the note row identifier

#### Scenario: Search result includes routing fields
- **WHEN** a matching logical note row is returned by search
- **THEN** the result includes note type, repo, date, title, summary, notes text, tags, and content fields needed for section routing

#### Scenario: Search result includes snippet data when available
- **WHEN** a matching logical note row is returned by FTS search
- **THEN** the result includes an FTS snippet or enough data for the application layer to build the requested classification-specific excerpt

## ADDED Requirements

### Requirement: Storage supports incremental scan marking
The store layer SHALL support marking existing note rows from unchanged non-Granola project files as seen in the current scan run without rewriting note content or creating duplicate rows. Marking rows as seen SHALL update `seen_in_scan_id` and `scanned_at`, SHALL preserve row IDs and note content, and SHALL NOT cause FTS content reindexing. Mark-seen operations SHALL be project-scoped and SHALL NOT be used in a way that violates non-destructive scan date filter rules.

#### Scenario: Existing non-Granola source path marked seen
- **WHEN** a scan run determines that `1.Projects/Acme/a.md` is unchanged for project `Acme` during a scan without active date filters
- **THEN** storage marks existing rows for that project and source path as seen in the current scan run
- **AND** row IDs remain stable
- **AND** `scanned_at` is updated for the current scan
- **AND** FTS entries remain queryable without a content reindex

#### Scenario: Mark seen is project-scoped
- **WHEN** the same source path exists for two projects because of a multi-project meeting
- **AND** project `Acme` marks the source path seen
- **THEN** rows for other projects are not marked seen by that operation

#### Scenario: Active date filter does not delete out-of-slice rows
- **WHEN** a scan has active date filters
- **AND** an existing row for the project has a date outside the active slice
- **THEN** storage does not delete that row solely because it was outside the active date slice
- **AND** stale cleanup is skipped or constrained so out-of-slice rows are preserved

#### Scenario: Missing source path cannot hide stale rows
- **WHEN** a previously indexed source path is not discovered during the current successful non-date-sliced scan
- **THEN** it is not marked seen
- **AND** stale cleanup removes it for the scanned project

### Requirement: Storage exposes content hashes for discovered files
The store layer SHALL allow scan code to look up existing content hashes by project and source path so unchanged non-Granola files can be detected before reparsing. Hash lookup SHALL be scoped by project and SHALL NOT require reading note content from the database. Scan code SHALL bypass hash-skip behavior for Granola meeting files.

#### Scenario: Existing hash returned for source path
- **WHEN** notes for project `Acme` and source path `1.Projects/Acme/a.md` are already stored
- **THEN** hash lookup returns the stored content hash for that source path

#### Scenario: No hash for unseen source path
- **WHEN** no rows exist for project `Acme` and source path `1.Projects/Acme/new.md`
- **THEN** hash lookup reports that no stored hash exists

### Requirement: Store search exposes safe FTS snippets
The store layer SHALL use SQLite FTS5 snippet support to produce bounded excerpts for matching rows without interpolating raw user query text into SQL. Snippet token counts SHALL be bounded and selected by the caller or store default.

#### Scenario: Snippet query uses existing sanitized match expression
- **WHEN** a search query contains quotes, punctuation, or FTS metacharacters
- **THEN** snippet retrieval uses the sanitized FTS match expression
- **AND** the SQL statement remains parameterized

#### Scenario: Snippet output is bounded
- **WHEN** a matching meeting note has long content
- **THEN** the returned snippet is bounded to the configured token count
