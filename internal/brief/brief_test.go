package brief

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/cyphant/zettelbrief/internal/models"
	"github.com/cyphant/zettelbrief/internal/store"
)

func TestComposeScoresRoutesLimitsAndMapsSources(t *testing.T) {
	generated := time.Date(2026, 5, 1, 6, 42, 13, 0, time.UTC)
	results := []store.SearchResult{
		result(2, models.NoteTypeDailyWork, "b.md", "002", "One.Backend", "One.Backend", "persistence", "TODO: risk?"),
		result(1, models.NoteTypeDailyWork, "a.md", "001", "One.Backend", "", "persistence", ""),
		result(3, models.NoteTypeKnowledge, "c.md", "", "", "Architecture", "persistence", ""),
		result(4, models.NoteTypeMeeting, "m.md", "", "", "Planning", "decision: keep sqlite persistence", ""),
		result(5, models.NoteTypeProjectState, "s.md", "", "", "State", "persistence", ""),
	}
	results[4].Content = "# Risk\nPersistence issue\n# Open Question\nTODO persistence"
	entries, sources := Compose(results, []string{"persistence"}, "One.Backend", generated)
	if len(entries) == 0 || len(sources.Entries) != len(entries) {
		t.Fatalf("entries=%d sources=%d", len(entries), len(sources.Entries))
	}
	prior := sectionEntries(entries, "Relevant Prior Work")
	if len(prior) != 3 || prior[0].Note.SourcePath != "a.md" || prior[1].Note.SourcePath != "c.md" || prior[2].Note.SourcePath != "b.md" {
		t.Fatalf("prior ordering = %#v", prior)
	}
	if len(sectionEntries(entries, "Meeting Context")) != 1 || len(sectionEntries(entries, "Decisions And Constraints")) != 1 || len(sectionEntries(entries, "Risks For This Task")) != 2 || len(sectionEntries(entries, "Open Questions")) != 2 {
		t.Fatalf("unexpected routing: %#v", entries)
	}
	for _, mapping := range sources.Entries {
		if mapping.SourcePath == "" || mapping.RowID == 0 || mapping.BriefSection == "" || mapping.EntryID == "" || mapping.Metadata["type"] == "" {
			t.Fatalf("incomplete source mapping: %#v", mapping)
		}
	}
}

func TestRenderMarkdownEmptyAndDeduplicatedSources(t *testing.T) {
	empty := RenderMarkdown(nil)
	if !strings.Contains(empty, "## Relevant Prior Work") || strings.Count(empty, EmptySectionText) != 6 {
		t.Fatalf("empty markdown = %s", empty)
	}
	entries := []Entry{{ID: "one", SectionName: "Relevant Prior Work", Note: result(1, models.NoteTypeKnowledge, "same.md", "", "", "T", "S", ""), Text: "First"}, {ID: "two", SectionName: "Risks For This Task", Note: result(2, models.NoteTypeDailyWork, "same.md", "002", "One.Backend", "T", "S", "note"), Text: "Second"}}
	md := RenderMarkdown(entries)
	if strings.Count(md, "- same.md") != 1 || !strings.Contains(md, "First [one]") || !strings.Contains(md, "Second [two]") {
		t.Fatalf("markdown = %s", md)
	}
}

func TestTypeWeightBreaksEqualDensity(t *testing.T) {
	results := []store.SearchResult{
		result(1, models.NoteTypeKnowledge, "a.md", "", "", "", "persistence", ""),
		result(2, models.NoteTypeDailyWork, "b.md", "001", "One.Backend", "", "persistence", ""),
	}
	entries, _ := Compose(results, []string{"persistence"}, "", time.Now())
	prior := sectionEntries(entries, "Relevant Prior Work")
	if len(prior) != 2 || prior[0].Note.Type != models.NoteTypeDailyWork {
		t.Fatalf("type weighting order = %#v", prior)
	}
}

