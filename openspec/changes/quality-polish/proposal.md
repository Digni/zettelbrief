## Why

Change 3 makes generated briefs useful for agent decision-making, not merely source-backed. Existing fetch output ranks by keyword/type only and renders basic entry text; the roadmap calls for excerpts, confidence, recency, scan date slicing, incremental scan behavior, stronger validation, and broader test coverage.

## What Changes

- Improve fetch result quality with per-classification excerpts, evidence-based confidence markers, structured `match_reason`, and richer `sources.json` fields including excerpt, score, recency factor, and offsets where available.
- Add type-conditional recency scoring: apply bounded decay to `daily_work` and `meeting` notes while leaving `project_state` and `knowledge` authoritative regardless of age.
- Improve query handling by stripping task-framing stopwords and weighting identifier-like terms higher during scoring.
- Add non-destructive `--since` and `--until` slicing to `scan` in addition to existing fetch date filters; scan date filters limit what is processed but do not delete already-indexed out-of-range rows.
- Add measured, threshold-gated incremental scan support that may skip unchanged project files by content hash when benchmarks justify it, while preserving stale-row cleanup correctness, scan run accounting, and FTS efficiency; Granola meeting files are always parsed because project association depends on current config aliases.
- Tighten graceful validation and warning behavior for missing vaults, empty project folders, malformed notes, malformed dates, and unsupported metadata shapes.
- Extend test coverage across scan, fetch, brief composition, search/scoring, configuration validation, and storage behavior.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `brief-fetching`: Brief entries gain excerpts, repo-grounded confidence, match reasons, type-conditional recency scoring, richer source mappings, and improved query token handling.
- `vault-scanning`: Scan accepts inclusive non-destructive date slicing and supports safe incremental processing of unchanged non-Granola files.
- `configuration`: Validation and user-facing diagnostics are tightened for missing vaults, empty folders, malformed note metadata, and unsupported shapes.
- `sqlite-storage`: Storage/query behavior changes to expose snippet data, persisted scan/change metadata needed for threshold-gated incremental scans, FTS trigger behavior that avoids visibility-only reindexing, and structured source fields.

## Impact

- Affected CLI/API surface: `zettelbrief scan` gains `--since` and `--until`; `zettelbrief fetch` output remains in the same directory shape but `brief.md` and `sources.json` include richer data.
- Affected code areas: `internal/app/scan.go`, `internal/app/fetch.go`, `internal/store/search.go`, `internal/store/db.go`, `internal/store/notes.go`, `internal/brief/brief.go`, `internal/config/config.go`, scanner/parser helpers, and their tests.
- Affected systems: local SQLite database schema/FTS query behavior and local briefing output files under `.zettelbrief/briefs/`.
- No embeddings, MCP server, chat interface, or external agent API are introduced.
