## Why

Change 1 made configured Obsidian project notes queryable in SQLite, but the CLI still only supports `init`, `scan`, and `status` (`cmd/zettelbrief/main.go:26-29`). This change delivers the next roadmap increment: turning indexed notes into cited briefing packs that agents can read before planning work (`README.md:24-36`, `ROADMAP.md:57-79`).

## What Changes

- Add a `zettelbrief fetch --project <name> [--repo <repo>] [--type <type>] [--since <date>] [--until <date>] "query"` command that searches the local SQLite database and writes a briefing output directory. Date flags are fetch-only in this change; scan date slicing remains Change 3 scope.
- Add safe FTS5 query construction over note-specific searchable text, avoiding raw user-query interpolation and avoiding false matches across daily-work sections from the same file.
- Add metadata filtering for project, repo, fetch date range, and note type using parameterized SQL against the existing `notes` table columns. Repo filtering excludes other repo-specific rows while keeping project-level rows with no repo.
- Add deterministic relevance scoring based on keyword match density, section-specific note-type weighting, and stable tie-breaking, without recency weighting or confidence markers.
- Add a brief composer that routes retrieved notes into the README/roadmap sections using concrete field/heading predicates and writes `.zettelbrief/briefs/<timestamp>/{brief.md,sources.json}`.
- Add tests covering safe search, filters, scoring/order, brief routing, output privacy, source mapping, output file generation, and CLI behavior.

## Capabilities

### New Capabilities
- `brief-fetching`: Covers the `fetch` CLI command, full-text/metadata retrieval, deterministic relevance scoring, briefing file generation, and source mapping.

### Modified Capabilities
- `sqlite-storage`: Add query-facing requirements for safe FTS5 retrieval and metadata-filtered note lookup over the existing stored note schema.

## Impact

- CLI: `cmd/zettelbrief/main.go` currently registers only `init`, `scan`, and `status`; it will add `fetch` and associated flags.
- Store layer: `internal/store/db.go` already opens `.zettelbrief/zettelbrief.db`, applies `busy_timeout`, enables WAL, and migrates FTS5-backed tables; it will gain read/query helpers and any needed note-specific indexing/normalization while preserving scan/status behavior.
- Data model: `internal/models/note.go` already exposes project, type, repo, branch, date, title, summary, notes text, source path, content, tags, and metadata fields used for filtering, routing, and citation.
- Output files: local `.zettelbrief/briefs/<timestamp>/brief.md` and `sources.json` will be created beside the existing database directory.
- Tests: store, app/composer, and end-to-end CLI tests will expand from scan/status coverage to fetch/search/brief coverage.
