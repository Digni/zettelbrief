package store

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

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
}

func TokenizeSearchQuery(query string) []string {
	matches := searchTokenPattern.FindAllString(strings.ToLower(query), -1)
	seen := map[string]bool{}
	tokens := make([]string, 0, len(matches))
	for _, token := range matches {
		token = strings.Trim(token, "_")
		if token == "" || seen[token] {
			continue
		}
		seen[token] = true
		tokens = append(tokens, token)
	}
	return tokens
}

var searchTokenPattern = regexp.MustCompile(`[\pL\pN_]+`)

func BuildFTSQuery(tokens []string) (string, error) {
	if len(tokens) == 0 {
		return "", fmt.Errorf("%w: query contains no searchable terms", ErrInvalidSearch)
	}
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		cleaned := strings.TrimSpace(token)
		if cleaned == "" {
			continue
		}
		parts = append(parts, `"`+strings.ReplaceAll(cleaned, `"`, `""`)+`"`)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("%w: query contains no searchable terms", ErrInvalidSearch)
	}
	return strings.Join(parts, " AND "), nil
}

func (db *DB) SearchNotes(req SearchRequest) ([]SearchResult, []string, error) {
	if strings.TrimSpace(req.Project) == "" {
		return nil, nil, fmt.Errorf("%w: project is required", ErrInvalidSearch)
	}
	tokens := TokenizeSearchQuery(req.Query)
	match, err := BuildFTSQuery(tokens)
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
SELECT n.id, n.project, n.type, n.section_id, n.repo, n.branch, n.date, n.title, n.summary, n.verification, n.notes_text, n.commit_hash, n.ticket, n.granola_id, n.updated_at, n.tags, n.source_path, n.content, n.metadata_json, bm25(notes_fts) AS rank
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
		if err := rows.Scan(&r.ID, &r.Project, &typ, &r.SectionID, &r.Repo, &r.Branch, &r.Date, &r.Title, &r.Summary, &r.Verification, &r.NotesText, &r.CommitHash, &r.Ticket, &r.GranolaID, &r.UpdatedAt, &tagsRaw, &r.SourcePath, &r.Content, &r.MetadataJSON, &r.FTSRank); err != nil {
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
