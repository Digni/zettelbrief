package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

type NoteType string

const (
	NoteTypeDailyWork    NoteType = "daily_work"
	NoteTypeMeeting      NoteType = "meeting"
	NoteTypeKnowledge    NoteType = "knowledge"
	NoteTypeProjectState NoteType = "project_state"
)

type Note struct {
	ID           int64           `json:"id,omitempty"`
	Project      string          `json:"project"`
	Type         NoteType        `json:"type"`
	SectionID    string          `json:"section_id"`
	Repo         sql.NullString  `json:"repo"`
	Branch       sql.NullString  `json:"branch"`
	Date         sql.NullString  `json:"date"`
	Title        sql.NullString  `json:"title"`
	Summary      sql.NullString  `json:"summary"`
	Verification sql.NullString  `json:"verification"`
	NotesText    sql.NullString  `json:"notes_text"`
	CommitHash   sql.NullString  `json:"commit_hash"`
	Ticket       sql.NullString  `json:"ticket"`
	GranolaID    sql.NullString  `json:"granola_id"`
	UpdatedAt    sql.NullString  `json:"updated_at"`
	Tags         []string        `json:"tags"`
	SourcePath   string          `json:"source_path"`
	Content      string          `json:"content"`
	ContentHash  string          `json:"content_hash"`
	MetadataJSON json.RawMessage `json:"metadata_json,omitempty"`
	SeenInScanID sql.NullInt64   `json:"seen_in_scan_id"`
	ScannedAt    time.Time       `json:"scanned_at"`
}

type NoteMetadata struct {
	SectionID    string                 `json:"section_id,omitempty"`
	Repo         string                 `json:"repo,omitempty"`
	Branch       string                 `json:"branch,omitempty"`
	Date         string                 `json:"date,omitempty"`
	Title        string                 `json:"title,omitempty"`
	Summary      string                 `json:"summary,omitempty"`
	Verification string                 `json:"verification,omitempty"`
	NotesText    string                 `json:"notes_text,omitempty"`
	CommitHash   string                 `json:"commit_hash,omitempty"`
	Ticket       string                 `json:"ticket,omitempty"`
	GranolaID    string                 `json:"granola_id,omitempty"`
	UpdatedAt    string                 `json:"updated_at,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	Extra        map[string]interface{} `json:"extra,omitempty"`
}

type DailyWorkSection struct {
	Index     int
	Heading   string
	SectionID string
	Content   string
}

type ScanRun struct {
	ID          int64          `json:"id"`
	Project     string         `json:"project"`
	StartedAt   string         `json:"started_at"`
	CompletedAt sql.NullString `json:"completed_at"`
	Status      string         `json:"status"`
	Error       sql.NullString `json:"error"`
	FilesSeen   int            `json:"files_seen"`
	NotesSeen   int            `json:"notes_seen"`
}

type ProjectStatus struct {
	Project               string           `json:"project"`
	TotalNotes            int              `json:"total_notes"`
	TypeCounts            map[NoteType]int `json:"type_counts"`
	LatestCompletedScan   sql.NullString   `json:"latest_completed_scan"`
	LatestFailedScan      sql.NullString   `json:"latest_failed_scan"`
	LatestFailedScanError sql.NullString   `json:"latest_failed_scan_error"`
}

func NullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func TagsJSON(tags []string) (string, error) {
	if tags == nil {
		tags = []string{}
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func MetadataRaw(extra map[string]interface{}) (json.RawMessage, error) {
	if len(extra) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(extra)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
