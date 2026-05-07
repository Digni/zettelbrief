package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cyphant/zettelbrief/internal/config"
	"github.com/cyphant/zettelbrief/internal/store"
)

func TestMalformedFrontmatterDateWarningsArePathScoped(t *testing.T) {
	vault := filepath.Join(t.TempDir(), "vault")
	projectDir := filepath.Join(vault, "1.Projects", "Acme")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "bad.md"), []byte("---\ndate: not-a-date\n---\n# Bad\n\npersistence"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "good.md"), []byte("---\ndate: 2026-04-01\n---\n# Good\n\npersistence"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{VaultPath: vault, Projects: map[string]config.ProjectConfig{"Acme": {Folders: []string{"1.Projects/Acme"}}}}
	db, err := store.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	summary, err := RunProjectScanWithOptions("Acme", cfg, db, ScanOptions{Since: "2026-04-01"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(summary.Warnings, "\n")
	if !strings.Contains(joined, "1.Projects/Acme/bad.md: malformed date frontmatter date") || strings.Contains(joined, "not-a-date") {
		t.Fatalf("warnings=%#v", summary.Warnings)
	}
	if strings.Contains(joined, "good.md") {
		t.Fatalf("valid date should not warn: %#v", summary.Warnings)
	}
}

func TestRunProjectScanDateSliceIsInclusiveAndNonDestructive(t *testing.T) {
	vault := filepath.Join(t.TempDir(), "vault")
	projectDir := filepath.Join(vault, "1.Projects", "Acme")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "2026-04-01-old.md"), []byte("# Old\n\nold persistence"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "2026-05-01-new.md"), []byte("# New\n\nnew persistence"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "undated.md"), []byte("# Undated\n\nundated persistence"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{VaultPath: vault, Projects: map[string]config.ProjectConfig{"Acme": {Folders: []string{"1.Projects/Acme"}}}}
	db, err := store.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := RunProjectScan("Acme", cfg, db); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(projectDir, "2026-04-01-old.md")); err != nil {
		t.Fatal(err)
	}
	summary, err := RunProjectScanWithOptions("Acme", cfg, db, ScanOptions{Since: "2026-05-01"})
	if err != nil {
		t.Fatal(err)
	}
	if summary.RecordsUpserted != 1 || summary.StaleRemoved != 0 {
		t.Fatalf("date-sliced summary=%#v", summary)
	}
	count, err := db.CountNotes("Acme")
	if err != nil || count != 3 {
		t.Fatalf("date slice should preserve existing out-of-slice/undated rows, count=%d err=%v", count, err)
	}
	if _, err := RunProjectScanWithOptions("Acme", cfg, db, ScanOptions{Since: "bad-date"}); err == nil {
		t.Fatalf("expected invalid scan date error")
	}
	if _, err := RunProjectScanWithOptions("Acme", cfg, db, ScanOptions{Since: "2026-05-01", Until: "2026-04-01"}); err == nil {
		t.Fatalf("expected reversed scan date range error")
	}
}

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
