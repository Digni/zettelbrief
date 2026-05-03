# brief-fetching Specification

## Purpose
TBD - created by archiving change search-brief-generation. Update Purpose after archive.
## Requirements
### Requirement: Fetch command creates briefing outputs
The system SHALL provide a `fetch` CLI command that accepts one required query argument and metadata filters for project, repo, note type, and fetch-only date range. A successful fetch SHALL write a UTC timestamped output directory containing `brief.md` and `sources.json` under `.zettelbrief/briefs/`. Date range flags on `scan` are out of scope for this change.

#### Scenario: Fetch writes brief and sources files
- **WHEN** `zettelbrief fetch --project VetZ --repo One.Backend "billable service persistence"` is run after VetZ has been scanned
- **THEN** the system creates `.zettelbrief/briefs/<utc-timestamp>/brief.md` where `<utc-timestamp>` uses `YYYY-MM-DDTHH-mm-ssZ`
- **AND** the system creates `.zettelbrief/briefs/<utc-timestamp>/sources.json`
- **AND** `sources.json` includes a RFC3339 UTC `generated_at` value
- **AND** the command output includes the created brief directory path

#### Scenario: Fetch requires a query
- **WHEN** `zettelbrief fetch --project VetZ` is run without a query argument
- **THEN** the command fails with a clear validation error
- **AND** no brief directory is created

#### Scenario: Fetch requires a project filter
- **WHEN** `zettelbrief fetch "billable service persistence"` is run without `--project`
- **THEN** the command fails with a clear validation error
- **AND** no brief directory is created

#### Scenario: Fetch validates note type filter
- **WHEN** `zettelbrief fetch --project VetZ --type unsupported "billable service persistence"` is run
- **THEN** the command fails with a clear validation error listing supported note types
- **AND** no brief directory is created

### Requirement: Fetch applies metadata filters
The system SHALL filter fetched notes by project, optional repo, optional note type, and optional inclusive fetch date range before composing the brief. The repo filter SHALL exclude notes with a different non-empty repo value, while allowing project-level notes with no repo value to remain eligible.

#### Scenario: Project filter limits results
- **WHEN** VetZ and Flive notes both contain the query terms
- **AND** `zettelbrief fetch --project VetZ "shared query"` is run
- **THEN** generated brief entries only cite VetZ notes

#### Scenario: Repo filter excludes other repo-specific notes but keeps project context
- **WHEN** scanned notes include daily work for repos `One.Backend` and `One.Frontend`
- **AND** matching knowledge, meeting, or project state notes have no repo value
- **AND** `zettelbrief fetch --project VetZ --repo One.Backend "persistence"` is run
- **THEN** generated entries do not cite `One.Frontend` rows
- **AND** generated entries may cite matching `One.Backend` rows
- **AND** generated entries may cite matching project-level rows with no repo value

#### Scenario: Repo filter requires daily-work prior-work match
- **WHEN** `zettelbrief fetch --project VetZ --repo One.Backend "persistence"` is run
- **THEN** `daily_work` entries under `Relevant Prior Work` cite only rows whose repo is `One.Backend`

#### Scenario: No repo filter allows matching daily work
- **WHEN** `zettelbrief fetch --project VetZ "persistence"` is run without `--repo`
- **THEN** matching `daily_work` entries may appear under `Relevant Prior Work` regardless of repo value

#### Scenario: Type filter limits note types
- **WHEN** matching notes exist for `daily_work` and `meeting`
- **AND** `zettelbrief fetch --project VetZ --type meeting "planning"` is run
- **THEN** generated brief entries only cite meeting notes

#### Scenario: Date range filter is inclusive
- **WHEN** matching notes have dates `2026-04-01`, `2026-04-15`, and `2026-05-01`
- **AND** `zettelbrief fetch --project VetZ --since 2026-04-01 --until 2026-04-15 "planning"` is run
- **THEN** generated brief entries SHALL cite eligible matching notes dated `2026-04-01` and `2026-04-15`
- **AND** generated brief entries SHALL NOT cite the `2026-05-01` note

