package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cyphant/zettelbrief/internal/config"
	"github.com/cyphant/zettelbrief/internal/store"
)

func TestRunFetchWritesOutputs(t *testing.T) {
	cfg, dbPath := scanFetchFixture(t)
	outRoot := filepath.Join(t.TempDir(), "briefs")
	summary, err := RunFetch(cfg, FetchOptions{Project: "VetZ", Repo: "One.Backend", Query: "persistence", DBPath: dbPath, OutputRoot: outRoot, Now: fixedFetchTime})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(summary.OutputDir, "2026-05-01T06-42-13Z") || summary.Entries == 0 {
		t.Fatalf("summary = %#v", summary)
	}
	briefData, err := os.ReadFile(filepath.Join(summary.OutputDir, "brief.md"))
	if err != nil {
		t.Fatal(err)
	}
	briefText := string(briefData)
	if !strings.Contains(briefText, "## Relevant Prior Work") || !strings.Contains(briefText, "1.Projects/VetZ/Backend/architecture-overview.md") {
		t.Fatalf("brief.md = %s", briefText)
	}
	sourcesData, err := os.ReadFile(filepath.Join(summary.OutputDir, "sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	var sources struct {
		GeneratedAt string `json:"generated_at"`
		Entries     []struct {
			SourcePath   string `json:"source_path"`
			RowID        int64  `json:"row_id"`
			SectionID    string `json:"section_id"`
			BriefSection string `json:"brief_section"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(sourcesData, &sources); err != nil {
		t.Fatal(err)
	}
	if sources.GeneratedAt != "2026-05-01T06:42:13Z" || len(sources.Entries) == 0 || sources.Entries[0].SourcePath == "" || sources.Entries[0].RowID == 0 || sources.Entries[0].BriefSection == "" {
		t.Fatalf("sources = %#v", sources)
	}
	assertMode(t, summary.OutputDir, 0o700)
	assertMode(t, filepath.Join(summary.OutputDir, "brief.md"), 0o600)
	assertMode(t, filepath.Join(summary.OutputDir, "sources.json"), 0o600)
	if _, err := RunFetch(cfg, FetchOptions{Project: "VetZ", Query: "persistence", DBPath: dbPath, OutputRoot: outRoot, Now: fixedFetchTime}); err == nil {
		t.Fatalf("expected same-timestamp output collision to fail instead of overwriting")
	}
}

func TestRunFetchNoMatchesStillWritesEmptyOutputs(t *testing.T) {
	cfg, dbPath := scanFetchFixture(t)
	summary, err := RunFetch(cfg, FetchOptions{Project: "VetZ", Query: "zzznomatch", DBPath: dbPath, OutputRoot: filepath.Join(t.TempDir(), "briefs"), Now: fixedFetchTime})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(summary.OutputDir, "brief.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "No matching sources were found.") != 6 {
		t.Fatalf("brief = %s", data)
	}
	sourcesData, err := os.ReadFile(filepath.Join(summary.OutputDir, "sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	var sources struct {
		Entries []struct{} `json:"entries"`
	}
	if err := json.Unmarshal(sourcesData, &sources); err != nil {
		t.Fatal(err)
	}
	if len(sources.Entries) != 0 {
		t.Fatalf("expected empty sources, got %s", sourcesData)
	}
}

func TestRunFetchValidationAndMissingDatabaseDoNotCreateOutput(t *testing.T) {
	cfg := fetchConfig()
	outRoot := filepath.Join(t.TempDir(), "briefs")
	cases := []FetchOptions{
		{Project: "VetZ", Query: "persistence", Type: "unsupported", DBPath: filepath.Join(t.TempDir(), "missing.db"), OutputRoot: outRoot},
		{Project: "VetZ", Query: "persistence", Since: "not-a-date", DBPath: filepath.Join(t.TempDir(), "missing.db"), OutputRoot: outRoot},
		{Project: "VetZ", Query: "persistence", Since: "2026-05-01", Until: "2026-04-01", DBPath: filepath.Join(t.TempDir(), "missing.db"), OutputRoot: outRoot},
		{Project: "VetZ", DBPath: filepath.Join(t.TempDir(), "missing.db"), OutputRoot: outRoot},
		{Query: "persistence", DBPath: filepath.Join(t.TempDir(), "missing.db"), OutputRoot: outRoot},
		{Project: "VetZ", Query: "persistence", DBPath: filepath.Join(t.TempDir(), "missing.db"), OutputRoot: outRoot},
	}
	for _, opts := range cases {
		if _, err := RunFetch(cfg, opts); err == nil {
			t.Fatalf("expected error for %#v", opts)
		}
	}
	if _, err := os.Stat(outRoot); !os.IsNotExist(err) {
		t.Fatalf("output root should not exist after validation failures")
	}
}

func scanFetchFixture(t *testing.T) (config.Config, string) {
	t.Helper()
	cfg := fetchConfig()
	dbPath := filepath.Join(t.TempDir(), "zettelbrief.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := RunProjectScan("VetZ", cfg, db); err != nil {
		t.Fatal(err)
	}
	return cfg, dbPath
}

func fetchConfig() config.Config {
	return config.Config{VaultPath: filepath.Join("..", "..", "testdata", "vault"), Projects: map[string]config.ProjectConfig{"VetZ": {Folders: []string{"1.Projects/VetZ", "1.Projects/VetZ/Backend"}, Aliases: []string{"Vetz"}}}}
}

func fixedFetchTime() time.Time {
	return time.Date(2026, 5, 1, 6, 42, 13, 0, time.UTC)
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode=%#o want %#o", path, got, want)
	}
}
