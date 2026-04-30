## Context

*zettelbrief* is a new Go CLI tool. The Obsidian vault at `~/Library/Mobile Documents/iCloud~md~obsidian/Documents/Default` contains project notes organized under `1.Projects/<Name>/` and Granola meeting notes under `4.Granola/`. Notes vary in structure — daily work logs use a semi-structured bullet convention, Granola meetings use frontmatter, and general knowledge notes are freeform Markdown. There is no universal classification scheme; note types must be inferred from where a file lives and what its frontmatter says.

This change establishes the ingest pipeline: scan the vault, classify notes, extract metadata, and store in SQLite. It is the foundation Change 2 (search + brief generation) and Change 3 (quality + polish) depend on.

## Goals / Non-Goals

**Goals:**
- Initialize a working Go CLI with `scan` and `status` subcommands
- Load configuration from global and per-project YAML files, merged at runtime
- Walk configured vault folders, discover all `.md` notes for configured projects
- Classify each note into a type: `daily_work`, `meeting`, `project_state`, or `knowledge`
- Preserve raw frontmatter type/tags so future `decision` and `plan` routing can be added without rescanning semantics changing
- Extract structured metadata (project, repo, branch, date, title, tags, Granola ID, section fields) per logical note record
- Store classified note records in a SQLite database with FTS5 full-text index
- Track complete/failed scan runs and remove stale rows for deleted/renamed source files after a successful full project scan
- `status` command reports scan timestamp, scan result, and per-project note counts including configured projects that have never been scanned

**Non-Goals:**
- Search, relevance scoring, or brief generation (Change 2)
- Incremental scan or change detection optimization (Change 3)
- Embedding-based semantic search (Post-v1)
- Writing anything to the Obsidian vault (read-only)
- Watching the vault for live changes

## Decisions

### 1. Project layout: standard Go with thin application orchestration

```
zettelbrief/
├── cmd/
│   └── zettelbrief/
│       └── main.go           # cobra root command wiring
├── internal/
│   ├── app/
│   │   └── scan.go           # orchestration: config + scan + store transactions
│   ├── config/
│   │   └── config.go         # Config struct, loading, merge, path validation
│   ├── scan/
│   │   ├── walker.go         # Filesystem walk, file discovery
│   │   ├── classifier.go     # Type detection (path + frontmatter)
│   │   └── extractor.go      # Metadata extraction per type; returns models.Note records
│   ├── models/
│   │   └── note.go           # Note, NoteType, ScanRun, Status types
│   └── store/
│       ├── db.go             # SQLite connection, migrations, pragmas
│       ├── notes.go          # CRUD operations on notes
│       └── status.go         # structured status queries
├── .zettelbrief/             # Runtime DB/briefs ignored; config.yaml may be committed if desired
└── go.mod
```

**Rationale**: `scan` remains filesystem/parsing focused and does not import SQLite. `store` remains persistence focused and does not format CLI output. `app` owns transactions, scan run lifecycle, and stale-row cleanup.

**Alternative considered**: `scan.Run(project, config, db)`. Rejected: it couples parsing/classification to SQLite and makes unit testing and future non-SQL outputs harder.

### 2. Configuration: global + discovered project merge

```yaml
# ~/.config/zettelbrief/config.yaml (global)
vault_path: "/Users/.../Obsidian/Documents/Default"
projects:
  VetZ:
    folders:
      - "1.Projects/VetZ"
      - "1.Projects/VetZ/Backend"
    aliases:
      - "Vetz"          # optional Granola folder-name aliases
  Flive:
    folders:
      - "1.Projects/Flive"

# .zettelbrief/config.yaml (per-project, optional)
# Adds/overrides project entries but cannot set vault_path.
```

Resolution order: global file loaded first → nearest `.zettelbrief/config.yaml` discovered by walking upward from the current directory is merged on top (if it exists). Per-project config can add or override project entries but not redefine `vault_path`.

Validation rules:
- `vault_path` is required for scan/status commands that open the vault and must point to an existing directory.
- Project `folders` must be vault-relative paths. Absolute paths, `..` traversal, empty paths, and paths that resolve outside `vault_path` are rejected.
- Symlinked directories are not followed by default. If a configured folder itself is a symlink, its real path must resolve inside `vault_path`.
- Project names are the canonical keys. `aliases` are optional and only used to match Granola `folders:` frontmatter.

**Rationale**: Vault path is machine-local and belongs in global config. Project config acts like Git config discovery so invoking the CLI from a repo subdirectory still works. Strict path validation prevents accidental ingestion outside the vault.

**Alternative considered**: Single project-local config with vault path in environment variable. Rejected: vault path is per-machine, not per-project.

### 3. Classification: normalized vault-relative paths, then frontmatter

All scanner functions receive clean, slash-separated, vault-relative paths. Absolute paths are converted once at the walker boundary after validating they remain under `vault_path`.

