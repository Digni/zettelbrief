package store

import (
	"database/sql"
	"testing"

	"github.com/cyphant/zettelbrief/internal/models"
)

func TestMigrationsUpsertStaleStatusAndFTS(t *testing.T) {
	db, err := Open(t.TempDir() + "/zettelbrief.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("idempotent migrate: %v", err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	runID, err := db.StartScanRunTx(tx, "Acme")
	if err != nil {
		t.Fatal(err)
	}
	note := models.Note{Project: "Acme", Type: models.NoteTypeKnowledge, SourcePath: "a.md", Content: "billable service update persistence", ContentHash: "h", Title: models.NullString("A"), SectionID: ""}
	if err := db.UpsertNoteTx(tx, note, runID); err != nil {
		t.Fatal(err)
	}
	daily := models.Note{Project: "Acme", Type: models.NoteTypeDailyWork, SourcePath: "daily.md", SectionID: "001-one", Repo: models.NullString("One"), Content: "daily", ContentHash: "d"}
	if err := db.UpsertNoteTx(tx, daily, runID); err != nil {
		t.Fatal(err)
	}
	meetingA := models.Note{Project: "Acme", Type: models.NoteTypeMeeting, SourcePath: "g.md", SectionID: "", GranolaID: models.NullString("g"), Content: "meeting", ContentHash: "g"}
	meetingB := meetingA
	meetingB.Project = "IReckonu"
	if err := db.UpsertNoteTx(tx, meetingA, runID); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertNoteTx(tx, meetingB, runID); err != nil {
		t.Fatal(err)
	}
	if err := db.CompleteScanRunTx(tx, runID, 3, 4); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	rowID, err := db.NoteRowID("Acme", "a.md", "")
	if err != nil {
		t.Fatal(err)
	}
	tx, _ = db.Begin()
	run2, _ := db.StartScanRunTx(tx, "Acme")
	note.Content = "updated content"
	if err := db.UpsertNoteTx(tx, note, run2); err != nil {
		t.Fatal(err)
	}
	removed, err := db.DeleteStaleNotesTx(tx, "Acme", run2)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	if err := db.CompleteScanRunTx(tx, run2, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	rowID2, err := db.NoteRowID("Acme", "a.md", "")
	if err != nil {
		t.Fatal(err)
	}
	if rowID != rowID2 {
		t.Fatalf("upsert should preserve row ID: %d vs %d", rowID, rowID2)
	}
	var ftsCount int
	if err := db.SQL.QueryRow(`SELECT COUNT(*) FROM notes_fts WHERE notes_fts MATCH ?`, "updated").Scan(&ftsCount); err != nil {
		t.Fatal(err)
	}
	if ftsCount != 1 {
		t.Fatalf("fts count=%d", ftsCount)
	}
	if err := db.RecordFailedScan("Acme", sql.ErrConnDone, 0, 0); err != nil {
		t.Fatal(err)
	}
	statuses, err := db.Status([]string{"Flive", "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	if statuses[0].Project != "Flive" || statuses[0].TotalNotes != 0 || statuses[1].TotalNotes != 1 || !statuses[1].LatestFailedScan.Valid {
		t.Fatalf("statuses = %#v", statuses)
	}
}
