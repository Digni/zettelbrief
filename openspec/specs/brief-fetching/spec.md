# brief-fetching Specification

## Purpose
TBD - created by archiving change search-brief-generation. Update Purpose after archive.
## Requirements
### Requirement: Fetch command creates briefing outputs
The system SHALL provide a `fetch` CLI command that accepts one required query argument and metadata filters for project, repo, note type, and fetch-only date range. A successful fetch SHALL write a UTC timestamped output directory containing `brief.md` and `sources.json` under `.zettelbrief/briefs/`. Date range flags on `scan` are out of scope for this change.

#### Scenario: Fetch writes brief and sources files
- **WHEN** `zettelbrief fetch --project Acme --repo One.Backend "billable service persistence"` is run after Acme has been scanned
- **THEN** the system creates `.zettelbrief/briefs/<utc-timestamp>/brief.md` where `<utc-timestamp>` uses `YYYY-MM-DDTHH-mm-ssZ`
- **AND** the system creates `.zettelbrief/briefs/<utc-timestamp>/sources.json`
- **AND** `sources.json` includes a RFC3339 UTC `generated_at` value
- **AND** the command output includes the created brief directory path

#### Scenario: Fetch requires a query
- **WHEN** `zettelbrief fetch --project Acme` is run without a query argument
- **THEN** the command fails with a clear validation error
- **AND** no brief directory is created

#### Scenario: Fetch requires a project filter
- **WHEN** `zettelbrief fetch "billable service persistence"` is run without `--project`
- **THEN** the command fails with a clear validation error
- **AND** no brief directory is created

#### Scenario: Fetch validates note type filter
- **WHEN** `zettelbrief fetch --project Acme --type unsupported "billable service persistence"` is run
- **THEN** the command fails with a clear validation error listing supported note types
- **AND** no brief directory is created

### Requirement: Fetch applies metadata filters
The system SHALL filter fetched notes by project, optional repo, optional note type, and optional inclusive fetch date range before composing the brief. The repo filter SHALL exclude notes with a different non-empty repo value, while allowing project-level notes with no repo value to remain eligible.

#### Scenario: Project filter limits results
- **WHEN** Acme and Flive notes both contain the query terms
- **AND** `zettelbrief fetch --project Acme "shared query"` is run
- **THEN** generated brief entries only cite Acme notes

#### Scenario: Repo filter excludes other repo-specific notes but keeps project context
- **WHEN** scanned notes include daily work for repos `One.Backend` and `One.Frontend`
- **AND** matching knowledge, meeting, or project state notes have no repo value
- **AND** `zettelbrief fetch --project Acme --repo One.Backend "persistence"` is run
- **THEN** generated entries do not cite `One.Frontend` rows
- **AND** generated entries may cite matching `One.Backend` rows
- **AND** generated entries may cite matching project-level rows with no repo value

#### Scenario: Repo filter requires daily-work prior-work match
- **WHEN** `zettelbrief fetch --project Acme --repo One.Backend "persistence"` is run
- **THEN** `daily_work` entries under `Relevant Prior Work` cite only rows whose repo is `One.Backend`

#### Scenario: No repo filter allows matching daily work
- **WHEN** `zettelbrief fetch --project Acme "persistence"` is run without `--repo`
- **THEN** matching `daily_work` entries may appear under `Relevant Prior Work` regardless of repo value

#### Scenario: Type filter limits note types
- **WHEN** matching notes exist for `daily_work` and `meeting`
- **AND** `zettelbrief fetch --project Acme --type meeting "planning"` is run
- **THEN** generated brief entries only cite meeting notes

#### Scenario: Date range filter is inclusive
- **WHEN** matching notes have dates `2026-04-01`, `2026-04-15`, and `2026-05-01`
- **AND** `zettelbrief fetch --project Acme --since 2026-04-01 --until 2026-04-15 "planning"` is run
- **THEN** generated brief entries SHALL cite eligible matching notes dated `2026-04-01` and `2026-04-15`
- **AND** generated brief entries SHALL NOT cite the `2026-05-01` note

#### Scenario: Date filter excludes undated notes
- **WHEN** a matching note has no date value
- **AND** `zettelbrief fetch --project Acme --since 2026-04-01 "planning"` is run
- **THEN** the undated note is not eligible for generated brief entries

#### Scenario: Invalid until date is rejected
- **WHEN** `zettelbrief fetch --project Acme --until not-a-date "planning"` is run
- **THEN** the command fails with a clear date validation error
- **AND** no brief directory is created