#### Scenario: Date filter excludes undated notes
- **WHEN** a matching note has no date value
- **AND** `zettelbrief fetch --project VetZ --since 2026-04-01 "planning"` is run
- **THEN** the undated note is not eligible for generated brief entries

#### Scenario: Invalid until date is rejected
- **WHEN** `zettelbrief fetch --project VetZ --until not-a-date "planning"` is run
- **THEN** the command fails with a clear date validation error
- **AND** no brief directory is created

#### Scenario: Reversed date range is rejected
- **WHEN** `zettelbrief fetch --project VetZ --since 2026-05-01 --until 2026-04-01 "planning"` is run
- **THEN** the command fails with a clear date range validation error
- **AND** no brief directory is created

### Requirement: Fetch ranks and limits results deterministically
The system SHALL retrieve up to 250 candidate notes after FTS and metadata filtering, then assign deterministic relevance scores using keyword match density, note-type weighting, and stable tie-breaking. Recency weighting and confidence markers SHALL NOT be included in this change. Keyword density SHALL be calculated as matched query tokens divided by searchable token count for the note. Ordering SHALL sort by final score descending, then source path ascending, then section ID ascending, then note row ID ascending. The composer SHALL render up to 15 entries per brief section.

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
The system SHALL make every generated brief entry traceable to exactly one logical note row. `sources.json` SHALL map every generated entry to source path and note identifiers, and `brief.md` SHALL include a human-readable Sources section.

#### Scenario: Every rendered entry has a source mapping
- **WHEN** `brief.md` contains generated source-backed entries
- **THEN** `sources.json` contains a corresponding mapping for each entry
- **AND** each mapping includes the vault-relative `source_path`
- **AND** each mapping includes the note row ID
- **AND** each mapping includes the note `section_id`
- **AND** each mapping includes the brief section name

#### Scenario: Sources section lists cited files
- **WHEN** a brief cites multiple note entries from the same vault file
- **THEN** the `Sources` section in `brief.md` lists the vault file path once
- **AND** `sources.json` preserves the individual entry mappings

### Requirement: Brief outputs preserve local privacy
The system SHALL create brief output directories and files with restrictive local permissions where supported because `brief.md` and `sources.json` may contain private note content.

#### Scenario: Brief files are written privately
- **WHEN** fetch writes `.zettelbrief/briefs/<timestamp>/brief.md` and `sources.json`
- **THEN** the brief directory is created with mode `0700` where supported
- **AND** the files are created with mode `0600` where supported

### Requirement: Fetch handles missing data safely
The system SHALL handle empty search results, missing database files, nullable note metadata, and invalid dates with clear behavior and without panics.

#### Scenario: No matching notes
- **WHEN** `zettelbrief fetch --project VetZ "query with no matches"` finds no notes
- **THEN** the command still writes `brief.md` and `sources.json`
- **AND** `brief.md` states that no matching sources were found in each content section
- **AND** `sources.json` contains an empty entry mapping

#### Scenario: Database has not been created
- **WHEN** `zettelbrief fetch --project VetZ "planning"` is run before any scan has created `.zettelbrief/zettelbrief.db`
- **THEN** the command fails with a clear message instructing the user to run `zettelbrief scan --project VetZ`
- **AND** no brief directory is created

#### Scenario: Invalid since date is rejected
- **WHEN** `zettelbrief fetch --project VetZ --since not-a-date "planning"` is run
- **THEN** the command fails with a clear date validation error
- **AND** no brief directory is created

#### Scenario: Nullable metadata does not panic
- **WHEN** a matching note has no title, repo, branch, summary, or date
- **THEN** the note can still be scored, routed, cited, and rendered using available content and source path when no active filter excludes it

