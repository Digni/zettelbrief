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
	return store.SearchResult{ID: id, Project: "VetZ", Type: typ, SourcePath: path, SectionID: section, Repo: models.NullString(repo), Title: models.NullString(title), Summary: models.NullString(summary), NotesText: models.NullString(notes), Content: strings.Join([]string{title, summary, notes}, " "), Date: sql.NullString{String: "2026-04-24", Valid: true}}
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
