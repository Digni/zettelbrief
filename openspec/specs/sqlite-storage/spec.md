# sqlite-storage Specification

## Purpose
TBD - created by archiving change foundation-and-ingest. Update Purpose after archive.
## Requirements
### Requirement: SQLite database location and privacy
The system SHALL create and manage a SQLite database at `.zettelbrief/zettelbrief.db` relative to the repository root or current working directory used for the project. The `.zettelbrief/` directory SHALL be created if it does not exist. The runtime directory and database file SHALL use restrictive local permissions where supported because the database contains copied private note content.

#### Scenario: First scan creates database
- **WHEN** `zettelbrief scan --project Acme` is run for the first time
- **THEN** `.zettelbrief/zettelbrief.db` is created with the full schema
- **AND** `.zettelbrief/` is created with mode `0700` where supported
- **AND** the database file is created with mode `0600` where supported

#### Scenario: Subsequent scans reuse existing database
- **WHEN** `zettelbrief scan --project Acme` is run and `.zettelbrief/zettelbrief.db` already exists
- **THEN** the existing database is opened and used without re-creating the schema

### Requirement: Schema migration tracking
The database SHALL contain a `schema_migrations` table that records applied schema versions.

#### Scenario: Fresh database migrated
- **WHEN** a new database is opened
- **THEN** all Change 1 schema objects are created
- **AND** the applied migration version is recorded

#### Scenario: Existing database opened
- **WHEN** an existing database has already applied the current schema version
- **THEN** migration runs without duplicating schema objects or data

### Requirement: Scan run tracking
The database SHALL contain a `scan_runs` table with project, start time, completion time, status, error, files seen, and notes seen. A project scan SHALL only be considered fresh when its latest run has status `completed`.

#### Scenario: Completed scan recorded
- **WHEN** a project scan finishes successfully
- **THEN** a `scan_runs` row is recorded with `status: completed`, `completed_at`, `files_seen`, and `notes_seen`

#### Scenario: Failed scan recorded
- **WHEN** a project scan fails before completion
- **THEN** a `scan_runs` row is recorded with `status: failed` and an error summary
- **AND** status output does not treat that failed run as a fresh completed scan

### Requirement: Notes table schema
The database SHALL contain a `notes` table with columns: `id` (INTEGER PRIMARY KEY), `project` (TEXT NOT NULL), `type` (TEXT NOT NULL), `section_id` (TEXT NOT NULL), `repo` (TEXT), `branch` (TEXT), `date` (TEXT), `title` (TEXT), `summary` (TEXT), `verification` (TEXT), `notes_text` (TEXT), `commit_hash` (TEXT), `ticket` (TEXT), `granola_id` (TEXT), `updated_at` (TEXT), `tags` (TEXT), `source_path` (TEXT NOT NULL), `content` (TEXT NOT NULL), `content_hash` (TEXT NOT NULL), `metadata_json` (TEXT), `seen_in_scan_id` (INTEGER), and `scanned_at` (TEXT NOT NULL). The table SHALL enforce `UNIQUE(project, source_path, section_id)`.

#### Scenario: Valid note inserted
- **WHEN** a classified logical note record is inserted with all required fields
- **THEN** the row is stored and queryable by `project`, `type`, `source_path`, and `section_id`

#### Scenario: NULL fields allowed for optional metadata
- **WHEN** a knowledge note has no `repo` or `branch`
- **THEN** the note is inserted with NULL for those columns without error

#### Scenario: Multiple daily work sections from one file
- **WHEN** a daily work file contains two sections with `- Repo:` fields
- **THEN** two rows are stored with the same `project` and `source_path` and different `section_id` values

#### Scenario: Multi-project Granola note
- **WHEN** a Granola note has `folders: [Acme, IReckonu]`
- **THEN** two rows are stored with the same `source_path` and `section_id` but different `project` values

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

### Requirement: Upsert behavior
Inserting a logical note record with a `(project, source_path, section_id)` key that already exists in the database SHALL update the existing row rather than creating a duplicate. The implementation SHALL use SQLite `INSERT ... ON CONFLICT(project, source_path, section_id) DO UPDATE` semantics rather than `INSERT OR REPLACE`. The `scanned_at` timestamp and `seen_in_scan_id` SHALL be updated on every upsert.

#### Scenario: Existing note updated
- **WHEN** a note at `1.Projects/Acme/1. Daily Work/2026/04/2026-04-24.md` with section `001-one-backend` is scanned again
- **THEN** its row is updated with the latest metadata and content
- **AND** its row ID remains stable
- **AND** `scanned_at` reflects the current time

#### Scenario: New note inserted
- **WHEN** a note at a previously unseen `(project, source_path, section_id)` key is scanned
- **THEN** a new row is inserted without affecting other notes

### Requirement: Full project scan transaction and stale cleanup
A successful full project scan SHALL apply note upserts and stale-row cleanup in a transaction. After all currently discovered records for a project are upserted with the current `scan_run` ID, rows for that project not seen in the current successful scan SHALL be deleted from `notes`.

