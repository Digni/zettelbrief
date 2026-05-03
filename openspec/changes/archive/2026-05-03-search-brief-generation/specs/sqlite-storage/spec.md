## ADDED Requirements

### Requirement: Safe FTS note retrieval
The store layer SHALL support searching notes through the existing FTS5 index using sanitized/tokenized user query terms and parameterized SQL values. The implementation SHALL NOT interpolate raw user query text into SQL or FTS `MATCH` expressions. Searchable text for a logical note SHALL be note-specific: daily-work rows use their section title and extracted metadata fields; meeting, project state, and knowledge rows use title, summary, content, notes text, and tags. Daily-work rows SHALL NOT match solely because another section in the same source file contains a query term.

#### Scenario: Quoted query text does not break search
- **WHEN** the user searches for `"billable" service:update - persistence`
- **THEN** the store builds a valid FTS5 query from sanitized tokens
- **AND** the query executes without SQL or FTS syntax errors
- **AND** matching notes can be returned

#### Scenario: Empty sanitized query is rejected
- **WHEN** the user query contains no searchable tokens after sanitization
- **THEN** the store returns a clear validation error
- **AND** no SQL query is executed with an empty raw `MATCH` expression

#### Scenario: User query is not raw SQL
- **WHEN** the user query contains SQL metacharacters or FTS metacharacters
- **THEN** those characters cannot alter the SQL statement structure
- **AND** metadata filter values are passed as SQL parameters

#### Scenario: Daily-work search stays section-specific
- **WHEN** one daily-work file contains two logical sections
- **AND** only the second section contains `billable persistence`
- **THEN** a search for `billable persistence` returns the second section
- **AND** the search does not return the first section solely because it shares the same source file content

### Requirement: Metadata-filtered note lookup
The store layer SHALL support retrieval of matching note rows filtered by project and optional repo, note type, and inclusive date bounds using the columns already present in the `notes` table. The repo filter SHALL match rows whose repo equals the requested repo or whose repo is empty/null; it SHALL exclude rows with a different non-empty repo. Date filters SHALL exclude rows with empty/null dates. Store search SHALL return at most 250 candidate rows after FTS and metadata filtering.

#### Scenario: Project filter is required by query API
- **WHEN** the fetch workflow calls the store search API without a project
- **THEN** the store returns a validation error
- **AND** no unbounded cross-project search is executed

#### Scenario: Optional metadata filters are combined
- **WHEN** the fetch workflow searches with project `Acme`, repo `One.Backend`, type `daily_work`, since `2026-04-01`, and until `2026-04-30`
- **THEN** the store returns only `daily_work` notes for project `Acme`
- **AND** every returned row has repo `One.Backend`
- **AND** every returned row has a date in the inclusive range `2026-04-01` through `2026-04-30`

#### Scenario: Repo filter allows project-level notes
- **WHEN** the fetch workflow searches with project `Acme` and repo `One.Backend` without a type filter
- **THEN** rows with repo `One.Frontend` are not returned
- **AND** matching rows with empty/null repo remain eligible

#### Scenario: Notes with null optional metadata remain eligible when filter absent
- **WHEN** matching knowledge notes have null repo or date values
- **AND** the corresponding repo or date filter is not supplied
- **THEN** those notes remain eligible for retrieval

### Requirement: Search result data supports scoring and citation
The store layer SHALL return enough data for the application layer to score, route, render, and cite fetched notes, including note identifier, project, type, section ID, metadata fields, source path, content, and any available FTS rank signal.

#### Scenario: Search result includes citation fields
- **WHEN** a matching logical note row is returned by search
- **THEN** the result includes `source_path`
- **AND** the result includes `section_id`
- **AND** the result includes the note row identifier

#### Scenario: Search result includes routing fields
- **WHEN** a matching logical note row is returned by search
- **THEN** the result includes note type, repo, date, title, summary, notes text, tags, and content fields needed for section routing
