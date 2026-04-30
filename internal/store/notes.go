package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/cyphant/zettelbrief/internal/models"
)

func (db *DB) UpsertNoteTx(tx *sql.Tx, note models.Note, scanRunID int64) error {
	tags, err := models.TagsJSON(note.Tags)
	if err != nil {
		return err
	}
	metadata := sql.NullString{String: string(note.MetadataJSON), Valid: len(note.MetadataJSON) > 0}
	_, err = tx.Exec(`
INSERT INTO notes(project, type, section_id, repo, branch, date, title, summary, verification, notes_text, commit_hash, ticket, granola_id, updated_at, tags, source_path, content, content_hash, metadata_json, seen_in_scan_id, scanned_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
ON CONFLICT(project, source_path, section_id) DO UPDATE SET
  type=excluded.type,
  repo=excluded.repo,
  branch=excluded.branch,
  date=excluded.date,
  title=excluded.title,
  summary=excluded.summary,
  verification=excluded.verification,
  notes_text=excluded.notes_text,
  commit_hash=excluded.commit_hash,
  ticket=excluded.ticket,
  granola_id=excluded.granola_id,
  updated_at=excluded.updated_at,
  tags=excluded.tags,
  content=excluded.content,
  content_hash=excluded.content_hash,
  metadata_json=excluded.metadata_json,
  seen_in_scan_id=excluded.seen_in_scan_id,
  scanned_at=datetime('now')`,
		note.Project, string(note.Type), note.SectionID, note.Repo, note.Branch, note.Date, note.Title, note.Summary, note.Verification, note.NotesText, note.CommitHash, note.Ticket, note.GranolaID, note.UpdatedAt, tags, note.SourcePath, note.Content, note.ContentHash, metadata, scanRunID)
	if err != nil {
		return fmt.Errorf("upsert note %s/%s: %w", note.Project, note.SourcePath, err)
	}
	return nil
}

func (db *DB) DeleteStaleNotesTx(tx *sql.Tx, project string, scanRunID int64) (int64, error) {
	res, err := tx.Exec(`DELETE FROM notes WHERE project=? AND (seen_in_scan_id IS NULL OR seen_in_scan_id <> ?)`, project, scanRunID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (db *DB) CountNotes(project string) (int, error) {
	var count int
	err := db.SQL.QueryRow(`SELECT COUNT(*) FROM notes WHERE project=?`, project).Scan(&count)
	return count, err
}

func (db *DB) NoteRowID(project, sourcePath, sectionID string) (int64, error) {
	var id int64
	err := db.SQL.QueryRow(`SELECT id FROM notes WHERE project=? AND source_path=? AND section_id=?`, project, sourcePath, sectionID).Scan(&id)
	return id, err
}

func DecodeTags(raw string) ([]string, error) {
	var tags []string
	if raw == "" {
		return tags, nil
	}
	return tags, json.Unmarshal([]byte(raw), &tags)
}
