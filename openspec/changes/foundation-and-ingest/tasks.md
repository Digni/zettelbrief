## 1. Project scaffold

- [x] 1.1 Initialize Go module (`go mod init github.com/cyphant/zettelbrief`) and add Cobra, YAML, and `modernc.org/sqlite` dependencies
- [x] 1.2 Create project directory layout: `cmd/zettelbrief/`, `internal/app/`, `internal/config/`, `internal/scan/`, `internal/models/`, `internal/store/`
- [x] 1.3 Update `.gitignore` to ignore runtime `.zettelbrief` artifacts (`zettelbrief.db*`, `briefs/`) while allowing optional `.zettelbrief/config.yaml`
- [x] 1.4 Verify `go build ./...` succeeds on empty scaffold

## 2. Models (internal/models)

- [x] 2.1 Define `NoteType` as string enum: `daily_work`, `meeting`, `knowledge`, `project_state`
- [x] 2.2 Define `Note` struct matching logical SQLite note schema, including `Project`, `Type`, `SectionID`, `Repo`, `Branch`, `Date`, `Title`, `Summary`, `Verification`, `NotesText`, `CommitHash`, `Ticket`, `GranolaID`, `UpdatedAt`, `Tags`, `SourcePath`, `Content`, `ContentHash`, `MetadataJSON`, `SeenInScanID`, `ScannedAt`
- [x] 2.3 Define `NoteMetadata` / extraction structs for type-specific metadata and daily-work section IDs
- [x] 2.4 Define `ScanRun` and structured `ProjectStatus` models
- [x] 2.5 Add JSON tags and serialization helpers for `Tags` and `MetadataJSON`

## 3. Configuration (internal/config)

- [x] 3.1 Define `Config` struct with `VaultPath` string and `Projects` map[string]ProjectConfig (`Folders []string`, `Aliases []string`)
- [x] 3.2 Implement `LoadGlobal(path string) (*Config, error)` — parse YAML from `~/.config/zettelbrief/config.yaml`, expand `~` in paths
- [x] 3.3 Implement upward discovery for nearest `.zettelbrief/config.yaml` from the current working directory
- [x] 3.4 Implement `LoadProject(path string) (*Config, error)` — parse discovered project config
- [x] 3.5 Implement `Merge` — project config adds/overrides project entries; warn and ignore if project config sets `vault_path`
- [x] 3.6 Implement `Load()` — load global then merge discovered project config, return final config
- [x] 3.7 Validate command-required config: `vault_path` must be set and exist for scan operations; project folders must be vault-relative, non-empty, no `..`, and resolve inside the vault; symlinks escaping vault are rejected/not followed
- [x] 3.8 Implement deterministic project ordering by sorted project names for `--all`
- [x] 3.9 Write unit tests: valid config, missing global file, invalid YAML, merge behavior, vault_path override warning, upward discovery, invalid absolute folder, `..` traversal, aliases, sorted project names

## 4. SQLite storage (internal/store)

- [x] 4.1 Implement `Open(dbPath string) (*sql.DB, error)` — create `.zettelbrief/` directory if needed with restrictive permissions, open SQLite with `modernc.org/sqlite`, set WAL/busy timeout where supported
- [x] 4.2 Implement migrations with `schema_migrations`
- [x] 4.3 Implement Change 1 migration — create `scan_runs`, `notes` table with `UNIQUE(project, source_path, section_id)`, `notes_fts`, and FTS insert/update/delete triggers
- [x] 4.4 Implement scan run operations: start run, complete run, fail run, latest completed/failed run queries
- [x] 4.5 Implement `UpsertNote(tx, note, scanRunID) error` using `INSERT ... ON CONFLICT(project, source_path, section_id) DO UPDATE` (not `INSERT OR REPLACE`)
- [x] 4.6 Implement stale cleanup for a project: delete rows whose `seen_in_scan_id` is not the current completed scan run
- [x] 4.7 Implement structured status query accepting configured project names and returning counts, type counts, latest completed scan, latest failed scan
- [x] 4.8 Write unit tests: schema creation, migration idempotency, upsert new + existing same key preserves row ID, multiple daily-work sections, multi-project Granola rows, stale deletion, failed scan does not delete stale rows, status with unscanned projects, empty database, FTS trigger sync

## 5. Vault scanning — walker and parsing (internal/scan)

- [x] 5.1 Implement safe vault-relative path normalization and validation helpers
- [x] 5.2 Implement `Walk(root string) ([]string, error)` — recursive directory walk returning canonical vault-relative paths of all `.md` files; skip non-.md files; do not follow symlinks escaping vault
- [x] 5.3 Implement file dedupe by canonical vault-relative path before parsing/extraction
- [x] 5.4 Implement bounded `ReadFile(path string, maxBytes int64) (string, error)` — read UTF-8 Markdown; warn/skip oversized or unavailable iCloud files
- [x] 5.5 Implement `ParseFrontmatter(content string) (map[string]interface{}, error)` — extract YAML frontmatter between `---` delimiters; return empty map if no frontmatter; invalid YAML causes the file to be skipped with warning
- [x] 5.6 Implement normalized frontmatter helpers for `tags` and `folders` supporting scalar strings, comma-separated strings, YAML sequences, and `#tag` forms
- [x] 5.7 Implement `HashContent(content string) string` — SHA-256 hex digest of file content for future incremental scan
- [x] 5.8 Write unit tests: walk finds all .md files, skips non-.md, rejects escaping symlinks, dedupes overlapping folders, frontmatter valid/invalid/missing, normalized tags/folders variants, hash stability, oversized/read-error handling

