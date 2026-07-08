// Package models this handles all the crud logic for the sync database in the calendar_sync.db
package models

import (
	"time"

	"github.com/jmoiron/sqlx"
)

func GetGoogleEventID(db *sqlx.DB, taskID int64) (string, time.Time, error) {
	var row struct {
		GetGoogleEventID string    `db:"google_event_id"`
		LastSyncedAt     time.Time `db:"last_synced_at"`
	}
	query := `SELECT google_event_id,last_synced_at FROM task_google_events WHERE task_id = ?`
	err := db.Get(&row, query, taskID)
	return row.GetGoogleEventID, row.LastSyncedAt, err
}

func SaveGoogleEventMapping(db *sqlx.DB, taskID int64, googleEventID string) error {
	query := `INSERT OR REPLACE INTO task_google_events (task_id, google_event_id, last_synced_at)
						VALUES (:task_id, :google_event_id, :last_synced_at)`
	_, err := db.NamedExec(query, map[string]any{
		"task_id":         taskID,
		"google_event_id": googleEventID,
		"last_synced_at":  time.Now(),
	})
	return err
}

func UpdateLastSyncedAt(db *sqlx.DB, taskID int64) error {
	query := `UPDATE task_google_events SET last_synced_at = ? WHERE task_id = ?`
	_, err := db.Exec(query, time.Now(), taskID)
	return err
}

func DeleteGoogleEventMapping(db *sqlx.DB, taskID int64) error {
	query := `DELETE FROM task_google_events WHERE task_id = ?`
	_, err := db.Exec(query, taskID)
	return err
}
