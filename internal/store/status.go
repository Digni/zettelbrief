package store

import (
	"database/sql"
	"fmt"

	"github.com/cyphant/zettelbrief/internal/models"
)

func (db *DB) Status(projectNames []string) ([]models.ProjectStatus, error) {
	statuses := make([]models.ProjectStatus, 0, len(projectNames))
	for _, project := range projectNames {
		status := models.ProjectStatus{Project: project, TypeCounts: map[models.NoteType]int{}}
		if err := db.SQL.QueryRow(`SELECT COUNT(*) FROM notes WHERE project=?`, project).Scan(&status.TotalNotes); err != nil {
			return nil, fmt.Errorf("count notes for %s: %w", project, err)
		}
		rows, err := db.SQL.Query(`SELECT type, COUNT(*) FROM notes WHERE project=? GROUP BY type`, project)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var typ string
			var count int
			if err := rows.Scan(&typ, &count); err != nil {
				_ = rows.Close()
				return nil, err
			}
			status.TypeCounts[models.NoteType(typ)] = count
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		_ = db.SQL.QueryRow(`SELECT completed_at FROM scan_runs WHERE project=? AND status='completed' ORDER BY id DESC LIMIT 1`, project).Scan(&status.LatestCompletedScan)
		var failedAt, failedErr sql.NullString
		_ = db.SQL.QueryRow(`SELECT completed_at, error FROM scan_runs WHERE project=? AND status='failed' ORDER BY id DESC LIMIT 1`, project).Scan(&failedAt, &failedErr)
		status.LatestFailedScan = failedAt
		status.LatestFailedScanError = failedErr
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func EmptyStatus(projectNames []string) []models.ProjectStatus {
	statuses := make([]models.ProjectStatus, 0, len(projectNames))
	for _, project := range projectNames {
		statuses = append(statuses, models.ProjectStatus{Project: project, TypeCounts: map[models.NoteType]int{}})
	}
	return statuses
}
