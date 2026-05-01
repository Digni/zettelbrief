## Why

Agents produce better plans when they know the local history of a project — previous attempts, meeting decisions, naming conventions, and known risks. That information lives in Obsidian daily work logs, Granola meeting notes, and project knowledge docs. But raw vault search is too noisy and inconsistent to use directly in every agent session. *zettelbrief* is the bridge: it reads those notes, classifies them, extracts structured metadata, and stores everything in a queryable local database. This first change establishes the ingest pipeline — the foundation every subsequent feature builds on.

## What Changes

- Initialize Go module with Cobra CLI framework (`scan`, `status` commands)
- Add YAML configuration system (global `~/.config/zettelbrief/config.yaml` + project `.zettelbrief/config.yaml` override)
- Scan configured Obsidian project folders and Granola meeting notes
- Classify notes by type (daily work, meeting, knowledge, project state) using passive path-based and frontmatter-based detection
- Extract type-specific metadata: repo, branch, date, title, tags, Granola ID, section fields, and project association
- Store classified logical note records in SQLite with FTS5 full-text index
- Track complete/failed scan runs and remove stale rows for deleted/renamed source files after successful scans
- `status` command showing scan freshness, scan result, and note counts per configured project

## Capabilities

### New Capabilities

- `configuration`: YAML-based configuration with global defaults and per-project overrides. Maps project names to Obsidian vault folder paths.
- `vault-scanning`: Walk configured vault folders, discover Markdown notes, resolve which project each note belongs to (path-based for project notes, frontmatter `folders:` field for Granola meetings).
- `note-classification`: Classify notes into types (daily_work, meeting, knowledge, project_state) using normalized vault-relative path patterns and frontmatter. Extract type-specific metadata including repo, branch, date, title, tags, Granola ID, section ID, and project. Preserve raw frontmatter type for future decision/plan routing.
- `sqlite-storage`: SQLite database with `notes` table for logical note records, `scan_runs` tracking, schema migrations, and FTS5 virtual table for full-text indexing. Support conflict-safe upsert, stale cleanup, and structured status queries.

### Modified Capabilities

<!-- No existing capabilities to modify -->

## Impact

- New Go module at repo root, no existing code affected
- New directory: `.zettelbrief/` for runtime artifacts; SQLite DB and generated briefs are gitignored, while optional `.zettelbrief/config.yaml` may be committed if desired
- New config files: `~/.config/zettelbrief/config.yaml` (global) and optional `.zettelbrief/config.yaml` (per-project)
- Dependency: `github.com/spf13/cobra` (CLI), `modernc.org/sqlite` (SQLite), `gopkg.in/yaml.v3` (config)
- Reads from Obsidian vault at configured path — read-only, never writes to vault
- Local SQLite database contains copied private note content and is created with restrictive permissions where supported
