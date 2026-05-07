package store

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/cyphant/zettelbrief/internal/models"
)

const DefaultSearchLimit = 250

var ErrInvalidSearch = errors.New("invalid search")

type SearchRequest struct {
	Project string
	Repo    string
	Type    models.NoteType
	Since   string
	Until   string
	Query   string
	Limit   int
}

type QueryTerm struct {
	Raw        string
	Tokens     []string
	Identifier bool
}

type SearchResult struct {
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
	CommitHash   sql.NullString
	Ticket       sql.NullString
	GranolaID    sql.NullString
	UpdatedAt    sql.NullString
	Tags         []string
	SourcePath   string
	Content      string
	MetadataJSON sql.NullString
	FTSRank      sql.NullFloat64
	Snippet      sql.NullString
}

var stopwords = map[string]bool{"a": true, "an": true, "and": true, "add": true, "change": true, "do": true, "fix": true, "for": true, "in": true, "of": true, "on": true, "the": true, "to": true, "update": true, "with": true, "work": true}

var rawSpanPattern = regexp.MustCompile(`[\pL\pN_./-]+`)
var componentPattern = regexp.MustCompile(`[\pL\pN]+`)

func TokenizeSearchQuery(query string) []string {
	terms := NormalizeSearchQuery(query)
	seen := map[string]bool{}
	tokens := make([]string, 0, len(terms))
	for _, term := range terms {
		for _, token := range term.Tokens {
			if token == "" || seen[token] {
				continue
			}
			seen[token] = true
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func NormalizeSearchQuery(query string) []QueryTerm {
	spans := rawSpanPattern.FindAllString(query, -1)
	terms := make([]QueryTerm, 0, len(spans))
	seen := map[string]bool{}
	for _, raw := range spans {
		raw = strings.Trim(raw, "_./-")
		if raw == "" {
			continue
		}
		identifier := isIdentifierLike(raw)
		parts := componentPattern.FindAllString(raw, -1)
		var tokens []string
		for _, part := range parts {
			for _, component := range splitCamel(strings.Trim(part, "_")) {
				token := strings.ToLower(component)
				if token == "" || (!identifier && stopwords[token]) {
					continue
				}
				tokens = append(tokens, token)
			}
		}
		tokens = dedupeStrings(tokens)
		if len(tokens) == 0 {
			continue
		}
		key := fmt.Sprintf("%t:%s", identifier, strings.Join(tokens, ","))
		if seen[key] {
			continue
		}
		seen[key] = true
		terms = append(terms, QueryTerm{Raw: raw, Tokens: tokens, Identifier: identifier})
	}
	return terms
}

func isIdentifierLike(raw string) bool {
	if strings.ContainsAny(raw, ".-/_") {
		return true
	}
	var prev rune
	for i, r := range raw {
		if i > 0 && unicode.IsLower(prev) && unicode.IsUpper(r) {
			return true
		}
		prev = r
	}
	return false
}

func splitCamel(value string) []string {
	if value == "" {
		return nil
	}
	runes := []rune(value)
	start := 0
	out := []string{}
	for i := 1; i < len(runes); i++ {
		if unicode.IsLower(runes[i-1]) && unicode.IsUpper(runes[i]) {
			out = append(out, string(runes[start:i]))
			start = i
		}
	}
	out = append(out, string(runes[start:]))
	return out
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func BuildFTSQuery(terms []QueryTerm) (string, error) {
	if len(terms) == 0 {
		return "", fmt.Errorf("%w: query contains no searchable terms after removing task-framing stopwords", ErrInvalidSearch)
	}
	parts := make([]string, 0, len(terms))
	for _, term := range terms {
		var quoted []string
		for _, token := range term.Tokens {
			cleaned := strings.TrimSpace(token)
			if cleaned != "" {
				quoted = append(quoted, `"`+strings.ReplaceAll(cleaned, `"`, `""`)+`"`)
			}
		}
		if len(quoted) == 0 {
			continue
		}
		if term.Identifier && len(quoted) > 1 {
			parts = append(parts, "("+strings.Join(quoted, " OR ")+")")
		} else {
			parts = append(parts, strings.Join(quoted, " AND "))
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("%w: query contains no searchable terms after removing task-framing stopwords", ErrInvalidSearch)
	}
	return strings.Join(parts, " AND "), nil
}

func (db *DB) SearchNotes(req SearchRequest) ([]SearchResult, []string, error) {
	if strings.TrimSpace(req.Project) == "" {
		return nil, nil, fmt.Errorf("%w: project is required", ErrInvalidSearch)
	}
	terms := NormalizeSearchQuery(req.Query)
	tokens := TokenizeSearchQuery(req.Query)
	match, err := BuildFTSQuery(terms)
	if err != nil {
		return nil, nil, err
	}
	limit := req.Limit
	if limit <= 0 || limit > DefaultSearchLimit {
		limit = DefaultSearchLimit
	}
	where := []string{"notes_fts MATCH ?", "n.project = ?"}
	args := []interface{}{match, req.Project}
	if req.Repo != "" {
		where = append(where, "(n.repo = ? OR n.repo IS NULL OR n.repo = '')")
		args = append(args, req.Repo)
	}
	if req.Type != "" {
		where = append(where, "n.type = ?")
		args = append(args, string(req.Type))
	}
	if req.Since != "" {
		where = append(where, "n.date IS NOT NULL AND n.date <> '' AND n.date >= ?")
		args = append(args, req.Since)
	}
	if req.Until != "" {
		where = append(where, "n.date IS NOT NULL AND n.date <> '' AND n.date <= ?")
		args = append(args, req.Until)
	}
	args = append(args, limit)
	query := `
SELECT n.id, n.project, n.type, n.section_id, n.repo, n.branch, n.date, n.title, n.summary, n.verification, n.notes_text, n.commit_hash, n.ticket, n.granola_id, n.updated_at, n.tags, n.source_path, n.content, n.metadata_json, bm25(notes_fts) AS rank, snippet(notes_fts, 3, '', '', '…', 32) AS snippet
FROM notes_fts
JOIN notes n ON n.id = notes_fts.rowid
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY rank ASC, n.source_path ASC, n.section_id ASC, n.id ASC
LIMIT ?`
	rows, err := db.SQL.Query(query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("search notes: %w", err)
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var typ string
		var tagsRaw sql.NullString
		if err := rows.Scan(&r.ID, &r.Project, &typ, &r.SectionID, &r.Repo, &r.Branch, &r.Date, &r.Title, &r.Summary, &r.Verification, &r.NotesText, &r.CommitHash, &r.Ticket, &r.GranolaID, &r.UpdatedAt, &tagsRaw, &r.SourcePath, &r.Content, &r.MetadataJSON, &r.FTSRank, &r.Snippet); err != nil {
			return nil, nil, err
		}
		r.Type = models.NoteType(typ)
		if tagsRaw.Valid {
			tags, err := DecodeTags(tagsRaw.String)
			if err != nil {
				return nil, nil, fmt.Errorf("decode tags for note %d: %w", r.ID, err)
			}
			r.Tags = tags
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return results, tokens, nil
}