Classifier runs in two passes for each file:

```
1. Path-based detection (deterministic, always runs):
   - Path contains "/1. Daily Work/" or starts with "1. Daily Work/" → daily_work
   - Path starts with "4.Granola/"                              → meeting
   - Filename is "State.md"                                     → project_state
   - Fallback                                                    → knowledge

2. Frontmatter-based refinement (runs when frontmatter exists):
   - tags containing "state" after normalization                 → project_state
   - folders                                                     → maps Granola note to project(s)
   - type/granola_id                                             → confirms meeting metadata
```

Frontmatter helpers normalize common Obsidian/Granola shapes: scalar string, comma-separated string, YAML sequence, `#tag` values, and casing differences. Unexpected but valid shapes are warned and ignored rather than panicking.

**Rationale**: Path-based detection handles the 90% case without relying on frontmatter. Frontmatter provides precision for Granola notes and state documents. Normalization keeps the classifier passive while tolerating real vault variation.

**Alternative considered**: Frontmatter-only classification. Rejected: most daily work notes lack frontmatter entirely.

### 4. Metadata extraction: type-specific strategy producing logical note records

Each source Markdown file may produce one or more `models.Note` records:

| Note Type | Extraction Strategy | Row granularity |
|-----------|---------------------|-----------------|
| `daily_work` | Regex parse `- Repo:`, `- Branch:`, `- Summary:`, `- Verification:`, `- Notes:`, optional `- Commit:`, `- Ticket:` per `## Section` | One row per section with `section_id` derived from heading index/slug |
| `meeting` | Frontmatter: `granola_id`, `title`, `created`, `updated`, `folders:`. Content: section titles/action item text preserved in `metadata_json` | One row per matched project |
| `project_state` | Frontmatter: `tags`, `updated`. Content: first heading and section titles in `metadata_json` | One row per source file/project |
| `knowledge` | First `# Heading` → title. Frontmatter `tags:` and raw frontmatter `type` preserved in `metadata_json` | One row per source file/project |

Daily work sections without `- Repo:` are skipped with a warning because repo-specific briefs depend on that association. The full source content is kept on each logical row for v1 simplicity; `source_path + section_id` identifies the cited section.

**Rationale**: Daily work files often contain multiple repo/branch entries. Treating a file as one row would collapse metadata and make repo filters incorrect. A logical record model also supports Granola notes associated with multiple projects.

**Alternative considered**: Single row per source file plus JSON arrays. Rejected: it complicates filtering and FTS result citation in Change 2.

### 5. SQLite schema: logical notes + scan runs + FTS5

```sql
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE scan_runs (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    project      TEXT NOT NULL,
    started_at   TEXT NOT NULL DEFAULT (datetime('now')),
    completed_at TEXT,
    status       TEXT NOT NULL, -- running, completed, failed
    error        TEXT,
    files_seen   INTEGER NOT NULL DEFAULT 0,
    notes_seen   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE notes (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    project       TEXT NOT NULL,
    type          TEXT NOT NULL,           -- daily_work, meeting, knowledge, project_state
    section_id    TEXT NOT NULL DEFAULT '',-- empty for whole-file records
    repo          TEXT,
    branch        TEXT,
    date          TEXT,                    -- ISO 8601 date
    title         TEXT,
    summary       TEXT,
    verification  TEXT,
    notes_text    TEXT,
    commit_hash   TEXT,
    ticket        TEXT,
    granola_id    TEXT,
    updated_at    TEXT,
    tags          TEXT,                    -- JSON array of normalized tags
    source_path   TEXT NOT NULL,           -- vault-relative path
    content       TEXT NOT NULL,           -- full Markdown content
    content_hash  TEXT NOT NULL,           -- SHA-256 for future incremental scan
    metadata_json TEXT,                    -- type-specific extras; no secrets added by app
    seen_in_scan_id INTEGER,
    scanned_at    TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project, source_path, section_id),
    FOREIGN KEY(seen_in_scan_id) REFERENCES scan_runs(id)
);

CREATE VIRTUAL TABLE notes_fts USING fts5(
    title,
    summary,
    content,
    tags,
    content='notes',
    content_rowid='id'
);

-- Triggers keep FTS5 in sync for INSERT, UPDATE, DELETE.
```

`UpsertNote` uses `INSERT ... ON CONFLICT(project, source_path, section_id) DO UPDATE`, not `INSERT OR REPLACE`, so row IDs remain stable and FTS triggers receive normal updates.

A successful full project scan runs in a transaction:
1. create/start a `scan_runs` row,
2. upsert all discovered logical note records with `seen_in_scan_id`,
3. delete rows for that project whose previous `seen_in_scan_id` was not updated by the current scan,
4. mark the run completed,
5. commit.

If scanning fails, the note changes roll back and the failed run is recorded separately so `status` does not treat a partial scan as fresh.

