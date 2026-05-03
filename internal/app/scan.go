package app

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cyphant/zettelbrief/internal/config"
	"github.com/cyphant/zettelbrief/internal/models"
	scanpkg "github.com/cyphant/zettelbrief/internal/scan"
	"github.com/cyphant/zettelbrief/internal/store"
)

type ScanSummary struct {
	Project         string
	FilesDiscovered int
	RecordsUpserted int
	StaleRemoved    int64
	Warnings        []string
}

func RunProjectScan(project string, cfg config.Config, db *store.DB) (ScanSummary, error) {
	summary := ScanSummary{Project: project}
	if err := cfg.ValidateForScan(project); err != nil {
		return summary, err
	}
	notes, filesSeen, warnings, err := collectProjectNotes(project, cfg)
	summary.FilesDiscovered = filesSeen
	summary.Warnings = append(summary.Warnings, warnings...)
	if err != nil {
		_ = db.RecordFailedScan(project, err, filesSeen, len(notes))
		return summary, err
	}
	tx, err := db.Begin()
	if err != nil {
		return summary, err
	}
	failTx := func(err error) (ScanSummary, error) {
		_ = tx.Rollback()
		_ = db.RecordFailedScan(project, err, filesSeen, len(notes))
		return summary, err
	}
	runID, err := db.StartScanRunTx(tx, project)
	if err != nil {
		return failTx(err)
	}
	for _, note := range notes {
		if err := db.UpsertNoteTx(tx, note, runID); err != nil {
			return failTx(err)
		}
		summary.RecordsUpserted++
	}
	removed, err := db.DeleteStaleNotesTx(tx, project, runID)
	if err != nil {
		return failTx(err)
	}
	summary.StaleRemoved = removed
	if err := db.CompleteScanRunTx(tx, runID, filesSeen, len(notes)); err != nil {
		return failTx(err)
	}
	if err := tx.Commit(); err != nil {
		_ = db.RecordFailedScan(project, err, filesSeen, len(notes))
		return summary, err
	}
	return summary, nil
}

func collectProjectNotes(project string, cfg config.Config) ([]models.Note, int, []string, error) {
	var warnings []string
	var files []string
	pc := cfg.Projects[project]
	for _, folder := range pc.Folders {
		walked, err := scanpkg.WalkVault(cfg.VaultPath, folder)
		if err != nil {
			return nil, len(files), warnings, fmt.Errorf("walk project folder %q: %w", folder, err)
		}
		files = append(files, walked...)
	}
	files = scanpkg.DedupePaths(files)
	var notes []models.Note
	for _, rel := range files {
		fileNotes, fileWarnings := parseProjectFile(project, cfg.VaultPath, rel)
		warnings = append(warnings, fileWarnings...)
		notes = append(notes, fileNotes...)
	}
	granolaFiles, err := scanpkg.WalkVault(cfg.VaultPath, "4.Granola")
	if err == nil {
		for _, rel := range granolaFiles {
			fileNotes, fileWarnings := parseGranolaFile(project, cfg, rel)
			warnings = append(warnings, fileWarnings...)
			notes = append(notes, fileNotes...)
		}
		files = append(files, granolaFiles...)
	} else if !os.IsNotExist(err) {
		warnings = append(warnings, fmt.Sprintf("4.Granola: %v", err))
	}
	return notes, len(scanpkg.DedupePaths(files)), warnings, nil
}

func parseProjectFile(project, vaultPath, rel string) ([]models.Note, []string) {
	content, fm, warnings, ok := readAndParse(vaultPath, rel)
	if !ok {
		return nil, warnings
	}
	typ := scanpkg.ClassifyType(rel, fm)
	return buildNotes(project, typ, rel, content, fm, &warnings), warnings
}

func parseGranolaFile(project string, cfg config.Config, rel string) ([]models.Note, []string) {
	content, fm, warnings, ok := readAndParse(cfg.VaultPath, rel)
	if !ok {
		return nil, warnings
	}
	folders, supported := scanpkg.NormalizeFrontmatterList(fm, "folders")
	if !supported {
		return nil, append(warnings, fmt.Sprintf("%s: unsupported folders frontmatter shape", rel))
	}
	if len(folders) == 0 {
		return nil, append(warnings, fmt.Sprintf("%s: missing folders frontmatter", rel))
	}
	aliases := map[string][]string{}
	for name, pc := range cfg.Projects {
		aliases[name] = pc.Aliases
	}
	matches := scanpkg.MatchGranolaProjects(folders, aliases)
	for _, value := range matches.Ambiguous {
		warnings = append(warnings, fmt.Sprintf("%s: ambiguous Granola folder %q", rel, value))
	}
	for _, value := range matches.Unmatched {
		warnings = append(warnings, fmt.Sprintf("%s: unmatched Granola folder %q", rel, value))
	}
	for _, matched := range matches.Matched {
		if matched == project {
			return buildNotes(project, models.NoteTypeMeeting, rel, content, fm, &warnings), warnings
		}
	}
	return nil, warnings
}

