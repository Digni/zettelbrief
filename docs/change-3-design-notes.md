# zettelbrief Change 3 — design notes

Influences for the Quality+Polish phase. Order matters: excerpts first, recency second, incremental last.

-----

## Excerpts: per-classification, not universal

Don’t write a generic “N tokens around the hit” function. The parser already extracts structured fields for some types — use them. Fall back to FTS5 `snippet()` for the rest.

|Classification |Excerpt source                                                   |Why                                                             |
|---------------|-----------------------------------------------------------------|----------------------------------------------------------------|
|`daily_work`   |Verbatim `Summary:` field; fall back to `Notes:` if empty        |Parser already isolates the relevant span. No windowing needed. |
|`meeting`      |FTS5 `snippet()` — highest-density match window                  |Long notes covering multiple topics; query determines relevance.|
|`project_state`|First paragraph after title, or frontmatter `summary:` if present|Declarative content; the lede is usually the answer.            |
|`knowledge`    |FTS5 `snippet()` around matched heading                          |Generic catch-all; let the matcher pick the span.               |

Use SQLite’s built-in:

```sql
snippet(notes_fts, -1, '', '', '…', 20)
```

Token count tunable per type. Daily-work excerpts are often 10–20 tokens (a one-line summary); meetings benefit from 30–40.

**Budget target**: 80–120 tokens per excerpt. A brief with 5 sources × 6 sections × 1 excerpt at this size = 3–4k tokens total. Comfortable for the agent to scan before deciding which files to open.

-----

## Confidence markers: tie to evidence, not just match type

Roadmap defines HIGH/MEDIUM/LOW. Make the excerpt *show why* — the agent’s decision quality depends on it.

- **HIGH** (direct repo/branch match): excerpt should include the structural evidence. `Repo: One.Backend / Branch: feat/billable / Summary: …`
- **MEDIUM** (project match): snippet around the project name reference.
- **LOW** (keyword only): snippet around the keyword hit. Including LOW results is only worthwhile if the agent can tell whether the keyword is in relevant context — the excerpt is what makes that call possible.

Add a `match_reason` field in `sources.json` that records the *why*: `"repo:One.Backend, query_term:billable"` or `"keyword:persistence"`. Debugging-grade for now, audit-grade for client work later.

-----

## Recency: type-conditional, not blanket

A linear decay applied to everything is wrong. A 2-year-old decision note is usually *more* authoritative than yesterday’s daily log.

|Type                                             |Recency weight                                     |
|-------------------------------------------------|---------------------------------------------------|
|`daily_work`                                     |Decay applies. `score × max(0.3, 1 - age_days/180)`|
|`meeting`                                        |Decay applies. Same formula.                       |
|`project_state`                                  |No decay (always reflects current state).          |
|`knowledge` (incl. frontmatter `decision`/`plan`)|No decay.                                          |

Bottom-cap at 0.3 so old daily-work notes don’t disappear entirely — they can still be relevant for “we tried this before” cases.

-----

## sources.json: structured data the agent uses programmatically

`brief.md` is for the agent to scan. `sources.json` is for the agent to act on. Useful additions beyond what’s in the brief:

```json
{
  "source_path": "1.Projects/Acme/1. Daily Work/2026-04-25.md",
  "section_id": "billable-update-fix",
  "classification": "daily_work",
  "confidence": "HIGH",
  "match_reason": "repo:One.Backend, query_term:billable",
  "excerpt": "Reproduced the persistence bug on dev. Root cause appears to be the transaction scope ending before...",
  "char_offset_start": 1247,
  "char_offset_end": 1389,
  "score": 0.87,
  "recency_factor": 1.0
}
```

Char offsets let the agent jump to the right span when it opens the file. `match_reason` makes wrong rankings debuggable.

-----

## Query handling: stopwords before search

Real task descriptions are sentence-shaped: `"fix billable service update persistence"`. FTS5 tokenizes but doesn’t know which words carry signal. Two deterministic moves:

1. Strip a small stopword list (`fix`, `add`, `update`, `the`, `a`, common verbs that appear in task framing). Configurable, ~30 words.
1. Detect identifiers — camelCase, snake_case, dotted, capitalized — and weight them higher in scoring. These are nearly always signal.

Optional: `--terms` and `--identifier` flags so the caller can pass structured terms directly when known. The agent calling zettelbrief usually does know.

No LLM needed. Both are 20-line functions.

-----

## Skip incremental scan until you measure

Roadmap has it in Change 3. Worth deferring. On a personal vault of a few thousand notes, full rescan is probably <2s. Adding incremental adds: hash storage, change detection, partial-write recovery, cache invalidation rules for FTS5. Real complexity.

Measure first. If full scan is fast enough, ship without it and reclaim that budget for excerpt quality and recency tuning.

-----

## Suggested Change 3 order

1. **FTS5 `snippet()` + per-classification excerpt strategies** — single biggest leverage gain. Unlocks the agent’s “scan-then-decide” loop.
1. **Confidence markers wired to `match_reason`** — small change, big audit/debugging payoff.
1. **Type-conditional recency** — straightforward once excerpts are working.
1. **Stopword/identifier query handling** — improves precision noticeably with little code.
1. **Date-range flags (`--since`, `--until`)** — easy, ship anytime.
1. **Incremental scan** — only if measurement shows it’s needed.

Each step produces a usable, testable increment. Steps 1–4 give you a brief that’s *good*, not just correct.