#### Scenario: Reversed date range is rejected
- **WHEN** `zettelbrief fetch --project Acme --since 2026-05-01 --until 2026-04-01 "planning"` is run
- **THEN** the command fails with a clear date range validation error
- **AND** no brief directory is created

### Requirement: Fetch ranks and limits results deterministically
The system SHALL retrieve up to 250 candidate notes after FTS and metadata filtering, then assign deterministic relevance scores using keyword match density, identifier-aware query weighting, note-type weighting, and type-conditional recency. Identifier-aware query weighting SHALL classify dotted, snake_case, kebab-case, slash/path-like, and camelCase raw query spans before lowercasing as identifiers, normalize them into FTS-compatible component tokens, OR component tokens within the same identifier group for FTS recall, and keep their identifier group for scoring. Recency SHALL decay `daily_work` and `meeting` scores using UTC today and `max(0.3, 1 - max(age_days, 0)/180)` and SHALL NOT decay `project_state` or `knowledge` scores. Ordering SHALL sort by final score descending, then source path ascending, then section ID ascending, then note row ID ascending. The composer SHALL render up to 15 entries per brief section.

#### Scenario: Higher keyword density ranks first within same type
- **WHEN** two matching notes of the same type are eligible for the same brief section
- **AND** one note has a higher keyword density
- **THEN** the higher-density note appears before the lower-density note in that section

#### Scenario: Note type weighting affects section order
- **WHEN** matching notes of different types have equal keyword density and are eligible for the same brief section
- **THEN** the note with the higher section-specific note-type weight appears first

#### Scenario: Stable tie-breaking
- **WHEN** two matching notes have the same final relevance score
- **THEN** ordering is stable across runs by source path, section ID, and note row ID

#### Scenario: Section rendering is bounded
- **WHEN** more than 15 eligible entries match a brief section
- **THEN** the section renders the top 15 entries after deterministic ordering
- **AND** lower-ranked entries for that section are omitted from `brief.md`

#### Scenario: Future-dated note does not exceed recency factor one
- **WHEN** a `daily_work` or `meeting` note has a future date due to clock skew or metadata error
- **THEN** the recency calculation treats age as zero
- **AND** the recency factor is no greater than `1.0`

#### Scenario: Daily work recency affects ranking
- **WHEN** two otherwise equal `daily_work` notes have dates 10 days old and 200 days old
- **THEN** the newer note receives a higher final relevance score
- **AND** the older note's recency factor is no lower than `0.3`

#### Scenario: Project state is not recency-decayed
- **WHEN** a matching `project_state` note is older than a matching daily work note
- **THEN** the `project_state` note keeps a recency factor of `1.0`

#### Scenario: Dotted identifier terms receive higher signal
- **WHEN** a query contains a dotted identifier-like term such as `One.Backend`
- **THEN** the term is normalized into FTS-compatible component tokens such as `one` and `backend`
- **AND** matches on the identifier group contribute more to relevance than matches on ordinary task-framing words

#### Scenario: CamelCase identifier terms receive higher signal
- **WHEN** a query contains a camelCase identifier-like term such as `billableService`
- **THEN** the term remains searchable with FTS-compatible tokens
- **AND** matches on that identifier contribute more to relevance than matches on ordinary task-framing words

### Requirement: Brief composer routes entries into fixed sections
The system SHALL compose `brief.md` with the fixed sections `Relevant Prior Work`, `Meeting Context`, `Decisions And Constraints`, `Risks For This Task`, `Open Questions`, and `Sources`. Entries SHALL be routed according to note type and concrete metadata/content predicates.

#### Scenario: Daily work and knowledge feed prior work
- **WHEN** matching `daily_work` or `knowledge` notes are fetched
- **AND** the daily work row satisfies the repo filter rules when `--repo` is supplied
- **THEN** eligible entries appear under `Relevant Prior Work`

#### Scenario: Meeting notes feed meeting context
- **WHEN** matching `meeting` notes are fetched
- **THEN** eligible entries appear under `Meeting Context`

#### Scenario: Decisions section includes explicit decision or constraint text
- **WHEN** matching `meeting`, `project_state`, or `daily_work` notes contain a heading or line containing `decision`, `constraint`, or `decided`
- **THEN** eligible entries appear under `Decisions And Constraints`

