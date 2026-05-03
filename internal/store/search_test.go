package store

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/cyphant/zettelbrief/internal/models"
)

func TestSearchNotesSafeInputAndCitationFields(t *testing.T) {
	db := searchTestDB(t)
	insertSearchNote(t, db, models.Note{Project: "VetZ", Type: models.NoteTypeKnowledge, SourcePath: "knowledge.md", SectionID: "", Title: models.NullString("Billable"), Summary: models.NullString("service update"), Content: "billable service update persistence", ContentHash: "h", Tags: []string{"backend"}})
	results, tokens, err := db.SearchNotes(SearchRequest{Project: "VetZ", Query: `"billable" service:update - persistence`})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(tokens) != 4 {
		t.Fatalf("tokens = %#v", tokens)
	}
	if len(results) != 1 {
		t.Fatalf("results = %#v", results)
	}
	got := results[0]
	if got.ID == 0 || got.SourcePath != "knowledge.md" || got.SectionID != "" || got.Type != models.NoteTypeKnowledge || len(got.Tags) != 1 || got.Tags[0] != "backend" {
		t.Fatalf("result missing routing/citation fields: %#v", got)
	}
}

func TestSearchNotesRejectsEmptyQueryAndProject(t *testing.T) {
	db := searchTestDB(t)
	if _, _, err := db.SearchNotes(SearchRequest{Project: "VetZ", Query: `:"" ---`}); !errors.Is(err, ErrInvalidSearch) {
		t.Fatalf("empty query err = %v", err)
	}
	if _, _, err := db.SearchNotes(SearchRequest{Query: "persistence"}); !errors.Is(err, ErrInvalidSearch) {
		t.Fatalf("missing project err = %v", err)
	}
}

func TestSearchNotesMetadataFilters(t *testing.T) {
	db := searchTestDB(t)
	insertSearchNote(t, db, noteForFilters("VetZ", models.NoteTypeDailyWork, "backend-old.md", "One.Backend", "2026-04-01"))
	insertSearchNote(t, db, noteForFilters("VetZ", models.NoteTypeDailyWork, "backend-edge.md", "One.Backend", "2026-04-30"))
	insertSearchNote(t, db, noteForFilters("VetZ", models.NoteTypeKnowledge, "backend-wrong-type.md", "One.Backend", "2026-04-15"))
	insertSearchNote(t, db, noteForFilters("VetZ", models.NoteTypeDailyWork, "backend-too-late.md", "One.Backend", "2026-05-01"))
	insertSearchNote(t, db, noteForFilters("VetZ", models.NoteTypeDailyWork, "frontend.md", "One.Frontend", "2026-04-15"))
	insertSearchNote(t, db, noteForFilters("VetZ", models.NoteTypeKnowledge, "project.md", "", ""))
	insertSearchNote(t, db, noteForFilters("Flive", models.NoteTypeDailyWork, "other.md", "One.Backend", "2026-04-15"))

	results, _, err := db.SearchNotes(SearchRequest{Project: "VetZ", Repo: "One.Backend", Type: models.NoteTypeDailyWork, Since: "2026-04-01", Until: "2026-04-30", Query: "persistence"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("combined results = %#v", results)
	}
	for _, result := range results {
		if result.Project != "VetZ" || result.Type != models.NoteTypeDailyWork || !result.Repo.Valid || result.Repo.String != "One.Backend" || !result.Date.Valid {
			t.Fatalf("unexpected filtered result: %#v", result)
		}
	}

	results, _, err = db.SearchNotes(SearchRequest{Project: "VetZ", Repo: "One.Backend", Query: "persistence"})
	if err != nil {
		t.Fatal(err)
	}
	seenProjectLevel := false
	for _, result := range results {
		if result.Repo.Valid && result.Repo.String == "One.Frontend" {
			t.Fatalf("frontend repo should be excluded: %#v", results)
		}
		if !result.Repo.Valid || result.Repo.String == "" {
			seenProjectLevel = true
		}
	}
	if !seenProjectLevel {
		t.Fatalf("expected project-level row in %#v", results)
	}

	results, _, err = db.SearchNotes(SearchRequest{Project: "VetZ", Since: "2026-04-01", Query: "persistence"})
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if !result.Date.Valid || result.Date.String == "" {
			t.Fatalf("date filter returned undated note: %#v", results)
		}
	}
}

func TestSearchNotesIncludesNotesText(t *testing.T) {
	db := searchTestDB(t)
	insertSearchNote(t, db, models.Note{Project: "VetZ", Type: models.NoteTypeDailyWork, SourcePath: "notes.md", SectionID: "001", Repo: models.NullString("One.Backend"), Title: models.NullString("One.Backend"), NotesText: models.NullString("follow-up persistence checks"), Content: "no query term here", ContentHash: "n"})
	results, _, err := db.SearchNotes(SearchRequest{Project: "VetZ", Query: "persistence"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].NotesText.Valid {
		t.Fatalf("notes_text search results = %#v", results)
	}
}

func TestSearchNotesDailyWorkIsSectionSpecific(t *testing.T) {
	db := searchTestDB(t)
	insertSearchNote(t, db, models.Note{Project: "VetZ", Type: models.NoteTypeDailyWork, SourcePath: "daily.md", SectionID: "001-one-backend", Repo: models.NullString("One.Backend"), Title: models.NullString("One.Backend"), Summary: models.NullString("API cleanup"), Content: "repo one backend api cleanup", ContentHash: "a"})
	insertSearchNote(t, db, models.Note{Project: "VetZ", Type: models.NoteTypeDailyWork, SourcePath: "daily.md", SectionID: "002-one-frontend", Repo: models.NullString("One.Frontend"), Title: models.NullString("One.Frontend"), Summary: models.NullString("billable persistence UI"), Content: "billable persistence UI", ContentHash: "b"})
	results, _, err := db.SearchNotes(SearchRequest{Project: "VetZ", Query: "billable persistence"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].SectionID != "002-one-frontend" {
		t.Fatalf("section-specific results = %#v", results)
	}
}

func searchTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(t.TempDir() + "/zettelbrief.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func insertSearchNote(t *testing.T, db *DB, note models.Note) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	runID, err := db.StartScanRunTx(tx, note.Project)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertNoteTx(tx, note, runID); err != nil {
		t.Fatal(err)
	}
	if err := db.CompleteScanRunTx(tx, runID, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func noteForFilters(project string, typ models.NoteType, path, repo, date string) models.Note {
	return models.Note{Project: project, Type: typ, SourcePath: path, SectionID: path, Repo: models.NullString(repo), Date: models.NullString(date), Title: models.NullString(path), Summary: models.NullString("persistence"), Content: "persistence", ContentHash: path, NotesText: sql.NullString{}}
}