func readAndParse(vaultPath, rel string) (string, map[string]interface{}, []string, bool) {
	var warnings []string
	abs := filepath.Join(vaultPath, filepath.FromSlash(rel))
	content, err := scanpkg.ReadFile(abs, scanpkg.DefaultMaxNoteBytes)
	if err != nil {
		return "", nil, append(warnings, fmt.Sprintf("%s: %v", rel, err)), false
	}
	fm, err := scanpkg.ParseFrontmatter(content)
	if err != nil {
		return "", nil, append(warnings, fmt.Sprintf("%s: invalid frontmatter: %v", rel, err)), false
	}
	if _, ok := scanpkg.NormalizeFrontmatterList(fm, "tags"); !ok {
		warnings = append(warnings, fmt.Sprintf("%s: unsupported tags frontmatter shape", rel))
	}
	return content, fm, warnings, true
}

func buildNotes(project string, typ models.NoteType, rel, content string, fm map[string]interface{}, warnings *[]string) []models.Note {
	switch typ {
	case models.NoteTypeDailyWork:
		sections := scanpkg.SplitDailyWorkSections(content)
		notes := make([]models.Note, 0, len(sections))
		for _, section := range sections {
			meta, ok := scanpkg.ExtractDailyWork(section)
			if !ok {
				*warnings = append(*warnings, fmt.Sprintf("%s: daily work section %q skipped: missing Repo", rel, section.Heading))
				continue
			}
			meta.Date = scanpkg.ExtractDate(rel, fm, typ)
			notes = append(notes, noteFromMetadata(project, typ, rel, section.Content, meta))
		}
		return notes
	case models.NoteTypeMeeting:
		meta := scanpkg.ExtractMeeting(fm)
		if meta.Date == "" {
			meta.Date = scanpkg.ExtractDate(rel, fm, typ)
		}
		return []models.Note{noteFromMetadata(project, typ, rel, content, meta)}
	default:
		meta := scanpkg.ExtractGeneric(content, fm, rel)
		meta.Date = scanpkg.ExtractDate(rel, fm, typ)
		return []models.Note{noteFromMetadata(project, typ, rel, content, meta)}
	}
}

func noteFromMetadata(project string, typ models.NoteType, rel, content string, meta models.NoteMetadata) models.Note {
	metadata, _ := models.MetadataRaw(meta.Extra)
	return models.Note{
		Project:      project,
		Type:         typ,
		SectionID:    meta.SectionID,
		Repo:         models.NullString(meta.Repo),
		Branch:       models.NullString(meta.Branch),
		Date:         models.NullString(meta.Date),
		Title:        models.NullString(meta.Title),
		Summary:      models.NullString(meta.Summary),
		Verification: models.NullString(meta.Verification),
		NotesText:    models.NullString(meta.NotesText),
		CommitHash:   models.NullString(meta.CommitHash),
		Ticket:       models.NullString(meta.Ticket),
		GranolaID:    models.NullString(meta.GranolaID),
		UpdatedAt:    models.NullString(meta.UpdatedAt),
		Tags:         meta.Tags,
		SourcePath:   rel,
		Content:      content,
		ContentHash:  scanpkg.HashContent(content),
		MetadataJSON: metadata,
		SeenInScanID: sql.NullInt64{},
	}
}

func FormatSummary(summary ScanSummary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Project: %s\n", summary.Project)
	fmt.Fprintf(&b, "Files discovered: %d\n", summary.FilesDiscovered)
	fmt.Fprintf(&b, "Records inserted/updated: %d\n", summary.RecordsUpserted)
	fmt.Fprintf(&b, "Stale records removed: %d\n", summary.StaleRemoved)
	fmt.Fprintf(&b, "Warnings: %d\n", len(summary.Warnings))
	return strings.TrimRight(b.String(), "\n")
}