#### Scenario: Risks section includes blockers and notes
- **WHEN** matching daily work notes have non-empty `Notes` metadata
- **OR** matching project state content contains a heading with `blocker`, `risk`, or `issue`
- **THEN** eligible entries appear under `Risks For This Task`

#### Scenario: Open questions section includes pending work
- **WHEN** matching project state content contains a heading with `pending`, `todo`, or `open question`
- **OR** matching daily work notes have `Notes` metadata containing `?` or `TODO`
- **THEN** eligible entries appear under `Open Questions`

#### Scenario: Empty section remains explicit
- **WHEN** no fetched notes are eligible for a brief section
- **THEN** `brief.md` still includes that section
- **AND** the section states that no matching sources were found

### Requirement: Brief entries are source-backed
The system SHALL make every generated brief entry traceable to exactly one logical note row. `sources.json` SHALL map every generated entry to source path, note identifiers, classification, confidence, match reason, excerpt, final score, and recency factor. When unambiguous character offsets are available, `sources.json` SHALL include `char_offset_start` and `char_offset_end`; otherwise those fields SHALL be omitted rather than serialized as null. `brief.md` SHALL include a human-readable Sources section.

#### Scenario: Every rendered entry has a source mapping
- **WHEN** `brief.md` contains generated source-backed entries
- **THEN** `sources.json` contains a corresponding mapping for each entry
- **AND** each mapping includes the vault-relative `source_path`
- **AND** each mapping includes the note row ID
- **AND** each mapping includes the note `section_id`
- **AND** each mapping includes the brief section name
- **AND** each mapping includes `classification`, `confidence`, `match_reason`, `excerpt`, `score`, and `recency_factor`

#### Scenario: Sources section lists cited files
- **WHEN** a brief cites multiple note entries from the same vault file
- **THEN** the `Sources` section in `brief.md` lists the vault file path once
- **AND** `sources.json` preserves the individual entry mappings

#### Scenario: Character offsets are included when uniquely known
- **WHEN** an excerpt is derived from a unique substring found in the stored note content
- **THEN** the source mapping includes character offsets for that excerpt span

#### Scenario: Ambiguous character offsets are omitted
- **WHEN** an excerpt cannot be found or appears multiple times in stored note content
- **THEN** `char_offset_start` and `char_offset_end` are omitted from that source mapping

### Requirement: Brief outputs preserve local privacy
The system SHALL create brief output directories and files with restrictive local permissions where supported because `brief.md` and `sources.json` may contain private note content.

#### Scenario: Brief files are written privately
- **WHEN** fetch writes `.zettelbrief/briefs/<timestamp>/brief.md` and `sources.json`
- **THEN** the brief directory is created with mode `0700` where supported
- **AND** the files are created with mode `0600` where supported

### Requirement: Fetch handles missing data safely
The system SHALL handle empty search results, missing database files, nullable note metadata, and invalid dates with clear behavior and without panics.

#### Scenario: No matching notes
- **WHEN** `zettelbrief fetch --project Acme "query with no matches"` finds no notes
- **THEN** the command still writes `brief.md` and `sources.json`
- **AND** `brief.md` states that no matching sources were found in each content section
- **AND** `sources.json` contains an empty entry mapping

#### Scenario: Database has not been created
- **WHEN** `zettelbrief fetch --project Acme "planning"` is run before any scan has created `.zettelbrief/zettelbrief.db`
- **THEN** the command fails with a clear message instructing the user to run `zettelbrief scan --project Acme`
- **AND** no brief directory is created

#### Scenario: Invalid since date is rejected
- **WHEN** `zettelbrief fetch --project Acme --since not-a-date "planning"` is run
- **THEN** the command fails with a clear date validation error
- **AND** no brief directory is created

#### Scenario: Nullable metadata does not panic
- **WHEN** a matching note has no title, repo, branch, summary, or date
- **THEN** the note can still be scored, routed, cited, and rendered using available content and source path when no active filter excludes it

### Requirement: Brief entries include evidence excerpts
The system SHALL render each non-empty brief entry with a concise evidence excerpt selected by note classification. `daily_work` excerpts SHALL use the parsed `Summary` field and fall back to parsed `Notes`. `meeting` and `knowledge` excerpts SHALL use an FTS5 snippet around matched terms with a type-appropriate explicit FTS column and bounded token count. `project_state` excerpts SHALL use frontmatter summary when present, otherwise the first paragraph after the title. Excerpts SHALL be bounded for agent readability. If snippet generation returns empty text or fails, the system SHALL fall back to a bounded first-content excerpt rather than rendering unbounded raw content.