func TestQualityFieldsConfidenceRecencyIdentifierAndExcerpt(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	candidate := Candidate{ID: 10, Project: "Acme", Type: models.NoteTypeDailyWork, SourcePath: "daily.md", SectionID: "001", Repo: models.NullString("One.Backend"), Branch: models.NullString("feature/billable"), Title: models.NullString("One.Backend"), Summary: models.NullString("Fixed billable persistence"), Content: "Fixed billable persistence", Date: sql.NullString{String: "2026-04-21", Valid: true}}
	terms := []store.QueryTerm{{Raw: "One.Backend", Tokens: []string{"one", "backend"}, Identifier: true}, {Raw: "persistence", Tokens: []string{"persistence"}}}
	entries, sources := ComposeCandidatesWithTerms([]Candidate{candidate}, terms, "One.Backend", now)
	if len(entries) == 0 || entries[0].Confidence != ConfidenceHigh || entries[0].MatchReason != "repo:One.Backend" {
		t.Fatalf("entry confidence/reason = %#v", entries)
	}
	if entries[0].BaseScore == 0 || entries[0].Score == 0 || entries[0].RecencyFactor <= 0.9 || entries[0].Excerpt != "Fixed billable persistence" {
		t.Fatalf("entry quality fields = %#v", entries[0])
	}
	if len(sources.Entries) == 0 || sources.Entries[0].Classification != "daily_work" || sources.Entries[0].Confidence != ConfidenceHigh || sources.Entries[0].MatchReason == "" || sources.Entries[0].Excerpt == "" || sources.Entries[0].Score == 0 || sources.Entries[0].RecencyFactor == 0 || sources.Entries[0].CharOffsetStart == nil {
		t.Fatalf("source mapping = %#v", sources.Entries)
	}

	projectCandidate := candidate
	projectCandidate.Repo = models.NullString("Other.Repo")
	entries, _ = ComposeCandidatesWithTerms([]Candidate{projectCandidate}, terms, "", now)
	if entries[0].Confidence != ConfidenceMedium || entries[0].MatchReason != "project:Acme" {
		t.Fatalf("project confidence = %#v", entries[0])
	}

	lowCandidate := candidate
	lowCandidate.Project = ""
	lowCandidate.Repo = sql.NullString{}
	entries, _ = ComposeCandidatesWithTerms([]Candidate{lowCandidate}, []store.QueryTerm{{Raw: "billable", Tokens: []string{"billable"}}}, "", now)
	if entries[0].Confidence != ConfidenceLow || entries[0].MatchReason != "branch:feature/billable" {
		t.Fatalf("low confidence = %#v", entries[0])
	}

	meeting := Candidate{ID: 11, Project: "Acme", Type: models.NoteTypeMeeting, SourcePath: "meeting.md", Title: models.NullString("Meeting"), Content: "full meeting content", Snippet: models.NullString("snippet around persistence"), Date: sql.NullString{String: "2025-10-01", Valid: true}}
	entries, _ = ComposeCandidatesWithTerms([]Candidate{meeting}, []store.QueryTerm{{Raw: "persistence", Tokens: []string{"persistence"}}}, "", now)
	if len(entries) == 0 || entries[0].Excerpt != "snippet around persistence" || entries[0].RecencyFactor < 0.3 || entries[0].RecencyFactor > 1.0 {
		t.Fatalf("meeting excerpt/recency = %#v", entries)
	}

	state := Candidate{ID: 12, Project: "Acme", Type: models.NoteTypeProjectState, SourcePath: "state.md", Title: models.NullString("State"), Content: "# State\n\nFirst project paragraph.\n\n# Open Question\nLater.", Date: sql.NullString{String: "2020-01-01", Valid: true}}
	entries, _ = ComposeCandidatesWithTerms([]Candidate{state}, []store.QueryTerm{{Raw: "project", Tokens: []string{"project"}}}, "", now)
	if len(entries) == 0 || entries[0].Excerpt != "First project paragraph." || entries[0].RecencyFactor != 1.0 {
		t.Fatalf("state excerpt/recency = %#v", entries)
	}
}

func TestIdentifierWeightingIncreasesScore(t *testing.T) {
	candidate := Candidate{ID: 1, Project: "Acme", Type: models.NoteTypeKnowledge, SourcePath: "k.md", Title: models.NullString("One Backend"), Content: "one backend persistence"}
	plainBase, _, _ := ScoreCandidateDetailed(candidate, []store.QueryTerm{{Raw: "one", Tokens: []string{"one"}}}, "Relevant Prior Work", time.Now())
	identifierBase, _, _ := ScoreCandidateDetailed(candidate, []store.QueryTerm{{Raw: "One.Backend", Tokens: []string{"one", "backend"}, Identifier: true}}, "Relevant Prior Work", time.Now())
	if identifierBase <= plainBase {
		t.Fatalf("identifierBase=%f plainBase=%f", identifierBase, plainBase)
	}
}

func TestSectionCapAndTieBreaking(t *testing.T) {
	var results []store.SearchResult
	for i := 20; i >= 1; i-- {
		path := string(rune('a'+i-1)) + ".md"
		results = append(results, result(int64(i), models.NoteTypeKnowledge, path, "", "", "T", "persistence", ""))
	}
	entries, _ := Compose(results, []string{"persistence"}, "", time.Now())
	prior := sectionEntries(entries, "Relevant Prior Work")
	if len(prior) != SectionLimit {
		t.Fatalf("prior len=%d", len(prior))
	}
	if prior[0].Note.SourcePath != "a.md" || prior[14].Note.SourcePath != "o.md" {
		t.Fatalf("tie order = %#v", prior)
	}
}

func result(id int64, typ models.NoteType, path, section, repo, title, summary, notes string) store.SearchResult {
	return store.SearchResult{ID: id, Project: "Acme", Type: typ, SourcePath: path, SectionID: section, Repo: models.NullString(repo), Title: models.NullString(title), Summary: models.NullString(summary), NotesText: models.NullString(notes), Content: strings.Join([]string{title, summary, notes}, " "), Date: sql.NullString{String: "2026-04-24", Valid: true}}
}

func sectionEntries(entries []Entry, section string) []Entry {
	var out []Entry
	for _, entry := range entries {
		if entry.SectionName == section {
			out = append(out, entry)
		}
	}
	return out
}
