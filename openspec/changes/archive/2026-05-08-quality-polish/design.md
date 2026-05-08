## Context

Change 3 is the roadmap quality phase: incremental scan, recency scoring, snippets, confidence markers, scan/fetch date slicing, validation polish, and full test coverage (`ROADMAP.md:86-101`). The additional design notes prioritize excerpt quality, confidence/match reasons, type-conditional recency, query stopwords/identifiers, date flags, and only implementing incremental scan after measuring whether full scans are actually slow (`docs/change-3-design-notes.md:7-110`).

Current fetch search accepts project/repo/type/date/query fields (`internal/store/search.go:19-24`), tokenizes all query words equally (`internal/store/search.go:50-65`), queries FTS5 with parameterized filters (`internal/store/search.go:85-123`), and returns content plus metadata but no snippet/excerpt/match-reason fields (`internal/store/search.go:27-48`). Current brief composition scores by keyword density plus type weight (`internal/brief/brief.go:68-85`), sorts deterministically (`internal/brief/brief.go:87-99`), renders simple one-line text (`internal/brief/brief.go:102-136`, `internal/brief/brief.go:200-207`), and maps each entry to basic source fields only (`internal/brief/brief.go:34-42`, `internal/brief/brief.go:139-144`).

Current scanning fully parses every discovered file and upserts every logical note on each successful scan (`internal/app/scan.go:24-67`, `internal/app/scan.go:70-99`). Existing storage already persists `content_hash` and `seen_in_scan_id` in `notes` (`internal/store/db.go:54-58`) and updates FTS via triggers (`internal/store/db.go:91-96`), which is enough to mark unchanged rows seen without re-parsing if done carefully. Current scan validation already checks missing vaults, unknown projects, and invalid folders (`internal/config/config.go:144-190`), so this change should refine messages and warnings rather than replace the validation model.

I ran `go test ./...` before drafting; all existing packages passed.

## Goals / Non-Goals

**Goals:**

- Make brief entries evidence-rich: each rendered entry has a concise excerpt, confidence marker, and match reason suitable for scan-then-open agent workflows.
- Make `sources.json` programmatically useful by adding classification, confidence, match reason, excerpt, score, recency factor, and best-effort character offsets.
- Improve relevance deterministically with type-conditional recency and signal-aware query terms while preserving stable tie-breaking.
- Add inclusive non-destructive `--since`/`--until` slicing to scan and keep fetch date semantics intact.
- Measure scan performance first, then implement incremental scan only with tests that preserve stale cleanup and FTS consistency; unchanged Granola files are always parsed because project association depends on current config aliases.
- Improve validation messages/warnings without making note classification prescriptive.

**Non-Goals:**

- Embeddings or semantic search.
- MCP, chat, or agent-facing server APIs.
- Writing to the Obsidian vault.
- LLM-based summarization or classification.
- Public output-format version negotiation; the existing `brief.md`/`sources.json` files are enriched in place.

## Decisions

1. **Build excerpts by classification, with FTS5 snippet fallback.**
   - Decision: add excerpt construction after store search and before rendering. `daily_work` uses `Summary` then `Notes`; `project_state` uses frontmatter/metadata summary if present, otherwise the first paragraph after title; `meeting` and `knowledge` use SQLite `snippet(notes_fts, <column>, '', '', '…', <tokens>)` exposed by store search, with explicit FTS column indexes selected by note type rather than `-1` when practical. Snippet marker strings and token counts are compile-time constants. If FTS5 returns an empty/null snippet or errors for the external-content table, fallback to the first bounded 120 tokens of content. This follows `docs/change-3-design-notes.md:7-26` and matches existing metadata returned from `internal/store/search.go:27-48`.
   - Alternative considered: a universal token window over `Content`. Rejected because daily work already has parser-isolated fields and meetings need match-density snippets.
   - Verification checkpoint: `go test ./internal/store ./internal/brief` should include tests for each note type's excerpt source and pass.