#### Scenario: Deleted vault file removed from database
- **WHEN** a file that was present in the previous successful `Acme` scan is deleted from the vault
- **AND** `zettelbrief scan --project Acme` completes successfully
- **THEN** rows for the deleted file are removed from `notes` and `notes_fts`

#### Scenario: Failed scan does not delete stale rows
- **WHEN** `zettelbrief scan --project Acme` fails midway through scanning
- **THEN** the transaction is rolled back
- **AND** existing notes from the previous successful scan remain queryable
- **AND** the failed scan is visible in `scan_runs`

### Requirement: SQLite connection behavior
The system SHALL configure SQLite for local CLI reliability by using parameterized SQL for all user/config/vault-derived values, a busy timeout, and WAL mode where supported.

#### Scenario: Concurrent command waits for lock
- **WHEN** `status` opens the database while a scan transaction is active
- **THEN** SQLite uses the configured busy timeout before failing with a clear database-lock error

#### Scenario: SQL values parameterized
- **WHEN** a note contains quotes, SQL metacharacters, or FTS metacharacters in title/content/tags
- **THEN** insert/update/status SQL uses parameters and stores the content without SQL injection or syntax errors

### Requirement: Structured status querying
The system SHALL support querying structured scan state including configured project name, total note count per project, count per note type, latest completed scan timestamp per project, and latest failed scan error if any. Status formatting SHALL happen in the CLI layer, not inside the store package.

#### Scenario: Status after scanning two projects
- **WHEN** `Acme` has been scanned with 50 notes and `Flive` has been scanned with 30 notes
- **THEN** status query returns structured data for `Acme: 50 notes` and `Flive: 30 notes` with their respective completed scan timestamps

#### Scenario: Status with un-scanned configured project
- **WHEN** `Acme` has been scanned but configured project `Flive` has never been scanned
- **THEN** status query includes `Acme` with its count and timestamp, and indicates `Flive` as not yet scanned

#### Scenario: Status with empty database
- **WHEN** no projects have been scanned
- **THEN** status query reports zero notes and indicates configured projects as not yet scanned

### Requirement: Safe FTS note retrieval
The store layer SHALL support searching notes through the existing FTS5 index using sanitized/tokenized user query terms and parameterized SQL values. The implementation SHALL NOT interpolate raw user query text into SQL or FTS `MATCH` expressions. Searchable text for a logical note SHALL be note-specific: daily-work rows use their section title and extracted metadata fields; meeting, project state, and knowledge rows use title, summary, content, notes text, and tags. Daily-work rows SHALL NOT match solely because another section in the same source file contains a query term.

#### Scenario: Quoted query text does not break search
- **WHEN** the user searches for `"billable" service:update - persistence`
- **THEN** the store builds a valid FTS5 query from sanitized tokens
- **AND** the query executes without SQL or FTS syntax errors
- **AND** matching notes can be returned

#### Scenario: Empty sanitized query is rejected
- **WHEN** the user query contains no searchable tokens after sanitization
- **THEN** the store returns a clear validation error
- **AND** no SQL query is executed with an empty raw `MATCH` expression

#### Scenario: User query is not raw SQL
- **WHEN** the user query contains SQL metacharacters or FTS metacharacters
- **THEN** those characters cannot alter the SQL statement structure
- **AND** metadata filter values are passed as SQL parameters

#### Scenario: Daily-work search stays section-specific
- **WHEN** one daily-work file contains two logical sections
- **AND** only the second section contains `billable persistence`
- **THEN** a search for `billable persistence` returns the second section
- **AND** the search does not return the first section solely because it shares the same source file content

### Requirement: Metadata-filtered note lookup
The store layer SHALL support retrieval of matching note rows filtered by project and optional repo, note type, and inclusive date bounds using the columns already present in the `notes` table. The repo filter SHALL match rows whose repo equals the requested repo or whose repo is empty/null; it SHALL exclude rows with a different non-empty repo. Date filters SHALL exclude rows with empty/null dates. Store search SHALL return at most 250 candidate rows after FTS and metadata filtering.

#### Scenario: Project filter is required by query API
- **WHEN** the fetch workflow calls the store search API without a project
- **THEN** the store returns a validation error
- **AND** no unbounded cross-project search is executed

#### Scenario: Optional metadata filters are combined
- **WHEN** the fetch workflow searches with project `Acme`, repo `One.Backend`, type `daily_work`, since `2026-04-01`, and until `2026-04-30`
- **THEN** the store returns only `daily_work` notes for project `Acme`
- **AND** every returned row has repo `One.Backend`
- **AND** every returned row has a date in the inclusive range `2026-04-01` through `2026-04-30`

#### Scenario: Repo filter allows project-level notes
- **WHEN** the fetch workflow searches with project `Acme` and repo `One.Backend` without a type filter
- **THEN** rows with repo `One.Frontend` are not returned
- **AND** matching rows with empty/null repo remain eligible

#### Scenario: Notes with null optional metadata remain eligible when filter absent
- **WHEN** matching knowledge notes have null repo or date values
- **AND** the corresponding repo or date filter is not supplied
- **THEN** those notes remain eligible for retrieval

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

