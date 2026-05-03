package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cyphant/zettelbrief/internal/config"
	"github.com/cyphant/zettelbrief/internal/store"
)

func TestRunProjectScanFixtureAndStaleCleanup(t *testing.T) {
	vault := filepath.Join("..", "..", "testdata", "vault")
	db, err := store.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := config.Config{VaultPath: vault, Projects: map[string]config.ProjectConfig{"Acme": {Folders: []string{"1.Projects/Acme", "1.Projects/Acme/Backend"}, Aliases: []string{"Acme"}}, "IReckonu": {Folders: []string{"1.Projects/IReckonu"}}}}
	summary, err := RunProjectScan("Acme", cfg, db)
	if err != nil {
		t.Fatal(err)
	}
	if summary.FilesDiscovered != 5 || summary.RecordsUpserted != 5 || len(summary.Warnings) == 0 {
		t.Fatalf("summary = %#v", summary)
	}
	count, err := db.CountNotes("Acme")
	if err != nil || count != 5 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	// Simulate a previously scanned row disappearing on the next successful full scan.
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	runID, _ := db.StartScanRunTx(tx, "Acme")
	if _, err := tx.Exec(`INSERT INTO notes(project, type, section_id, source_path, content, content_hash, seen_in_scan_id) VALUES ('Acme', 'knowledge', '', 'deleted.md', 'deleted', 'h', ?)`, runID); err != nil {
		t.Fatal(err)
	}
	_ = db.CompleteScanRunTx(tx, runID, 1, 1)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	summary, err = RunProjectScan("Acme", cfg, db)
	if err != nil {
		t.Fatal(err)
	}
	if summary.StaleRemoved == 0 {
		t.Fatalf("expected stale row to be removed")
	}
	if _, err := os.Stat(filepath.Join("..", "..", "testdata", "vault", "1.Projects", "Acme", "image.png")); err != nil {
		t.Fatal(err)
	}
}
