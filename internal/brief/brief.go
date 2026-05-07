package brief

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/cyphant/zettelbrief/internal/models"
	"github.com/cyphant/zettelbrief/internal/store"
)

const SectionLimit = 15

var SectionNames = []string{"Relevant Prior Work", "Meeting Context", "Decisions And Constraints", "Risks For This Task", "Open Questions"}

const EmptySectionText = "No matching sources were found."

type Confidence string

const (
	ConfidenceHigh   Confidence = "HIGH"
	ConfidenceMedium Confidence = "MEDIUM"
	ConfidenceLow    Confidence = "LOW"
)

type Candidate struct {
	ID           int64
	Project      string
	Type         models.NoteType
	SectionID    string
	Repo         sql.NullString
	Branch       sql.NullString
	Date         sql.NullString
	Title        sql.NullString
	Summary      sql.NullString
	Verification sql.NullString
	NotesText    sql.NullString
	Tags         []string
	SourcePath   string
	Content      string
	Snippet      sql.NullString
}

type Entry struct {
	ID            string
	SectionName   string
	Candidate     Candidate
	Note          store.SearchResult
	Score         float64
	BaseScore     float64
	RecencyFactor float64
	Text          string
	Excerpt       string
	Confidence    Confidence
	MatchReason   string
	OffsetStart   *int
	OffsetEnd     *int
}

type SourceMap struct {
	GeneratedAt time.Time       `json:"generated_at"`
	Entries     []SourceMapping `json:"entries"`
}