2. **Make confidence evidence-based, not just type-based.**
   - Decision: compute confidence and `match_reason` from actual filter-backed evidence and matched/query terms: HIGH only for direct repo filter-to-metadata matches; MEDIUM for project-level matches without a direct repo match; LOW for keyword-only matches. Branch metadata may be included in `match_reason` and excerpts when matched, but branch matches alone do not raise confidence to HIGH because the current CLI has no branch filter (`cmd/zettelbrief/main.go:121-149`; `internal/store/search.go:19-24`). For HIGH, render evidence such as repo/summary into the excerpt when available, as recommended by `docs/change-3-design-notes.md:30-38`.
   - Alternative considered: infer HIGH/MEDIUM/LOW purely from note type. Rejected because the design notes require the excerpt and match reason to show why a result was trusted.
   - Verification checkpoint: `go test ./internal/brief` should assert confidence and match_reason values for repo-specific, project-level, branch-in-query-but-not-HIGH, and keyword-only entries.

3. **Extend source mappings as the stable programmatic surface and reduce store/brief coupling.**
   - Decision: extend `brief.SourceMapping` beyond the current fields (`internal/brief/brief.go:34-42`) with `classification`, `confidence`, `match_reason`, `excerpt`, `char_offset_start`, `char_offset_end`, `score`, and `recency_factor`. Character offsets are best-effort and serialized with `omitempty`: exact only for direct field excerpts when the excerpt is a unique substring in stored content, snippet-derived only when derivable, and omitted when unknown or ambiguous. While the current `brief.Entry` stores `store.SearchResult` directly (`internal/brief/brief.go:21-25`), this change should introduce an application/brief-layer candidate DTO (for example `brief.Candidate`) mapped from store results in `internal/app/fetch.go` before composition so new quality fields do not deepen persistence DTO coupling.
   - Alternative considered: only enrich Markdown. Rejected because `sources.json` is the agent-action surface in `docs/change-3-design-notes.md:57-76`.
   - Alternative considered: continue passing `store.SearchResult` through the brief package. Rejected for the new quality work because snippets, confidence, and recency are brief-domain concerns and can be carried by a narrower DTO without changing storage behavior.
   - Verification checkpoint: `go test ./internal/app ./internal/brief` should compare generated JSON fields, ensure existing citation fields remain present, and ensure `internal/brief` no longer needs additional persistence-specific fields for new quality behavior.

4. **Apply recency only where recency indicates usefulness.**
   - Decision: refactor `brief.Score` (`internal/brief/brief.go:68-85`) into base score plus `recency_factor`. For `daily_work` and `meeting`, define today's date as UTC and apply `max(0.3, 1 - age_days/180)` to the base score after clamping `age_days` to `>= 0` so future-dated notes do not receive factors above `1.0`; for `project_state` and `knowledge`, use factor `1.0`, matching `docs/change-3-design-notes.md:42-53`. Missing dates use factor `1.0` for non-decayed types and the bottom cap for decayed types. Malformed dates discovered during scan are warning-worthy metadata issues, not silent recency inputs.
   - Alternative considered: global linear decay. Rejected because older state/decision/knowledge notes can be authoritative.
   - Verification checkpoint: `go test ./internal/brief` should cover fresh/stale daily work, meetings, project state, knowledge, missing dates, and stable ties.

