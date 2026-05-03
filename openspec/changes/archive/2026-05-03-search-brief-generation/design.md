## Context

Change 1 established the local ingestion baseline: the Cobra root currently registers `init`, `scan`, and `status`, but no `fetch` command (`cmd/zettelbrief/main.go:26-29`). Scanning writes logical notes into `.zettelbrief/zettelbrief.db` using an existing `notes` table with project, type, repo, date, title, summary, notes text, source path, content, tags, and metadata columns (`internal/store/db.go:55-64`; `internal/models/note.go:18-40`). The database already maintains a FTS5 table over `title`, `summary`, `content`, and `tags` via triggers (`internal/store/db.go:60-63`), and write paths use parameterized SQL (`internal/store/notes.go:17-39`). The README and roadmap define the target user flow as `zettelbrief fetch --project VetZ --repo One.Backend "query"` writing `.zettelbrief/briefs/<timestamp>/{brief.md,sources.json}` (`README.md:24-36`; `ROADMAP.md:57-79`).

## Goals / Non-Goals

**Goals:**
- Add a `fetch` command that validates fetch-specific inputs, opens the existing database, executes safe full-text and metadata-filtered note retrieval, and writes a timestamped local brief directory.
- Keep retrieval deterministic and testable: tokenize/escape the user query, use parameterized metadata filters, search note-specific text, and score by keyword match density plus note-type weighting.
- Compose `brief.md` in the README-defined section shape and `sources.json` as an audit trail from each rendered entry to a logical note row.
- Preserve Change 1 scan/status behavior while fixing daily-work search granularity if implementation confirms rows currently index full-file content.

**Non-Goals:**
- No excerpt context windows, recency-weighted scoring, confidence markers, incremental scan, embeddings, MCP server, chat interface, JSON-only output mode, or scan `--since`/`--until` flags.
- No broad schema redesign; add only the minimal migration/indexing or read-time normalization required for note-specific search.

## Decisions

1. **Add an app-level fetch workflow with store and composer subpackages.**
   - Decision: implement `app.RunFetch` (or equivalent) that orchestrates config validation, DB querying, scoring, routing, and output writing, with lower-level query helpers in `internal/store` and brief rendering in `internal/brief` or `internal/app`.
   - Rationale: `cmd/zettelbrief/main.go:69-118` keeps `scan` command logic thin and delegates work to `internal/app/scan.go`; matching that pattern keeps CLI parsing separate from business logic.
   - Alternative considered: put search and composition directly in the Cobra command. Rejected because e2e coverage would be possible, but unit testing scoring and routing would be harder.

2. **Use FTS5 `MATCH ?` only with a sanitized generated query string, and keep all filters parameterized.**
   - Decision: convert the raw user query into a conservative FTS expression by extracting non-empty word tokens, escaping double quotes, quoting each token/phrase fragment, and joining terms with spaces/AND according to SQLite FTS5 syntax supported by `modernc.org/sqlite`. Metadata filters (`project`, `repo`, `type`, `date`) remain SQL parameters.
   - Rationale: the existing schema creates `notes_fts` (`internal/store/db.go:60`) and the roadmap explicitly requires escaped/tokenized user query handling rather than raw MATCH interpolation (`ROADMAP.md:63-65`).
   - Daily-work granularity: `internal/app/scan.go:161-172` currently builds daily rows from parsed sections but passes full file content into each row. Fetch must not let terms from one section match sibling sections. Prefer storing/indexing daily-work section content as the row content; if a migration is too invasive, the query layer must post-filter FTS candidates against section-specific title/summary/verification/notes/repo/branch/ticket text before scoring or citation.
   - Alternative considered: `LIKE` search over content. Rejected because it would ignore the FTS5 index and roadmap requirement.

3. **Represent retrieved notes separately from scanned notes.**
   - Decision: introduce a query result model containing the stored note fields plus computed search metrics (rank, keyword hit count/density, type weight, final score). Do not overload `models.Note`, which represents scanned logical notes and stores fields such as `ContentHash` and `SeenInScanID` (`internal/models/note.go:18-40`).
   - Rationale: fetch scoring is read-only and output-oriented; keeping it separate avoids coupling ingestion data to retrieval presentation.
   - Alternative considered: extend `models.Note` with score fields. Rejected to avoid polluting scan/storage semantics.

4. **Score deterministically with keyword density plus type weights, not recency.**
   - Decision: base score on matched query tokens divided by searchable token count, then add section-specific type weights. Use stable tie-breaking by score descending, source path ascending, section ID ascending, then row ID ascending. Repo filters exclude other repo-specific rows; when `--repo` is supplied, daily-work prior-work entries must match that repo, while project-level rows with no repo remain eligible.
   - Rationale: Change 2 includes keyword density and note-type weighting (`ROADMAP.md:63-67`) while recency belongs to Change 3 (`ROADMAP.md:90-94`).
   - Alternative considered: use SQLite `bm25(notes_fts)` alone. Rejected because the roadmap calls for explicit type weighting and section routing; bm25 can still be an internal input but not the sole ordering contract.

