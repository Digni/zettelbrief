# zettelbrief Roadmap

Three sequential changes that build on each other. Each produces a usable, testable increment.

```
Change 1:            Change 2:            Change 3:
Foundation+Ingest    Search+Generate      Quality+Polish

┌──────────┐         ┌──────────┐         ┌──────────┐
│ scan     │────────▶│   FTS5   │         │ scoring  │
│ config   │         │  search  │─┐       │ excerpts │
│ classify │         │ filters  │ │       │ increment│
│ extract  │         └──────────┘ │       │ confid.  │
│ db store │                      │       └──────────┘
│ status   │         ┌──────────┐ │
└──────────┘         │ generate │◀┘
                      │  brief   │
                      │ sources  │
                      └──────────┘
```

---

## Change 1: Foundation + Ingest

**Goal**: Turn the vault into a queryable SQLite database.

**Output**: Running `zettelbrief scan --project VetZ` populates a local SQLite database with classified, metadata-tagged notes.

**In scope**:
- Go module initialization, Cobra CLI scaffold (`scan`, `status` commands)
- Config system: global `~/.config/zettelbrief/config.yaml` + discovered project `.zettelbrief/config.yaml` overrides
  - `vault_path`, `projects: { name: { folders: [...], aliases: [...] } }`
  - strict vault-relative path validation; no absolute paths, `..` traversal, or symlink escapes
- Vault scanner: walks configured project folders + `4.Granola/` for matching frontmatter
- Note classifier (passive, normalized vault-relative path + frontmatter-based):
  - `daily_work` — path contains `1. Daily Work/`, one logical record per `##` section with `- Repo:`, `- Branch:` bullet structure
  - `meeting` — path is `4.Granola/`, frontmatter with `granola_id`, `title`, `folders:`, one logical record per matched project
  - `project_state` — filename `State.md`, frontmatter `tags: [state]`
  - `knowledge` — everything else, first heading as title; raw frontmatter `type` like `decision`/`plan` preserved for future routing
- Type-specific metadata extractors:
  - **Daily work parser**: regex-based `- Repo:`, `- Branch:`, `- Summary:`, `- Verification:`, `- Notes:` per `## Section`
  - **Meeting parser**: frontmatter `granola_id`, `title`, `created`, `updated`, `folders:`, sections + action items
  - **Generic extractor**: first heading → title, frontmatter tags if present
- Project resolution:
  - Path-based for `1.Projects/<Name>/` notes
  - Normalized frontmatter `folders:` field for Granola notes, matched against config project names and aliases
- SQLite schema: `notes` table with `UNIQUE(project, source_path, section_id)`, `scan_runs`, `schema_migrations`, and FTS5 virtual table
- Successful full scans remove stale rows for deleted/renamed files or removed daily-work sections
- `status` command showing scan freshness, latest failed scan, and note counts for every configured project including never-scanned projects

**Out of scope**:
- Search, brief generation, incremental scan, relevance scoring

---

## Change 2: Search + Brief Generation

**Goal**: Turn `zettelbrief fetch "query"` into real output files.

**Output**: `.zettelbrief/briefs/<timestamp>/{brief.md, sources.json}`

**In scope**:
- FTS5 full-text search over note content + metadata columns, with escaped/tokenized user query handling rather than raw MATCH interpolation
- Metadata filters: project, repo, date range, note type
- Relevance scoring: keyword match density + note type weighting
- Brief composer with section routing:

  | Brief Section | Feeds from |
  |--------------|-----------|
  | Relevant Prior Work | daily_work (repo match), knowledge |
  | Meeting Context | meeting (project match) |
  | Decisions And Constraints | meeting, state, daily_work |
  | Risks For This Task | daily_work (Notes field), state (Blockers section) |
  | Open Questions | state (Pending Todos section), daily_work |

- Section-level ordering within brief
- `brief.md` in the README-defined format
- `sources.json` mapping brief entries to vault file paths

**Out of scope**:
- Excerpt generation with context windows, recency scoring, confidence markers, incremental scan

---

## Change 3: Quality + Polish

**Goal**: Make briefs good, not just correct.

**In scope**:
- Incremental scan: skip files whose content hash hasn't changed since last scan
- Recency-weighted relevance: newer notes score higher in searches
- Snippet generation: excerpts with surrounding sentence/paragraph context
- Confidence markers per brief entry: HIGH (direct repo/branch match), MEDIUM (project match), LOW (keyword-only)
- Date-range slicing: `--since`, `--until` flags on `fetch` and `scan`
- Config validation with graceful errors for missing vaults, empty project folders, malformed notes
- Full test coverage across all three commands

**Out of scope**:
- Embedding-based semantic search (future, per README)
- MCP server, chat interface, any agent-facing API

---

## Future (Post-v1)

- Embedding-based semantic search (step 6 from README)
- `zettelbrief watch` — auto-rescan on vault file changes
- Multi-project briefs: cross-reference knowledge across related projects
- Output format options (JSON-only for programmatic consumption)