5. **Normalize query terms before search and scoring without adding new flags in this change.**
   - Decision: update tokenization (`internal/store/search.go:50-65`) to classify raw query spans before lowercasing, remove a small frozen internal stopword set (`a`, `an`, `and`, `add`, `change`, `do`, `fix`, `for`, `in`, `of`, `on`, `the`, `to`, `update`, `with`, `work`), and mark identifier-like terms. Identifier detection is deterministic: preserve the original raw query spans before lowercasing, classify dotted (`One.Backend`), snake_case, kebab-case, slash/path-like, and camelCase spans as identifiers, then normalize them into FTS-compatible component tokens while keeping an identifier group for scoring. Build FTS expressions as AND across query groups, but OR within identifier component groups (for example a dotted identifier may search `(one OR backend)` while scoring the group as one high-signal identifier) so identifier normalization does not over-constrain recall. Search still builds a safe parameterized FTS expression (`internal/store/search.go:67-83`, `internal/store/search.go:117-124`), while scoring weights identifier hits higher. Optional `--terms`/`--identifier` flags from the design notes remain future scope unless implementation shows no clean internal path.
   - Alternative considered: add CLI flags now. Rejected to keep the public interface minimal while delivering the deterministic improvement called out in `docs/change-3-design-notes.md:80-89`.
   - Verification checkpoint: `go test ./internal/store ./internal/brief` should cover stopword-only rejection with a friendly app-layer error, dotted/camelCase/snake_case/kebab-case identifier normalization before lowercasing, identifier weighting, OR-within-identifier groups, and FTS metacharacter safety.

6. **Add scan date slicing before incremental scan.**
   - Decision: introduce scan options carrying `Since`/`Until`, validate them with the same `YYYY-MM-DD` rules used by fetch (`internal/app/fetch.go:70-107`), and filter parsed logical notes by note date before upsert. Date slicing is non-destructive: active scan date filters exclude out-of-range and undated notes from writes in that invocation, and `RunProjectScan` must skip `DeleteStaleNotesTx` entirely for date-sliced runs so existing rows outside the active date slice cannot be deleted by cleanup. This matches the selected policy that scan date flags are temporary processing filters rather than database rescope operations.
   - Alternative considered: file path date filtering only. Rejected because meeting/project-state dates may come from frontmatter or parser metadata, not only path names.
   - Verification checkpoint: `go test ./internal/app` should demonstrate inclusive bounds, invalid dates, reversed ranges, undated-note exclusion from writes when filters are active, and preservation of existing out-of-range rows.

7. **Defer incremental scan because the benchmark gate was not met.**
   - Decision: first add a scan benchmark/test fixture that measures full scan behavior. Enable incremental skipping only if a checked-in `testdata/` benchmark corpus is measured over at least 5 warm-cache iterations, reports total file count, Granola file share, and changed/unchanged ratio, and the representative full scan median is greater than 2 seconds or a 90%-unchanged/10%-changed run demonstrates at least a 25% wall-clock improvement. The implemented benchmark fixture reports 20 total Markdown files, 10% Granola files, and a 90% unchanged / 10% changed scenario; `go test ./internal/app -run '^$' -bench BenchmarkScanPerformanceFixture -benchtime=5x -count=1` measured about `4432117 ns/op`, far below the 2-second gate. Therefore this change keeps full-scan behavior and defers active incremental skipping to future work with a larger real-vault measurement. Store hash lookup and mark-seen helpers may exist for future use, but scan code must not skip parsing based on content hash in this change.
   - Alternative considered: add a separate file-cache table immediately. Rejected because existing row hashes are sufficient for ordinary project files and adding cache invalidation before measuring is unnecessary complexity; Granola files are excluded from skipping instead of adding a config fingerprint.
   - Verification checkpoint: `go test ./internal/app ./internal/store` should prove unchanged non-Granola rows keep stable IDs/content, mark-seen updates `scanned_at` without FTS reindex, unchanged Granola files are still parsed for current alias associations, changed rows update and reindex, deleted files/sections are stale-cleaned on non-date-sliced full scans, date-filtered scans skip cleanup and preserve out-of-range rows, and failed scans do not mark stale rows.

8. **Tighten validation by extending existing paths.**
   - Decision: keep `config.ValidateForScan` as the central scan validation path (`internal/config/config.go:144-190`) and add targeted diagnostics in `config.validateProject` for empty project folder lists, unsupported frontmatter/list shapes, malformed dates from `ExtractDate`/frontmatter values (`internal/scan/extractor.go:115-148`), and missing vault/folder cases. Warnings must avoid note content and full frontmatter values, matching existing scan summary privacy behavior.
   - Alternative considered: a separate validation subsystem. Rejected because existing config and scan parsing already collect errors/warnings in one flow (`internal/app/scan.go:29-35`, `internal/app/scan.go:102-120`).
   - Verification checkpoint: `go test ./internal/config ./internal/app ./internal/scan` should cover graceful errors and privacy-safe warnings.