**Rationale**:
- `content_hash` enables future incremental scan without schema migration
- `UNIQUE(project, source_path, section_id)` handles daily work sections and multi-project Granola notes
- `scan_runs` distinguishes complete scans from partial/failed scans
- Stale cleanup prevents deleted or renamed vault files from remaining searchable forever
- FTS5 over `title`, `summary`, `content`, and `tags` covers the search surface for Change 2
- `schema_migrations` provides an upgrade path for Change 2/3 schema changes

**Alternative considered**: `UNIQUE(source_path)`. Rejected: cannot represent multi-section daily logs or multi-project meetings.

### 6. Scanner pipeline: discover → classify → extract → persist in app layer

```
RunProjectScan(project) error {
    validate project config and paths
    begin project scan transaction
    records := []models.Note{}

    for each configured project folder {
        files := walk(folder)
        dedupe files by canonical vault-relative path
        for each .md file {
            content := read bounded file
            frontmatter := parse frontmatter
            records += classify + extract project-folder note records
        }
    }

    granolaFiles := walk("4.Granola") once per project scan
    for each Granola .md file {
        content/frontmatter := parse
        if folders matches project name or aliases {
            records += extract meeting record for project
        }
    }

    store upserts records
    delete stale records for project
    complete scan transaction
}
```

File reads are bounded by a configurable/default maximum note size (warn and skip files over the limit). Per-file warnings include invalid frontmatter, unsupported frontmatter shapes, unmatched Granola folders, skipped daily-work sections, and iCloud/read errors. A project folder read error aborts the scan; an individual file read/frontmatter error is warned and skipped unless it prevents safe path validation.

`scan --all` scans projects in sorted project-name order for deterministic behavior. Granola parsing may be cached within the command invocation to avoid reparsing the same files for every project, but correctness does not depend on caching.

**Rationale**: Straight-line pipeline is easy to test, easy to debug, and fast enough for a few hundred files. Transactions and stale cleanup make full scans trustworthy.

### 7. CLI design: cobra with root + subcommands

```
zettelbrief scan --project <name>     # Scan one project
zettelbrief scan --all                # Scan all configured projects, sorted by project name
zettelbrief status                    # Show scan state for all configured projects
```

Root command (`zettelbrief`) does nothing by itself — it requires a subcommand. Project flag is required for `scan` unless `--all` is passed. CLI commands use structured data from `store`/`app` and handle formatting at the edge.

`status` loads config, opens the DB if present, and reports every configured project. Projects without a completed scan are shown as `not yet scanned`. Missing DB is not an error for `status`; missing/invalid config is an error if configured project names cannot be determined.

### 8. SQLite driver and local privacy posture

Use `modernc.org/sqlite` (pure Go SQLite driver) with WAL mode and a busy timeout where supported. The `.zettelbrief` runtime directory is created with restrictive permissions (`0700`), and the database file is created with restrictive permissions (`0600`) on platforms that support POSIX permissions. SQL statements use parameters for all user/config/vault-derived values. Future Change 2 FTS `MATCH` queries must escape/tokenize user input rather than interpolating raw query text.

The database contains full private note content copied from Obsidian/Granola. The app must not log note content or full frontmatter by default; warnings may include vault-relative paths and field names only.

**Rationale**: The tool is local-only, but the DB is a second copy of private project and meeting notes. Restrictive permissions, logging discipline, and gitignore rules reduce accidental exposure.

**Alternative considered**: `mattn/go-sqlite3`. Rejected: requires CGo, complicates cross-compilation for macOS/Linux.

## Risks / Trade-offs

**[Granola folder name drift]**: Granola `folders:` frontmatter may use spelling/casing that differs from config project names. → Match canonical names and configured aliases with case-insensitive normalization. Log unmatched folder names as warnings.

**[Daily work bullet format variation]**: Not all daily work notes use the exact `- Repo:`, `- Branch:` convention. → Extractor is lenient for optional fields, trims whitespace, handles missing optional bullets gracefully, and warns/skips sections without `- Repo:`.

**[Vault not available]**: The vault is an iCloud-synced directory that may be offline, contain placeholder files, or have sync conflicts. → Config validation errors if the vault root is missing. Per-file read errors are warned/skipped; project folder traversal errors abort the scan with a failed scan run.

**[Large vault performance]**: Scanning the entire Granola directory for every project scan is wasteful if project A doesn't care about project B's meetings. → For `--all`, parse Granola notes once per command invocation and fan out matched project associations where practical. Full correctness still comes from project-based filtering.

**[Schema evolution]**: Change 2/3 will add search and quality metadata. → Include `schema_migrations` from Change 1 and use additive migrations for future changes.

## Open Questions

- Should `zettelbrief status` output be plain text or JSON-structured for programmatic consumption? (Leaning: plain text for v1, JSON with `--json` flag later)
- Should the scanner log a per-file progress indicator or only summary output? (Leaning: summary only by default, verbose with `-v`)