type SourceMapping struct {
	EntryID         string            `json:"entry_id"`
	BriefSection    string            `json:"brief_section"`
	SourcePath      string            `json:"source_path"`
	RowID           int64             `json:"row_id"`
	SectionID       string            `json:"section_id"`
	SectionName     string            `json:"section_name"`
	Classification  string            `json:"classification"`
	Confidence      Confidence        `json:"confidence"`
	MatchReason     string            `json:"match_reason"`
	Excerpt         string            `json:"excerpt"`
	Score           float64           `json:"score"`
	RecencyFactor   float64           `json:"recency_factor"`
	CharOffsetStart *int              `json:"char_offset_start,omitempty"`
	CharOffsetEnd   *int              `json:"char_offset_end,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

func CandidateFromSearchResult(result store.SearchResult) Candidate {
	return Candidate{ID: result.ID, Project: result.Project, Type: result.Type, SectionID: result.SectionID, Repo: result.Repo, Branch: result.Branch, Date: result.Date, Title: result.Title, Summary: result.Summary, Verification: result.Verification, NotesText: result.NotesText, Tags: result.Tags, SourcePath: result.SourcePath, Content: result.Content, Snippet: result.Snippet}
}

func CandidatesFromSearchResults(results []store.SearchResult) []Candidate {
	candidates := make([]Candidate, 0, len(results))
	for _, result := range results {
		candidates = append(candidates, CandidateFromSearchResult(result))
	}
	return candidates
}

func Compose(results []store.SearchResult, queryTokens []string, repoFilter string, generatedAt time.Time) ([]Entry, SourceMap) {
	return ComposeCandidates(CandidatesFromSearchResults(results), queryTokens, repoFilter, generatedAt)
}

func ComposeCandidates(candidates []Candidate, queryTokens []string, repoFilter string, generatedAt time.Time) ([]Entry, SourceMap) {
	terms := make([]store.QueryTerm, 0, len(queryTokens))
	for _, token := range queryTokens {
		terms = append(terms, store.QueryTerm{Raw: token, Tokens: []string{token}})
	}
	return ComposeCandidatesWithTerms(candidates, terms, repoFilter, generatedAt)
}

func ComposeCandidatesWithTerms(candidates []Candidate, queryTerms []store.QueryTerm, repoFilter string, generatedAt time.Time) ([]Entry, SourceMap) {
	bySection := map[string][]Entry{}
	for _, candidate := range candidates {
		for _, section := range routeSections(candidate, repoFilter) {
			base, final, factor := ScoreCandidateDetailed(candidate, queryTerms, section, generatedAt)
			excerpt, start, end := excerpt(candidate)
			confidence, reason := confidenceAndReason(candidate, queryTerms, repoFilter)
			entry := Entry{SectionName: section, Candidate: candidate, Note: searchResultFromCandidate(candidate), BaseScore: base, Score: final, RecencyFactor: factor, Excerpt: excerpt, OffsetStart: start, OffsetEnd: end, Confidence: confidence, MatchReason: reason}
			entry.Text = entryText(entry)
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
	return ScoreCandidate(CandidateFromSearchResult(result), queryTokens, section)
}

func ScoreCandidate(candidate Candidate, queryTokens []string, section string) float64 {
	terms := make([]store.QueryTerm, 0, len(queryTokens))
	for _, token := range queryTokens {
		terms = append(terms, store.QueryTerm{Raw: token, Tokens: []string{token}})
	}
	_, final, _ := ScoreCandidateDetailed(candidate, terms, section, time.Now().UTC())
	return final
}

func ScoreCandidateDetailed(candidate Candidate, queryTerms []store.QueryTerm, section string, now time.Time) (base, final, recencyFactor float64) {
	searchable := strings.Join([]string{ns(candidate.Title), ns(candidate.Summary), ns(candidate.Verification), ns(candidate.NotesText), strings.Join(candidate.Tags, " "), candidate.Content}, " ")
	words := store.TokenizeSearchQuery(searchable)
	wordSet := map[string]bool{}
	for _, word := range words {
		wordSet[word] = true
	}
	hits := 0.0
	for _, term := range queryTerms {
		termHit := false
		for _, token := range term.Tokens {
			if wordSet[token] {
				termHit = true
				break
			}
		}
		if termHit {
			if term.Identifier {
				hits += 2
			} else {
				hits++
			}
		}
	}
	density := 0.0
	if len(words) > 0 {
		density = hits / float64(len(words))
	}
	base = density + typeWeight(candidate.Type, section)
	recencyFactor = recency(candidate, now)
	final = base * recencyFactor
	return base, final, recencyFactor
}

func recency(candidate Candidate, now time.Time) float64 {
	if candidate.Type != models.NoteTypeDailyWork && candidate.Type != models.NoteTypeMeeting {
		return 1.0
	}
	if !candidate.Date.Valid || candidate.Date.String == "" {
		return 0.3
	}
	date, err := time.Parse("2006-01-02", candidate.Date.String)
	if err != nil {
		return 0.3
	}
	today := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC)
	age := math.Floor(today.Sub(date).Hours() / 24)
	if age < 0 {
		age = 0
	}
	factor := 1 - age/180
	if factor < 0.3 {
		return 0.3
	}
	if factor > 1 {
		return 1
	}
	return factor
}

func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Score != entries[j].Score {
			return entries[i].Score > entries[j].Score
		}
		left := entries[i].candidate()
		right := entries[j].candidate()
		if left.SourcePath != right.SourcePath {
			return left.SourcePath < right.SourcePath
		}
		if left.SectionID != right.SectionID {
			return left.SectionID < right.SectionID
		}
		return left.ID < right.ID
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
			if entry.Confidence != "" && entry.Excerpt != "" {
				fmt.Fprintf(&b, "- [%s] %s — %s [%s]\n", entry.Confidence, entry.Text, entry.Excerpt, entry.ID)
			} else {
				fmt.Fprintf(&b, "- %s [%s]\n", entry.Text, entry.ID)
			}
			sources[entry.candidate().SourcePath] = true
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
		candidate := entry.candidate()
		mappings = append(mappings, SourceMapping{EntryID: entry.ID, BriefSection: entry.SectionName, SourcePath: candidate.SourcePath, RowID: candidate.ID, SectionID: candidate.SectionID, SectionName: entrySectionName(candidate), Classification: string(candidate.Type), Confidence: entry.Confidence, MatchReason: entry.MatchReason, Excerpt: entry.Excerpt, Score: entry.Score, RecencyFactor: entry.RecencyFactor, CharOffsetStart: entry.OffsetStart, CharOffsetEnd: entry.OffsetEnd, Metadata: metadata(candidate)})
	}
	return SourceMap{GeneratedAt: generatedAt.UTC(), Entries: mappings}
}

func MarshalSources(sourceMap SourceMap) ([]byte, error) {
	return json.MarshalIndent(sourceMap, "", "  ")
}

func routeSections(candidate Candidate, repoFilter string) []string {
	sections := []string{}
	if (candidate.Type == models.NoteTypeDailyWork && dailyMatchesRepo(candidate, repoFilter)) || candidate.Type == models.NoteTypeKnowledge {
		sections = append(sections, "Relevant Prior Work")
	}
	if candidate.Type == models.NoteTypeMeeting {
		sections = append(sections, "Meeting Context")
	}
	combined := strings.ToLower(strings.Join([]string{ns(candidate.Title), ns(candidate.Summary), ns(candidate.NotesText), candidate.Content}, "\n"))
	if (candidate.Type == models.NoteTypeMeeting || candidate.Type == models.NoteTypeProjectState || candidate.Type == models.NoteTypeDailyWork) && containsAny(combined, []string{"decision", "constraint", "decided"}) {
		sections = append(sections, "Decisions And Constraints")
	}
	if candidate.Type == models.NoteTypeDailyWork && strings.TrimSpace(ns(candidate.NotesText)) != "" {
		sections = append(sections, "Risks For This Task")
	} else if candidate.Type == models.NoteTypeProjectState && headingContains(candidate.Content, []string{"blocker", "risk", "issue"}) {
		sections = append(sections, "Risks For This Task")
	}
	if candidate.Type == models.NoteTypeProjectState && headingContains(candidate.Content, []string{"pending", "todo", "open question"}) {
		sections = append(sections, "Open Questions")
	} else if candidate.Type == models.NoteTypeDailyWork {
		notes := strings.ToLower(ns(candidate.NotesText))
		if strings.Contains(notes, "?") || strings.Contains(notes, "todo") {
			sections = append(sections, "Open Questions")
		}
	}
	return sections
}

func dailyMatchesRepo(candidate Candidate, repoFilter string) bool {
	if repoFilter == "" || candidate.Type != models.NoteTypeDailyWork {
		return true
	}
	return candidate.Repo.Valid && candidate.Repo.String == repoFilter
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

func entryText(entry Entry) string {
	candidate := entry.candidate()
	for _, value := range []string{ns(candidate.Title), entry.MatchReason, candidate.SourcePath} {
		if value != "" {
			return oneLine(value)
		}
	}
	return candidate.SourcePath
}

func excerpt(candidate Candidate) (string, *int, *int) {
	var value string
	switch candidate.Type {
	case models.NoteTypeDailyWork:
		value = firstNonEmpty(ns(candidate.Summary), ns(candidate.NotesText))
	case models.NoteTypeMeeting, models.NoteTypeKnowledge:
		value = firstNonEmpty(ns(candidate.Snippet), candidate.Content)
	case models.NoteTypeProjectState:
		value = firstNonEmpty(ns(candidate.Summary), firstParagraphAfterTitle(candidate.Content), candidate.Content)
	default:
		value = firstNonEmpty(ns(candidate.Summary), ns(candidate.Title), candidate.Content)
	}
	bounded := boundWords(oneLine(value), 120)
	start, end := uniqueOffsets(candidate.Content, bounded)
	return bounded, start, end
}

func confidenceAndReason(candidate Candidate, queryTerms []store.QueryTerm, repoFilter string) (Confidence, string) {
	if repoFilter != "" && candidate.Repo.Valid && candidate.Repo.String == repoFilter {
		return ConfidenceHigh, "repo:" + candidate.Repo.String
	}
	if candidate.Project != "" {
		return ConfidenceMedium, "project:" + candidate.Project
	}
	branch := strings.ToLower(ns(candidate.Branch))
	for _, term := range queryTerms {
		for _, token := range term.Tokens {
			if branch != "" && strings.Contains(branch, token) {
				return ConfidenceLow, "branch:" + ns(candidate.Branch)
			}
		}
	}
	text := strings.ToLower(strings.Join([]string{ns(candidate.Title), ns(candidate.Summary), ns(candidate.NotesText), candidate.Content}, " "))
	for _, term := range queryTerms {
		for _, token := range term.Tokens {
			if strings.Contains(text, token) {
				return ConfidenceLow, "keyword:" + token
			}
		}
	}
	return ConfidenceLow, "keyword"
}

func metadata(candidate Candidate) map[string]string {
	out := map[string]string{"type": string(candidate.Type)}
	for key, value := range map[string]string{"repo": ns(candidate.Repo), "branch": ns(candidate.Branch), "date": ns(candidate.Date), "title": ns(candidate.Title), "summary": ns(candidate.Summary)} {
		if value != "" {
			out[key] = value
		}
	}
	return out
}

func entrySectionName(candidate Candidate) string {
	if title := ns(candidate.Title); title != "" {
		return title
	}
	return candidate.SectionID
}

func (entry Entry) candidate() Candidate {
	if entry.Candidate.ID != 0 || entry.Candidate.SourcePath != "" || entry.Candidate.Type != "" {
		return entry.Candidate
	}
	return CandidateFromSearchResult(entry.Note)
}

func searchResultFromCandidate(candidate Candidate) store.SearchResult {
	return store.SearchResult{ID: candidate.ID, Project: candidate.Project, Type: candidate.Type, SectionID: candidate.SectionID, Repo: candidate.Repo, Branch: candidate.Branch, Date: candidate.Date, Title: candidate.Title, Summary: candidate.Summary, Verification: candidate.Verification, NotesText: candidate.NotesText, Tags: candidate.Tags, SourcePath: candidate.SourcePath, Content: candidate.Content, Snippet: candidate.Snippet}
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
func firstParagraphAfterTitle(content string) string {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if len(lines) == 0 {
				continue
			}
			break
		}
		lines = append(lines, trimmed)
	}
	return strings.Join(lines, " ")
}
func uniqueOffsets(content, excerpt string) (*int, *int) {
	if content == "" || excerpt == "" {
		return nil, nil
	}
	start := strings.Index(content, excerpt)
	if start < 0 || strings.LastIndex(content, excerpt) != start {
		return nil, nil
	}
	end := start + len(excerpt)
	return &start, &end
}
func boundWords(value string, max int) string {
	words := strings.Fields(value)
	if len(words) <= max {
		return strings.Join(words, " ")
	}
	return strings.Join(words[:max], " ") + "…"
}
