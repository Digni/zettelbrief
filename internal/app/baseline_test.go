package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cyphant/zettelbrief/internal/config"
	"github.com/cyphant/zettelbrief/internal/store"
)

func TestBaselineFetchOutputShapeBeforeQualityPolish(t *testing.T) {
	cfg, dbPath := scanFetchFixture(t)
	summary, err := RunFetch(cfg, FetchOptions{Project: "Acme", Repo: "One.Backend", Query: "persistence", DBPath: dbPath, OutputRoot: filepath.Join(t.TempDir(), "briefs"), Now: fixedFetchTime})
	if err != nil {
		t.Fatal(err)
	}
	briefData, err := os.ReadFile(filepath.Join(summary.OutputDir, "brief.md"))
	if err != nil {
		t.Fatal(err)
	}
	briefText := string(briefData)
	if !strings.Contains(briefText, "## Relevant Prior Work") {
		t.Fatalf("brief missing expected section:\n%s", briefText)
	}
	sourcesData, err := os.ReadFile(filepath.Join(summary.OutputDir, "sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	var sources struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal(sourcesData, &sources); err != nil {
		t.Fatal(err)
	}
	if len(sources.Entries) == 0 {
		t.Fatalf("expected baseline source mappings")
	}
	for _, entry := range sources.Entries {
		for _, requiredField := range []string{"source_path", "row_id", "section_id", "brief_section"} {
			if _, ok := entry[requiredField]; !ok {
				t.Fatalf("source mapping missing %s: %s", requiredField, sourcesData)
			}
		}
	}
}

func TestBaselineScanFullRescanCleansRemovedRows(t *testing.T) {
	vault := filepath.Join(t.TempDir(), "vault")
	if err := os.MkdirAll(filepath.Join(vault, "1.Projects", "Acme"), 0o755); err != nil {
		t.Fatal(err)
	}
	notePath := filepath.Join(vault, "1.Projects", "Acme", "note.md")
	if err := os.WriteFile(notePath, []byte("# Note\n\nbillable persistence"), 0o600); err != nil {
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
	if count, err := db.CountNotes("Acme"); err != nil || count != 1 {
		t.Fatalf("initial count=%d err=%v", count, err)
	}
	if err := os.Remove(notePath); err != nil {
		t.Fatal(err)
	}
	summary, err := RunProjectScan("Acme", cfg, db)
	if err != nil {
		t.Fatal(err)
	}
	if summary.StaleRemoved != 1 {
		t.Fatalf("expected full rescan to remove stale row, summary=%#v", summary)
	}
}