#### Scenario: Daily work summary excerpt
- **WHEN** a matching daily work row has `Summary: Fixed billable persistence`
- **THEN** the brief entry excerpt is `Fixed billable persistence`

#### Scenario: Daily work notes fallback excerpt
- **WHEN** a matching daily work row has no summary and has non-empty notes text
- **THEN** the brief entry excerpt is derived from the notes text

#### Scenario: Meeting snippet excerpt
- **WHEN** a matching meeting note contains multiple unrelated topics
- **THEN** the brief entry excerpt is a snippet around the matched query terms

#### Scenario: Snippet fallback remains bounded
- **WHEN** FTS5 snippet generation returns empty text or fails for a matching note
- **THEN** the brief entry excerpt falls back to a bounded first-content excerpt
- **AND** the full raw content is not rendered as entry text

#### Scenario: Project state lede excerpt
- **WHEN** a matching project state note has no frontmatter summary
- **THEN** the brief entry excerpt is the first paragraph after the document title

### Requirement: Brief entries include confidence and match reason
The system SHALL assign each brief entry a confidence marker and machine-readable match reason tied to evidence. HIGH confidence SHALL represent a direct repo filter matching row repo metadata. MEDIUM confidence SHALL represent project-level matching without a direct repo match. LOW confidence SHALL represent keyword-only matching. Branch metadata MAY appear in match reasons or excerpts when query terms match it, but branch matches alone SHALL NOT raise confidence to HIGH because scan/fetch has no branch filter. The match reason SHALL identify the reason, such as `repo:<name>`, `project:<name>`, `branch:<name>`, or `keyword:<term>`.

#### Scenario: Direct repo match is high confidence
- **WHEN** fetch runs with `--repo One.Backend`
- **AND** a cited daily-work row has repo `One.Backend`
- **THEN** the entry has confidence `HIGH`
- **AND** its match reason includes `repo:One.Backend`

#### Scenario: Project-level note is medium confidence
- **WHEN** fetch runs for project `Acme`
- **AND** a cited project-level note has no repo value
- **THEN** the entry has confidence `MEDIUM`
- **AND** its match reason identifies the project-level match

#### Scenario: Daily work without repo is not routed as repo-specific prior work
- **WHEN** fetch runs with `--repo One.Backend`
- **AND** a matching daily-work row has no repo value
- **THEN** the row is not eligible for `Relevant Prior Work` through the daily-work repo-specific routing rule
- **AND** if another routing rule includes the row as project-level context, the entry does not receive confidence `HIGH`
- **AND** any included entry has project-level or keyword-only confidence based on the evidence that routed it

#### Scenario: Branch query match does not become high confidence
- **WHEN** a query term matches a cited row's branch metadata
- **AND** no repo filter directly matches the row repo
- **THEN** the entry does not receive confidence `HIGH` from the branch match alone
- **AND** its match reason may include `branch:<name>`

#### Scenario: Keyword-only match is low confidence
- **WHEN** a cited entry is included only because a query keyword matched its text
- **THEN** the entry has confidence `LOW`
- **AND** its match reason includes the matched keyword

### Requirement: Fetch normalizes task-shaped queries
The system SHALL normalize fetch queries by removing the deterministic stopword set `a`, `an`, `and`, `add`, `change`, `do`, `fix`, `for`, `in`, `of`, `on`, `the`, `to`, `update`, `with`, `work` before building the FTS query and by preserving identifier-like raw query spans as high-signal scoring groups. If no searchable terms remain after normalization, fetch SHALL fail with a clear application-layer validation error and SHALL NOT create a brief directory.

#### Scenario: Stopwords are removed before search
- **WHEN** the user searches for `fix the billable service update persistence`
- **THEN** task-framing words such as `fix`, `the`, and `update` do not contribute search terms
- **AND** signal terms such as `billable`, `service`, and `persistence` remain searchable

#### Scenario: Stopword-only query is rejected
- **WHEN** the user searches for `fix add update the`
- **THEN** fetch fails with a clear validation error
- **AND** no brief directory is created

#### Scenario: Identifier components are ORed within group
- **WHEN** the query contains identifier `One.Backend`
- **THEN** FTS matching does not require every component of the identifier group to be present in a row
- **AND** scoring still treats matches to the identifier group as high-signal evidence

