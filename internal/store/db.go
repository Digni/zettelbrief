package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cyphant/zettelbrief/internal/models"
	_ "modernc.org/sqlite"
)

const CurrentSchemaVersion = 1

type DB struct {
	SQL *sql.DB
}

func Open(dbPath string) (*DB, error) {
	if dbPath == "" {
		dbPath = filepath.Join(".zettelbrief", "zettelbrief.db")
	}
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	_ = os.Chmod(dir, 0o700)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	wrapped := &DB{SQL: db}
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		_ = db.Close()
		return nil, err
	}
	_, _ = db.Exec(`PRAGMA journal_mode = WAL`)
	if err := wrapped.Migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	_ = os.Chmod(dbPath, 0o600)
	return wrapped, nil
}

func (db *DB) Close() error {
	if db == nil || db.SQL == nil {
		return nil
	}
	return db.SQL.Close()
}

func (db *DB) Migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT (datetime('now')));`,
		`CREATE TABLE IF NOT EXISTS scan_runs (id INTEGER PRIMARY KEY AUTOINCREMENT, project TEXT NOT NULL, started_at TEXT NOT NULL DEFAULT (datetime('now')), completed_at TEXT, status TEXT NOT NULL, error TEXT, files_seen INTEGER NOT NULL DEFAULT 0, notes_seen INTEGER NOT NULL DEFAULT 0);`,
		`CREATE TABLE IF NOT EXISTS notes (id INTEGER PRIMARY KEY AUTOINCREMENT, project TEXT NOT NULL, type TEXT NOT NULL, section_id TEXT NOT NULL DEFAULT '', repo TEXT, branch TEXT, date TEXT, title TEXT, summary TEXT, verification TEXT, notes_text TEXT, commit_hash TEXT, ticket TEXT, granola_id TEXT, updated_at TEXT, tags TEXT, source_path TEXT NOT NULL, content TEXT NOT NULL, content_hash TEXT NOT NULL, metadata_json TEXT, seen_in_scan_id INTEGER, scanned_at TEXT NOT NULL DEFAULT (datetime('now')), UNIQUE(project, source_path, section_id), FOREIGN KEY(seen_in_scan_id) REFERENCES scan_runs(id));`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(title, summary, content, tags, content='notes', content_rowid='id');`,
		`CREATE TRIGGER IF NOT EXISTS notes_ai AFTER INSERT ON notes BEGIN INSERT INTO notes_fts(rowid, title, summary, content, tags) VALUES (new.id, new.title, new.summary, new.content, new.tags); END;`,
		`CREATE TRIGGER IF NOT EXISTS notes_ad AFTER DELETE ON notes BEGIN INSERT INTO notes_fts(notes_fts, rowid, title, summary, content, tags) VALUES('delete', old.id, old.title, old.summary, old.content, old.tags); END;`,
		`CREATE TRIGGER IF NOT EXISTS notes_au AFTER UPDATE ON notes BEGIN INSERT INTO notes_fts(notes_fts, rowid, title, summary, content, tags) VALUES('delete', old.id, old.title, old.summary, old.content, old.tags); INSERT INTO notes_fts(rowid, title, summary, content, tags) VALUES (new.id, new.title, new.summary, new.content, new.tags); END;`,
		`INSERT OR IGNORE INTO schema_migrations(version) VALUES (1);`,
	}
	for _, stmt := range stmts {
		if _, err := db.SQL.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

func (db *DB) Begin() (*sql.Tx, error) { return db.SQL.Begin() }

func (db *DB) StartScanRunTx(tx *sql.Tx, project string) (int64, error) {
	res, err := tx.Exec(`INSERT INTO scan_runs(project, status) VALUES (?, 'running')`, project)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) CompleteScanRunTx(tx *sql.Tx, id int64, filesSeen, notesSeen int) error {
	_, err := tx.Exec(`UPDATE scan_runs SET status='completed', completed_at=datetime('now'), files_seen=?, notes_seen=? WHERE id=?`, filesSeen, notesSeen, id)
	return err
}

func (db *DB) RecordFailedScan(project string, err error, filesSeen, notesSeen int) error {
	summary := ""
	if err != nil {
		summary = err.Error()
	}
	_, execErr := db.SQL.Exec(`INSERT INTO scan_runs(project, status, completed_at, error, files_seen, notes_seen) VALUES (?, 'failed', datetime('now'), ?, ?, ?)`, project, summary, filesSeen, notesSeen)
	return execErr
}

func (db *DB) LatestCompleted(project string) (*models.ScanRun, error) {
	return db.scanRun(`SELECT id, project, started_at, completed_at, status, error, files_seen, notes_seen FROM scan_runs WHERE project=? AND status='completed' ORDER BY id DESC LIMIT 1`, project)
}

func (db *DB) LatestFailed(project string) (*models.ScanRun, error) {
	return db.scanRun(`SELECT id, project, started_at, completed_at, status, error, files_seen, notes_seen FROM scan_runs WHERE project=? AND status='failed' ORDER BY id DESC LIMIT 1`, project)
}

func (db *DB) scanRun(query, project string) (*models.ScanRun, error) {
	var run models.ScanRun
	err := db.SQL.QueryRow(query, project).Scan(&run.ID, &run.Project, &run.StartedAt, &run.CompletedAt, &run.Status, &run.Error, &run.FilesSeen, &run.NotesSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func DBExists(path string) bool {
	if path == "" {
		path = filepath.Join(".zettelbrief", "zettelbrief.db")
	}
	_, err := os.Stat(path)
	return err == nil
}

func sanitizeFTSQuery(q string) string {
	return strings.ReplaceAll(q, `"`, `""`)
}
