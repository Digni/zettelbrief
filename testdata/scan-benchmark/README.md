# Scan Benchmark Fixture

Representative warm-cache scan fixture for the `quality-polish` incremental scan gate.

- Total Markdown files: 20
- Granola files: 2 (10%)
- Project files: 18 (90%)
- Intended changed/unchanged scenario: 90% unchanged / 10% changed

## Baseline measurement

Command: `go test ./internal/app -run '^$' -bench BenchmarkScanPerformanceFixture -benchtime=5x -count=1`

Result on Apple M4 Max (2026-05-04): `BenchmarkScanPerformanceFixture-14  5  4432117 ns/op`.

The representative full scan median/mean proxy is well below the 2s gate for this fixture.
