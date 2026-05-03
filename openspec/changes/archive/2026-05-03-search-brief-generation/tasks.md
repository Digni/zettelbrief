## 1. Store Search Foundation

- [x] 1.1 Add store search request/result types carrying project, optional repo/type/fetch-date filters, query tokens, 250-candidate limit, note metadata, source path, section ID, row ID, and optional FTS rank.
- [x] 1.2 Implement FTS query tokenization/sanitization that rejects empty searchable queries and handles quotes, punctuation, SQL metacharacters, and FTS metacharacters without raw MATCH interpolation.
- [x] 1.3 Ensure searchable text is logical-note-specific, including a regression test proving a query term in one daily-work section does not return sibling sections from the same source file.
- [x] 1.4 Implement parameterized note retrieval requiring project and combining optional type/date filters plus repo semantics of `repo = requested OR repo empty/null`, excluding other non-empty repo values.
- [x] 1.5 Add store unit tests for safe FTS input, empty sanitized queries, required project validation, repo semantics, date-boundary/null-date behavior, combined filters, and returned citation/routing fields.
- [x] 1.6 Verification checkpoint: run `go test ./internal/store` and confirm all store tests pass.

## 2. Fetch Scoring and Brief Composition

- [x] 2.1 Add deterministic scoring helpers for keyword density, section-specific note-type weighting, 15-entry section caps, and tie-breaking by score desc, source path asc, section ID asc, row ID asc; do not add recency or confidence markers.
- [x] 2.2 Add brief section routing using concrete predicates: daily/knowledge prior work, meeting context, explicit decision/constraint text, daily Notes or blocker/risk/issue headings, and pending/todo/open-question headings or daily Notes with `?`/`TODO`.
- [x] 2.3 Add Markdown rendering for `brief.md` with all fixed sections, explicit empty-section text, per-entry source markers, and a de-duplicated Sources section.
- [x] 2.4 Add `sources.json` generation mapping every rendered entry to source path, row ID, section ID, section name, and available metadata.
- [x] 2.5 Add unit tests for scoring order, type weighting, tie-breaking, repo-filtered daily prior work, routing predicates, nullable metadata rendering, empty results, and entry-to-source mapping completeness.
- [x] 2.6 Verification checkpoint: run `go test ./internal/app ./internal/store` plus any new brief package tests and confirm all pass.

## 3. Fetch Application Workflow and Output Writing

- [x] 3.1 Implement an app-level fetch workflow that performs fetch-specific project/config validation, checks that `.zettelbrief/zettelbrief.db` exists, calls store search, scores/routes results, and writes output files.
- [x] 3.2 Implement UTC timestamped `.zettelbrief/briefs/<YYYY-MM-DDTHH-mm-ssZ>/` directory creation with testable clock/output-root injection, RFC3339 UTC `generated_at` in `sources.json`, and restrictive local permissions (`0700` directories, `0600` files where supported).
- [x] 3.3 Validate fetch-only `--since` and `--until` date filters before searching, reject invalid dates and reversed ranges, and do not add scan date flags in this change.
- [x] 3.4 Add workflow tests for successful output creation, no-match output, missing database error, invalid/reversed date errors, invalid type error, fetch-specific validation, and no partial output on validation failures.
- [x] 3.5 Verification checkpoint: run `go test ./internal/app` and confirm fetch workflow tests pass.

## 4. CLI Integration

- [x] 4.1 Add `newFetchCommand` to the Cobra root with required query argument, required `--project`, and optional `--repo`, `--type`, `--since`, and `--until` flags.
- [x] 4.2 Ensure CLI errors are clear and do not print usage noise beyond existing command conventions.
- [x] 4.3 Print the created brief directory path on successful fetch.
- [x] 4.4 Extend end-to-end CLI tests to build the binary, scan fixture notes, run `fetch`, assert `brief.md` and `sources.json` exist, assert section headings and cited source paths, and cover required query/project/date validation.
- [x] 4.5 Verification checkpoint: run `go test ./...` and confirm all package and e2e tests pass.

## 5. Final Validation

- [x] 5.1 Run `openspec validate search-brief-generation --strict` and confirm the change is valid.
- [x] 5.2 Run `openspec status --change search-brief-generation` and confirm the change remains apply-ready while tasks reflect implementation progress.
- [x] 5.3 Run `go test ./...` from a clean state and confirm no regressions to scan/status behavior.
- [x] 5.4 Manually exercise a fixture-backed fetch command and inspect the generated `brief.md` and `sources.json` for section order, source-backed entries, repo/date filtering, and no uncited claims.