5. **Write brief outputs under the existing runtime directory root.**
   - Decision: create `.zettelbrief/briefs/<timestamp>/brief.md` and `sources.json`, with restrictive permissions consistent with DB privacy (`internal/store/db.go:21-45`). Directory timestamps use UTC in filesystem-safe ISO form `2006-01-02T15-04-05Z`; `sources.json` includes exact RFC3339 UTC `generated_at` metadata such as `2026-05-01T06:42:13Z`.
   - Rationale: README and roadmap both specify `.zettelbrief/briefs/<timestamp>/` (`README.md:30-36`; `ROADMAP.md:59-61`), and UTC avoids local timezone ambiguity while remaining sortable and portable as a directory name.
   - Alternative considered: print the brief to stdout. Rejected because the required output is file-based and source-auditable.

6. **Route notes into fixed sections before rendering.**
   - Decision: map note types/fields to the five README sections: Relevant Prior Work, Meeting Context, Decisions And Constraints, Risks For This Task, and Open Questions. Keep routing narrow and testable: decisions/constraints require headings or lines containing `decision`, `constraint`, or `decided`; risks use daily-work `Notes` or project-state headings containing `blocker`, `risk`, or `issue`; open questions use project-state headings containing `pending`, `todo`, or `open question`, or daily-work `Notes` containing `?`/`TODO`. Include a final Sources section in `brief.md` and row-level details in `sources.json`.
   - Rationale: the brief shape is fixed in `README.md:38-65`, and the roadmap gives section feeds (`ROADMAP.md:67-79`).
   - Alternative considered: one flat relevance list. Rejected because it loses the section-level orientation the brief is supposed to provide.

## Risks / Trade-offs

- **FTS query syntax rejects edge-case user input** → Sanitize to a minimal quoted-token expression, test quotes/punctuation/FTS metacharacters, and fall back to a clear no-results/error path rather than interpolating raw MATCH strings.
- **Sparse or missing metadata causes empty sections** → Composer MUST tolerate null repo/date/title/summary fields from `models.Note` (`internal/models/note.go:23-38`) and render only source-backed entries; active date filters exclude undated notes, while absent filters keep nullable metadata eligible.
- **Scoring is useful but not semantically deep** → Keep deterministic keyword/type scoring for Change 2 and explicitly defer recency/confidence/excerpts/embeddings to Change 3/future.
- **Brief output could expose private note content** → Write only under local `.zettelbrief/` with restrictive permissions, mirroring database privacy handling (`internal/store/db.go:25-45`).
- **Shared store changes could affect scan/status** → Add query helpers without modifying existing write helpers; run `go test ./...` after each store/app chunk.

## Migration Plan

1. Verify daily-work search granularity against the existing `notes.content` behavior. If daily rows still store full-file content, update scanning/storage or add a minimal migration so indexed searchable text is logical-note-specific.
2. Implement query helpers as additive read APIs with the explicit repo/date semantics from the specs.
3. Implement composer/output writing as additive app logic.
4. Register the `fetch` command in the root command.
5. Rollback is deleting the new fetch/search/composer code and command registration; if a minimal index/content migration is added, keep it backward-compatible with existing scan/status database files.

## Resolved Sizing Decisions

- Timestamp format: output directories use UTC `2006-01-02T15-04-05Z`; `sources.json` uses RFC3339 UTC `generated_at`.
- Result bounds: fetch retrieves up to 250 candidate notes after FTS and metadata filters, then renders up to 15 entries per brief section. No public `--limit` flag is added in Change 2; revisit configurability in Change 3 only if real briefs are too noisy or too sparse.

## Planning Verification

- [x] Every file/line reference was read directly by me (not from subagent summary alone)
- [x] I ran diagnostic commands myself for facts in the plan (`go test ./...` passed)
- [x] Each step has a verification checkpoint with concrete command and expected outcome (see `tasks.md`)
- [x] I searched for existing patterns before proposing new ones (`cmd/zettelbrief/main.go`, `internal/app/scan.go`, `internal/store/*.go`, existing specs)
- [x] I checked current filesystem state for counts, paths, and names (`openspec/specs`, Go files, roadmap)
- [x] Blast radius listed if shared code is touched (CLI, app, store, models/output)
- [x] Edge cases documented for every integration point and data transformation

## Pre-Mortem

1. **Production failure: FTS query errors on punctuation-heavy task text.** This would happen if the implementation bypasses the planned tokenization/quoting and passes raw user input into `MATCH`; tests must include quotes, colons, dashes, and empty-token input.
2. **Production failure: daily-work citations point at the wrong section.** This would happen if full-file content remains indexed for every daily-work row; tests must prove a term in one daily-work section does not return sibling sections.
3. **Production failure: generated briefs are hard to audit.** If `sources.json` only lists unique files and not entry-to-row mappings, it will not satisfy the source-backed intent in `README.md:40-65`; composer tests must assert every rendered entry has source path, row ID, section ID, and section name.
