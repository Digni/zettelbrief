# zettelbrief

`zettelbrief` turns a personal Obsidian knowledge base into a focused briefing for coding agents.

The goal is to give an agent useful project context before it starts planning or implementing work. The source material already exists in Obsidian: daily work logs, Granola meeting notes, project notes, decisions, plans, and handover documents. `zettelbrief` reads those notes, finds what is likely relevant to the current task, and produces a compact, cited context pack that an agent can review before writing a plan.

## Why

Agents work better when they know the local history of a project: previous attempts, meeting decisions, naming conventions, unresolved questions, and known risks. That information is often present in Obsidian, but raw vault search is too noisy and inconsistent to use directly in every agent session.

`zettelbrief` is the in-between layer. It keeps Obsidian as the human-facing source of truth while making the relevant parts agent-readable, repeatable, and auditable.

## Concept

The first version should stay local and boring:

- Scan Markdown notes from configured Obsidian project folders.
- Classify v1 notes as daily work, meeting, project state, or general knowledge; preserve frontmatter types like decision/plan for future routing.
- Extract metadata such as project, repository, branch, date, title, tags, source type, logical section ID, and file path.
- Search notes by structured metadata and full-text relevance.
- Build a short briefing with excerpts, relevance notes, confidence, and source citations.
- Write the generated briefing into a local output directory so agents can read it like any other project artifact.

Example workflow:

```sh
zettelbrief fetch --project Acme --repo One.Backend "fix billable service update persistence"
```

Possible output:

```text
.zettelbrief/briefs/2026-04-26-153012/
  brief.md
  sources.json
```

## Brief Shape

A generated brief should be concise and source-backed:

```md
# Brief

## Relevant Prior Work
- ...

## Meeting Context
- ...

## Decisions And Constraints
- ...

## Risks For This Task
- ...

## Open Questions
- ...

## Sources
- 1.Projects/Acme/Meetings/...
- 1.Projects/Acme/1. Daily Work/...
```

The agent should receive a useful orientation, not a dump of raw notes.

## Non-Goals

- Not an MCP server.
- Not a replacement for Obsidian.
- Not a general chat interface over the vault.
- Not a place for agents to invent uncited project memory.
- Not tied to Azure DevOps or the `adomi` tool.

## Initial Direction

Start with deterministic local retrieval before adding semantic search:

1. Configure vault and project folder mappings.
2. Scan and classify Markdown notes.
3. Store metadata in SQLite with scan-run tracking and stale-row cleanup.
4. Add full-text search with metadata filters.
5. Generate cited `brief.md` context packs.
6. Add embeddings later if keyword and metadata search are not enough.

