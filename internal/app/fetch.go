package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cyphant/zettelbrief/internal/brief"
	"github.com/cyphant/zettelbrief/internal/config"
	"github.com/cyphant/zettelbrief/internal/models"
	"github.com/cyphant/zettelbrief/internal/store"
)

type FetchOptions struct {
	Project    string
	Repo       string
	Type       string
	Since      string
	Until      string
	Query      string
	DBPath     string
	OutputRoot string
	Now        func() time.Time
}

type FetchSummary struct {
	OutputDir string
	Entries   int
}

func RunFetch(cfg config.Config, opts FetchOptions) (FetchSummary, error) {
	var summary FetchSummary
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.DBPath == "" {
		opts.DBPath = filepath.Join(".zettelbrief", "zettelbrief.db")
	}
	if opts.OutputRoot == "" {
		opts.OutputRoot = filepath.Join(filepath.Dir(opts.DBPath), "briefs")
	}
	typ, err := validateFetch(cfg, opts)
	if err != nil {
		return summary, err
	}
	if !store.DBExists(opts.DBPath) {
		return summary, fmt.Errorf("database %s does not exist; run zettelbrief scan --project %s first", opts.DBPath, opts.Project)
	}
	db, err := store.Open(opts.DBPath)
	if err != nil {
		return summary, err
	}
	defer db.Close()
	results, _, err := db.SearchNotes(store.SearchRequest{Project: opts.Project, Repo: opts.Repo, Type: typ, Since: opts.Since, Until: opts.Until, Query: opts.Query})
	if err != nil {
		return summary, err
	}
	generatedAt := opts.Now().UTC()
	candidates := brief.CandidatesFromSearchResults(results)
	entries, sourceMap := brief.ComposeCandidatesWithTerms(candidates, store.NormalizeSearchQuery(opts.Query), opts.Repo, generatedAt)
	outputDir := filepath.Join(opts.OutputRoot, generatedAt.Format("2006-01-02T15-04-05Z"))
	if err := os.MkdirAll(opts.OutputRoot, 0o700); err != nil {
		return summary, fmt.Errorf("create briefs directory: %w", err)
	}
	_ = os.Chmod(opts.OutputRoot, 0o700)
	if err := os.Mkdir(outputDir, 0o700); err != nil {
		return summary, fmt.Errorf("create brief directory: %w", err)
	}
	_ = os.Chmod(outputDir, 0o700)
	briefPath := filepath.Join(outputDir, "brief.md")
	if err := os.WriteFile(briefPath, []byte(brief.RenderMarkdown(entries)), 0o600); err != nil {
		return summary, fmt.Errorf("write brief.md: %w", err)
	}
	sources, err := brief.MarshalSources(sourceMap)
	if err != nil {
		return summary, err
	}
	if err := os.WriteFile(filepath.Join(outputDir, "sources.json"), append(sources, '\n'), 0o600); err != nil {
		return summary, fmt.Errorf("write sources.json: %w", err)
	}
	return FetchSummary{OutputDir: outputDir, Entries: len(entries)}, nil
}

func validateFetch(cfg config.Config, opts FetchOptions) (models.NoteType, error) {
	if strings.TrimSpace(opts.Project) == "" {
		return "", errors.New("fetch requires --project")
	}
	if cfg.Projects == nil {
		return "", errors.New("no configured projects")
	}
	if _, ok := cfg.Projects[opts.Project]; !ok {
		return "", fmt.Errorf("unknown project %q (configured: %s)", opts.Project, strings.Join(cfg.SortedProjectNames(), ", "))
	}
	if strings.TrimSpace(opts.Query) == "" {
		return "", errors.New("fetch requires a query")
	}
	if len(store.TokenizeSearchQuery(opts.Query)) == 0 {
		return "", errors.New("fetch query contains no searchable terms after removing task-framing stopwords")
	}
	typ, err := parseNoteType(opts.Type)
	if err != nil {
		return "", err
	}
	if _, err := parseDateFlag("since", opts.Since); err != nil {
		return "", err
	}
	if _, err := parseDateFlag("until", opts.Until); err != nil {
		return "", err
	}
	if opts.Since != "" && opts.Until != "" && opts.Since > opts.Until {
		return "", fmt.Errorf("invalid date range: --since %s is after --until %s", opts.Since, opts.Until)
	}
	return typ, nil
}

func parseNoteType(value string) (models.NoteType, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	typ := models.NoteType(value)
	switch typ {
	case models.NoteTypeDailyWork, models.NoteTypeMeeting, models.NoteTypeKnowledge, models.NoteTypeProjectState:
		return typ, nil
	default:
		return "", fmt.Errorf("invalid note type %q (supported: daily_work, knowledge, meeting, project_state)", value)
	}
}

func parseDateFlag(name, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --%s date %q (expected YYYY-MM-DD)", name, value)
	}
	return parsed, nil
}
