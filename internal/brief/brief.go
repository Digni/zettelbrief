package brief

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cyphant/zettelbrief/internal/models"
	"github.com/cyphant/zettelbrief/internal/store"
)

const SectionLimit = 15

var SectionNames = []string{"Relevant Prior Work", "Meeting Context", "Decisions And Constraints", "Risks For This Task", "Open Questions"}

const EmptySectionText = "No matching sources were found."

type Entry struct {
	ID          string
	SectionName string
	Note        store.SearchResult
	Score       float64
	Text        string
}

type SourceMap struct {
	GeneratedAt time.Time       `json:"generated_at"`
	Entries     []SourceMapping `json:"entries"`
}

type SourceMapping struct {
	EntryID      string            `json:"entry_id"`
	BriefSection string            `json:"brief_section"`
	SourcePath   string            `json:"source_path"`
	RowID        int64             `json:"row_id"`
	SectionID    string            `json:"section_id"`
	SectionName  string            `json:"section_name"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

func Compose(results []store.SearchResult, queryTokens []string, repoFilter string, generatedAt time.Time) ([]Entry, SourceMap) {
	bySection := map[string][]Entry{}
	for _, result := range results {
		for _, section := range routeSections(result, repoFilter) {
			entry := Entry{SectionName: section, Note: result, Text: entryText(result)}
			entry.Score = Score(result, queryTokens, section)
			bySection[section] = append(bySection[section], entry)
		}
	}
	var entries []Entry
	for _, section := range SectionNames {
		sectionEntries := bySection[section]
		sortEntries(sectionEntries)
		if len(sectionEntries) > SectionLimit {
			sectionEntries = sectionEntries[:SectionLimit]
		}
		for i := range sectionEntries {
			sectionEntries[i].ID = fmt.Sprintf("%s-%02d", slug(section), i+1)
		}
		entries = append(entries, sectionEntries...)
	}
	return entries, BuildSourceMap(entries, generatedAt)
}

func Score(result store.SearchResult, queryTokens []string, section string) float64 {
	searchable := strings.Join([]string{ns(result.Title), ns(result.Summary), ns(result.Verification), ns(result.NotesText), strings.Join(result.Tags, " "), result.Content}, " ")
	words := store.TokenizeSearchQuery(searchable)
	if len(words) == 0 {
		return typeWeight(result.Type, section)
	}
	wordSet := map[string]bool{}
	for _, word := range words {
		wordSet[word] = true
	}
	hits := 0
	for _, token := range queryTokens {
		if wordSet[token] {
			hits++
		}
	}
	return float64(hits)/float64(len(words)) + typeWeight(result.Type, section)
}

func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Score != entries[j].Score {
			return entries[i].Score > entries[j].Score
		}
		if entries[i].Note.SourcePath != entries[j].Note.SourcePath {
			return entries[i].Note.SourcePath < entries[j].Note.SourcePath
		}
		if entries[i].Note.SectionID != entries[j].Note.SectionID {
			return entries[i].Note.SectionID < entries[j].Note.SectionID
		}
		return entries[i].Note.ID < entries[j].Note.ID
	})
}

func RenderMarkdown(entries []Entry) string {
	bySection := map[string][]Entry{}
	for _, entry := range entries {
		bySection[entry.SectionName] = append(bySection[entry.SectionName], entry)
	}
	var b strings.Builder
	b.WriteString("# Zettelbrief\n\n")
	sources := map[string]bool{}
	for _, section := range SectionNames {
		b.WriteString("## " + section + "\n\n")
		sectionEntries := bySection[section]
		if len(sectionEntries) == 0 {
			b.WriteString(EmptySectionText + "\n\n")
			continue
		}
		for _, entry := range sectionEntries {
			fmt.Fprintf(&b, "- %s [%s]\n", entry.Text, entry.ID)
			sources[entry.Note.SourcePath] = true
		}
		b.WriteByte('\n')
	}
	b.WriteString("## Sources\n\n")
	if len(sources) == 0 {
		b.WriteString(EmptySectionText + "\n")
		return b.String()
	}
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		fmt.Fprintf(&b, "- %s\n", path)
	}
	return b.String()
}

func BuildSourceMap(entries []Entry, generatedAt time.Time) SourceMap {
	mappings := make([]SourceMapping, 0, len(entries))
	for _, entry := range entries {
		mappings = append(mappings, SourceMapping{EntryID: entry.ID, BriefSection: entry.SectionName, SourcePath: entry.Note.SourcePath, RowID: entry.Note.ID, SectionID: entry.Note.SectionID, SectionName: entrySectionName(entry.Note), Metadata: metadata(entry.Note)})
	}
	return SourceMap{GeneratedAt: generatedAt.UTC(), Entries: mappings}
}

func MarshalSources(sourceMap SourceMap) ([]byte, error) {
	return json.MarshalIndent(sourceMap, "", "  ")
}

func routeSections(result store.SearchResult, repoFilter string) []string {
	sections := []string{}
	if (result.Type == models.NoteTypeDailyWork && dailyMatchesRepo(result, repoFilter)) || result.Type == models.NoteTypeKnowledge {
		sections = append(sections, "Relevant Prior Work")
	}
	if result.Type == models.NoteTypeMeeting {
		sections = append(sections, "Meeting Context")
	}
	combined := strings.ToLower(strings.Join([]string{ns(result.Title), ns(result.Summary), ns(result.NotesText), result.Content}, "\n"))
	if (result.Type == models.NoteTypeMeeting || result.Type == models.NoteTypeProjectState || result.Type == models.NoteTypeDailyWork) && containsAny(combined, []string{"decision", "constraint", "decided"}) {
		sections = append(sections, "Decisions And Constraints")
	}
	if result.Type == models.NoteTypeDailyWork && strings.TrimSpace(ns(result.NotesText)) != "" {
		sections = append(sections, "Risks For This Task")
	} else if result.Type == models.NoteTypeProjectState && headingContains(result.Content, []string{"blocker", "risk", "issue"}) {
		sections = append(sections, "Risks For This Task")
	}
	if result.Type == models.NoteTypeProjectState && headingContains(result.Content, []string{"pending", "todo", "open question"}) {
		sections = append(sections, "Open Questions")
	} else if result.Type == models.NoteTypeDailyWork {
		notes := strings.ToLower(ns(result.NotesText))
		if strings.Contains(notes, "?") || strings.Contains(notes, "todo") {
			sections = append(sections, "Open Questions")
		}
	}
	return sections
}

func dailyMatchesRepo(result store.SearchResult, repoFilter string) bool {
	if repoFilter == "" || result.Type != models.NoteTypeDailyWork {
		return true
	}
	return result.Repo.Valid && result.Repo.String == repoFilter
}

func typeWeight(typ models.NoteType, section string) float64 {
	weights := map[string]map[models.NoteType]float64{
		"Relevant Prior Work":       {models.NoteTypeDailyWork: 0.30, models.NoteTypeKnowledge: 0.20},
		"Meeting Context":           {models.NoteTypeMeeting: 0.30},
		"Decisions And Constraints": {models.NoteTypeProjectState: 0.25, models.NoteTypeMeeting: 0.20, models.NoteTypeDailyWork: 0.10},
		"Risks For This Task":       {models.NoteTypeDailyWork: 0.20, models.NoteTypeProjectState: 0.15},
		"Open Questions":            {models.NoteTypeProjectState: 0.20, models.NoteTypeDailyWork: 0.15},
	}
	if byType, ok := weights[section]; ok {
		return byType[typ]
	}
	return 0
}

func entryText(result store.SearchResult) string {
	for _, value := range []string{ns(result.Summary), ns(result.Title), strings.TrimSpace(result.Content)} {
		if value != "" {
			return oneLine(value)
		}
	}
	return result.SourcePath
}

func metadata(result store.SearchResult) map[string]string {
	out := map[string]string{"type": string(result.Type)}
	for key, value := range map[string]string{"repo": ns(result.Repo), "branch": ns(result.Branch), "date": ns(result.Date), "title": ns(result.Title), "summary": ns(result.Summary)} {
		if value != "" {
			out[key] = value
		}
	}
	return out
}

func entrySectionName(result store.SearchResult) string {
	if title := ns(result.Title); title != "" {
		return title
	}
	return result.SectionID
}

func headingContains(content string, terms []string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(line, "#") && containsAny(line, terms) {
			return true
		}
	}
	return false
}

func containsAny(value string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func ns(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func slug(value string) string {
	return strings.ReplaceAll(strings.ToLower(value), " ", "-")
}
