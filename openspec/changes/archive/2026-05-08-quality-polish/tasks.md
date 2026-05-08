## 1. Baseline and Test Scaffolding

- [x] 1.1 Add focused fixtures/tests for current fetch output and scan behavior before changing implementation.
- [x] 1.2 Add a checked-in `testdata/` scan performance benchmark fixture for representative unchanged vault files; measure at least 5 warm-cache iterations and report total file count, Granola share, and changed/unchanged ratio.
- [x] 1.3 Run `go test ./...` and record the baseline expected pass state before implementation changes.

## 2. Store Search and Source Data

- [x] 2.1 Extend search query normalization to classify raw query spans before lowercasing, remove the frozen deterministic stopword list, preserve identifier-like raw query spans as scoring groups, and OR FTS component terms within identifier groups.
- [x] 2.2 Add tests for stopword removal, app-layer stopword-only rejection messaging, dotted/camelCase/snake_case/kebab-case identifier detection before lowercasing, OR-within-identifier matching, and FTS metacharacter safety.
- [x] 2.3 Introduce a brief/app-layer `brief.Candidate` DTO mapped from store search results before composition so new quality behavior does not deepen `internal/brief` coupling to persistence DTOs.
- [x] 2.4 Extend store SQL retrieval to include bounded FTS5 snippet data with explicit column selection and fallback behavior, then map snippets into `brief.Candidate` before brief composition.
- [x] 2.5 Add store helpers to look up content hashes by project/source path and mark existing source-path rows seen by updating `seen_in_scan_id` and `scanned_at` with project scoping, date-filter safety, and no FTS content reindex.
- [x] 2.6 Verify with `go test ./internal/store`.

## 3. Brief Quality

- [x] 3.1 Refactor scoring to consume `brief.Candidate` and return base score, final score, and recency factor with type-conditional decay using UTC today and clamped non-negative age.
- [x] 3.2 Add identifier-aware score weighting over `brief.Candidate` normalized identifier groups while preserving deterministic tie-breaking.
- [x] 3.3 Implement classification-specific excerpt selection from `brief.Candidate` for daily work, meeting, project state, and knowledge notes.
- [x] 3.4 Implement confidence and `match_reason` derivation from `brief.Candidate` for direct repo matches, project-level matches, branch query matches that do not become HIGH, and keyword-only matches.
- [x] 3.5 Extend `SourceMapping` JSON fields from `brief.Candidate` with classification, confidence, match reason, excerpt, score, recency factor, and best-effort `omitempty` offsets only for unique-substring matches.
- [x] 3.6 Update Markdown rendering of `brief.Candidate` entries to show bounded evidence excerpts and confidence markers without removing the Sources section or falling back to unbounded raw content.
- [x] 3.7 Verify with `go test ./internal/brief ./internal/app`.

## 4. Scan Date Slicing and Validation

- [x] 4.1 Add scan options and CLI flags for `--since` and `--until` using the existing YYYY-MM-DD validation semantics.
- [x] 4.2 Filter parsed logical notes by inclusive date bounds and exclude undated notes from writes while scan date filters are active, and skip stale cleanup for date-sliced scans so existing out-of-range rows are not deleted solely due to the filter.
- [x] 4.3 Update scan stale-cleanup tests for non-destructive date-sliced behavior and ordinary full-scan stale cleanup.
- [x] 4.4 Tighten `config.validateProject` validation for empty project folder lists, missing vault/folder errors, malformed metadata/date warnings (including date extractor status), and privacy-safe warning text.
- [x] 4.5 Verify with `go test ./internal/app ./internal/config ./internal/scan`.

## 5. Incremental Scan

- [x] 5.1 Use the measurement from task 1.2 to decide whether to enable incremental scan now; enable it only if representative full scan median is greater than 2 seconds or a 90%-unchanged/10%-changed run demonstrates at least a 25% wall-clock improvement, and if measurement contradicts the design, stop and update the plan before implementing a workaround.
- [x] 5.2 Deferred: benchmark gate was not met, so scan code does not enable content-hash skipping in this change.
- [x] 5.3 Deferred: because incremental skipping is disabled, Granola files continue to be parsed by the existing full-scan path.
- [x] 5.4 Deferred: because incremental skipping is disabled, changed/deleted files and removed sections continue to use existing full-scan upsert/stale cleanup semantics.
- [x] 5.5 Deferred: incremental-specific tests are not applicable until a future benchmark justifies enabling skip behavior; date-filter preservation and FTS mark-seen helper coverage were added where relevant.
- [x] 5.6 Verified applicable behavior with `go test ./internal/app ./internal/store`.

## 6. End-to-End Verification and Documentation

- [x] 6.1 Update CLI help/tests for new scan date flags, non-destructive scan-date behavior, benchmark gate documentation, and any changed validation messages.
- [x] 6.2 Add end-to-end fetch tests asserting enriched `brief.md` and `sources.json` fields.
- [x] 6.3 Run `go test ./...` and ensure all packages pass.
- [x] 6.4 Run `openspec status --change quality-polish` and confirm the change is apply-ready.
