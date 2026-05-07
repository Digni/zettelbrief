package store

import (
	"testing"

	"github.com/cyphant/zettelbrief/internal/models"
)

func TestContentHashLookupAndMarkSourcePathSeenAreProjectScoped(t *testing.T) {
	db := searchTestDB(t)
	insertSearchNote(t, db, models.Note{Project: "Acme", Type: models.NoteTypeKnowledge, SourcePath: "shared.md", SectionID: "acme", Title: models.NullString("Persistence"), Content: "billable persistence", ContentHash: "hash-acme"})
	insertSearchNote(t, db, models.Note{Project: "Beta", Type: models.NoteTypeKnowledge, SourcePath: "shared.md", SectionID: "beta", Title: models.NullString("Persistence"), Content: "billable persistence", ContentHash: "hash-beta"})
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	runID, err := db.StartScanRunTx(tx, "Acme")
	if err != nil {
		t.Fatal(err)
	}
	hash, ok, err := db.ContentHashForSourceTx(tx, "Acme", "shared.md")
	if err != nil || !ok || hash != "hash-acme" {
		t.Fatalf("hash=%q ok=%v err=%v", hash, ok, err)
	}
	if _, ok, err := db.ContentHashForSourceTx(tx, "Acme", "missing.md"); err != nil || ok {
		t.Fatalf("missing ok=%v err=%v", ok, err)
	}
	marked, err := db.MarkSourcePathSeenTx(tx, "Acme", "shared.md", runID)
	if err != nil {
		t.Fatal(err)
	}
	if marked != 1 {
		t.Fatalf("marked=%d", marked)
	}
	if err := db.CompleteScanRunTx(tx, runID, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	results, _, err := db.SearchNotes(SearchRequest{Project: "Acme", Query: "persistence"})
	if err != nil || len(results) != 1 {
		t.Fatalf("acme results=%#v err=%v", results, err)
	}
	var betaSeen int64
	if err := db.SQL.QueryRow(`SELECT seen_in_scan_id FROM notes WHERE project='Beta' AND source_path='shared.md'`).Scan(&betaSeen); err != nil {
		t.Fatal(err)
	}
	if betaSeen == runID {
		t.Fatalf("beta row was marked with Acme scan run: %#v", betaSeen)
	}
}
