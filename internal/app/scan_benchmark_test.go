package app

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/cyphant/zettelbrief/internal/config"
	"github.com/cyphant/zettelbrief/internal/store"
)

func BenchmarkScanPerformanceFixture(b *testing.B) {
	cfg := config.Config{VaultPath: filepath.Join("..", "..", "testdata", "scan-benchmark"), Projects: map[string]config.ProjectConfig{"Acme": {Folders: []string{"1.Projects/Acme"}, Aliases: []string{"Acme"}}}}
	b.ReportMetric(20, "total_files")
	b.ReportMetric(10, "granola_pct")
	b.ReportMetric(90, "unchanged_pct")
	b.ReportMetric(10, "changed_pct")
	for i := 0; i < b.N; i++ {
		db, err := store.Open(filepath.Join(b.TempDir(), fmt.Sprintf("bench-%d.sqlite", i)))
		if err != nil {
			b.Fatal(err)
		}
		if _, err := RunProjectScan("Acme", cfg, db); err != nil {
			_ = db.Close()
			b.Fatal(err)
		}
		_ = db.Close()
	}
}