## 6. Note classifier (internal/scan)

- [x] 6.1 Implement `ClassifyType(vaultRelativePath string, fm map[string]interface{}) NoteType` — path-based rules first (daily work, Granola, State.md, fallback knowledge)
- [x] 6.2 Implement frontmatter refinement: normalized `tags` containing `state` → override to `project_state`; `type`/`granola_id` confirms meeting metadata
- [x] 6.3 Implement project resolution for `1.Projects/<Name>/` paths
- [x] 6.4 Implement Granola project matching by configured project name and aliases, case-insensitive normalized matching, with ambiguous/unmatched warnings
- [x] 6.5 Write unit tests: daily work path, meeting path, state filename, knowledge fallback, state tag override, path project resolution, Granola exact match, Granola alias match, multi-project Granola record fan-out, ambiguous alias warning

## 7. Metadata extractors (internal/scan)

- [x] 7.1 Implement `SplitDailyWorkSections(content string) []DailyWorkSection` — split note content into sections by `## ` headings; create stable `section_id` from heading index and slug
- [x] 7.2 Implement `ExtractDailyWork(section DailyWorkSection) (NoteMetadata, bool)` — parse `- Repo:`, `- Branch:`, `- Summary:`, `- Verification:`, `- Notes:`, optional `- Commit:`, `- Ticket:`; skip/warn sections without `- Repo:`
- [x] 7.3 Implement `ExtractMeeting(fm map[string]interface{}) NoteMetadata` — extract `title`, `created`, `updated`, `granola_id`; parse date from `created` if ISO 8601; preserve relevant extras in `metadata_json`
- [x] 7.4 Implement `ExtractGeneric(content string, fm map[string]interface{}, path string) NoteMetadata` — first `# Heading` → title; normalized frontmatter `tags`; fallback title to filename; preserve raw frontmatter `type` such as `decision`/`plan` in `metadata_json`
- [x] 7.5 Implement `ExtractDate(path string, fm map[string]interface{}, noteType NoteType) string` — derive date from filename pattern, fallback to frontmatter `created` or `date` field
- [x] 7.6 Write unit tests: daily work with all fields, daily work missing branch, daily work no repo skipped/warned, multiple sections produce distinct IDs, meeting with complete frontmatter, meeting missing created, knowledge with heading, knowledge without heading, raw decision/plan type preservation, date from filename, date from frontmatter

## 8. Application scan pipeline (internal/app)

- [x] 8.1 Implement `RunProjectScan(project string, config Config, store Store) (ScanSummary, error)` — orchestrates scan transaction and calls scan/store packages
- [x] 8.2 Implement project folder scan: validate project config → walk/dedupe folders → parse/classify/extract → produce `models.Note` records
- [x] 8.3 Implement Granola scanning: walk `4.Granola/`, parse frontmatter `folders:`, match project names/aliases, produce one meeting record per matched project
- [x] 8.4 Implement transaction behavior: start scan run, upsert records with current run ID, delete stale rows, complete run, commit; on failure roll back note changes and record failed scan
- [x] 8.5 Implement warning collection without logging note content or full frontmatter values
- [x] 8.6 Implement scan summary output data: files discovered, records inserted/updated, stale records removed, warnings count, project name
- [x] 8.7 Write integration test: scan a fixture vault with daily work (multiple sections), Granola multi-project meeting, knowledge note, state doc, deleted file re-scan; verify all classified/stored/stale-cleaned correctly

## 9. CLI commands (cmd/zettelbrief)

- [x] 9.1 Wire Cobra root command with subcommands `scan` and `status` in `cmd/zettelbrief/main.go`
- [x] 9.2 Implement `scan` command: `--project` (string) and `--all` (bool) flags; require exactly one mode; load config → open DB → run app scanner → print summary
- [x] 9.3 Implement `scan --all` using sorted project-name order
- [x] 9.4 Implement `status` command: load config → open DB if present → query structured status for configured projects → format plain-text output in CLI layer
- [x] 9.5 Implement `--verbose`/`-v` global flag for per-file progress during scan, still avoiding note content/frontmatter logging

## 10. Acceptance and polish

- [x] 10.1 Create a test fixture vault in `testdata/vault/` with representative notes: daily work with multiple repo sections, Granola meeting with multiple folders, knowledge note, state doc, malformed frontmatter, oversized/unreadable fixture where feasible
- [x] 10.2 Create test config fixture in `testdata/config/` including project aliases and overlapping folders
- [x] 10.3 End-to-end test: `go run cmd/zettelbrief/main.go scan --project VetZ` using fixture vault → verify DB has correct note counts, logical section rows, type counts, and scan run status
- [x] 10.4 End-to-end test: re-run scan after deleting/renaming fixture note → verify stale DB rows are removed
- [x] 10.5 End-to-end test: `go run cmd/zettelbrief/main.go status` → verify configured-but-unscanned projects appear as not yet scanned
- [x] 10.6 Verify `go vet ./...` and `go test ./...` pass with coverage across all packages
- [x] 10.7 Verify the real vault can be scanned: `zettelbrief scan --project VetZ` against actual vault (manual verification)