## Risks / Trade-offs

- FTS5 `snippet()` with external-content tables can be brittle across SQLite drivers → Add store-level tests against `modernc.org/sqlite` and fall back to field excerpts if snippet query fails.
- Incremental scan can accidentally delete rows if unchanged files are not marked seen → Keep stale cleanup in one transaction for non-date-sliced full scans, skip cleanup for date-sliced scans, and add tests for unchanged files, deleted files, removed daily-work sections, date-filtered scans, and failed scans.
- Scan date slicing can be mistaken for destructive database rescoping → Define it as non-destructive in specs, preserve existing out-of-range rows, and document that scan date flags limit processing for the current invocation only.
- Granola project association can change without file-content changes when config aliases change → Always parse Granola files rather than content-hash skipping them; project-scoped stale rows from removed aliases are cleaned when the formerly associated project is scanned (or `scan --all` is run), not when only a different project is scanned.
- Recency can hide old but relevant notes → Bottom-cap daily_work/meeting decay at 0.3 and leave state/knowledge un-decayed.
- Richer excerpts may expose more private note content in outputs → Keep existing restrictive output permissions and keep excerpts bounded to the 80-120 token target.

## Migration Plan

- Schema changes, if any, must be additive and idempotent in `DB.Migrate`; existing databases must open without manual migration.
- Existing `brief.md` and `sources.json` consumers remain compatible because existing fields stay present and new JSON fields are additive.
- If incremental scan proves unsafe, unnecessary after measurement, or unable to satisfy date-filter/Granola alias rules, keep full-scan behavior and still deliver the other quality improvements.
- Rollback strategy: disable incremental path and retain full upsert scanning; enriched fetch output can be rolled back by restoring old render/source mapping while keeping additive DB schema harmless.

## Open Questions

- Should optional `--terms` and `--identifier` flags be added now? The current design defers them.

## Planning Verification

- [x] Every file/line reference was read directly by me (not from subagent summary alone)
- [x] I ran diagnostic commands myself for facts in the plan (`go test ./...`)
- [x] Each step has a verification checkpoint with concrete command and expected outcome
- [x] I searched for existing patterns before proposing new ones (`openspec/specs`, ROADMAP, design notes, store/brief/scan/config code)
- [x] I checked current filesystem state for counts, paths, and names (`openspec/specs`, `internal`, `cmd`)
- [x] Blast radius listed if shared code is touched (store, app DTO mapping, brief, scan, config, CLI/app tests)
- [x] Edge cases documented for every integration point and data transformation

## Pre-Mortem

1. Production briefs still rank poorly because identifier weighting is applied after FTS matching (`internal/store/search.go:67-83`) and stopword stripping removes too much context. Mitigation: use the frozen small stopword list, test stopword-only rejection and identifier weighting separately, and use OR within identifier component groups to avoid over-constraining FTS recall.
2. Incremental scan causes stale rows or unnecessary FTS churn because unchanged files bypass parsing while `DeleteStaleNotesTx` deletes rows not seen in the current run (`internal/store/notes.go`, `internal/app/scan.go:55-59`) and current FTS triggers run on any update (`internal/store/db.go:91-96`). Mitigation: implement mark-seen helpers that update `seen_in_scan_id` and `scanned_at`, narrow FTS update triggers or use a visibility side table, always parse Granola files, skip stale cleanup for date-sliced scans, and test unchanged/deleted/date-filtered/failed scans.
3. Snippet excerpts are empty or misleading because `notes_fts` indexes only selected columns (`internal/store/db.go:91-96`) and not all metadata. Mitigation: per-classification excerpts prefer structured fields first, and FTS snippets are fallback/meeting/knowledge-specific.
