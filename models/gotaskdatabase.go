package models

import ( // logging
	// easier sql mapping
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite" // underscore means not accessed directly but used for the database/sql driver
)

type Task struct {
	TaskID         *int64     `db:"task_id":         json:"task_id"         jsonschema_description:"Unique identifier for the task"`
	Description    string     `db:"description":     json:"description"     jsonschema_description:"Detailed task description"`
	Status         string     `db:"status":          json:"status"          jsonschema_description:"current status: (Pending, In Progress,COMPLETED"`
	CreatedAt      *time.Time `db:"created_at":      json:"created_at"      jsonschema_description:"when the task was created. RFC3339 format. eg. 2026-04-30T23:59:59Z"`
	UpdatedAt      *time.Time `db:"updated_at":      json:"updated_at"      jsonschema_description:"when the task was last updated. RFC3339 format. eg. 2026-04-30T23:59:59Z"`
	Priority       string     `db:"priority":        json:"priority"        jsonschema_description:"priority level: (Low,Regular,High)"`
	AssigneeID     *int64     `db:"assignee_id":     json:"assignee_id"     jsonschema_description:"ID of the person that is assiged to the task."`
	DoDate         *time.Time `db:"do_date":         json:"do_date"         jsonschema_description:"This is the planned completion date it will always be lower than FinalDueDate as it allows for time to review before final submission. RFC3339 format. eg. 2026-04-30T23:59:59Z"`
	FinalDueDate   *time.Time `db:"final_due_date":  json:"final_due_date"  jsonschema_description:"Final deadline for completion. RFC3339 format. eg. 2026-04-30T23:59:59Z"`
	StartTime      *time.Time `db:"start_time":      json:"start_time"      jsonschema_description:"The time that the user is scheduled to star working on the task . RFC3339 format. eg. 2026-04-30T23:59:59Z"`
	EndTime        *time.Time `db:"end_time":        json:"end_time"        jsonschema_description:"The time that the user is scheduled to star working on the task . RFC3339 format. eg. 2026-04-30T23:59:59Z"`
	CompletedAt    *time.Time `db:"completed_at":    json:"completed_at"    jsonschema_description:"The time that the user is scheduled to  . RFC3339 format. eg. 2026-04-30T23:59:59Z"`
	EstimatedHours *int64     `db:"estimated_hours": json:"estimated_hours" jsonschema_description:"Estimated hours to complete the task based on task description"`
	Progress       *int64     `db:"progress":        json:"progress"        jsonschema_description:"Progress percentage (0-100)"`
	ParentTaskID   *int64     `db:"parent_task_id":  json:"parent_task_id"  jsonschema_description:"ID of parent task if this is a subtask. leave blank if this is not a subtask"`
}

func StartDatabase() (*sqlx.DB, error) {
	db, err := sqlx.Open("sqlite", "./calendar_sync.db?_parseTime=true")
	if err != nil {
		return db, fmt.Errorf("error starting database %v", err)
	}

	schema := `

		CREATE TABLE IF NOT EXISTS tasks(
		task_id          INTEGER   PRIMARY KEY AUTOINCREMENT,
		description      TEXT      NOT NULL,
		status           TEXT      NOT NULL,
		created_at       DATETIME,
		updated_at       DATETIME,
		priority         TEXT,
		assignee_id      INTEGER,
		do_date          DATETIME,
		final_due_date   DATETIME,
		StartTime        DATETIME,
		end_time         DATETIME,
		completed_at     DATETIME,
		estimated_hours  INTEGER,
		progress         INTEGER,
		parent_task_id   INTEGER

	);
	`
	return db, err
}
