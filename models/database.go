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
	EstimatedHours *float64   `db:"estimated_hours": json:"estimated_hours" jsonschema_description:"Estimated hours to complete the task based on task description"`
	Progress       *int64     `db:"progress":        json:"progress"        jsonschema_description:"Progress percentage (0-100)"`
	ParentTaskID   *int64     `db:"parent_task_id":  json:"parent_task_id"  jsonschema_description:"ID of parent task if this is a subtask. leave blank if this is not a subtask"`
	Deleted        bool       `db:"deleted":         json:"deleted"         jsonschema_description:"flags whether the task has been deleted"`
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
		start_time       DATETIME,
		end_time         DATETIME,
		completed_at     DATETIME,
		estimated_hours  FLOAT,
		progress         INTEGER,
		parent_task_id   INTEGER
		deleted INTEGER DEFAULT 0
	);`

	schema +=		`CREATE TABLE IF NOT EXISTS sync_meta (
			key TEXT PRIMARY KEY,
			value TEXT
		);`

	schema += `CREATE TABLE IF NOT EXISTS events (
			id 								TEXT PRIMARY KEY,
			summary 					TEXT,
			description 			TEXT,
			start_time 				TEXT,
			end_time 					TEXT,
			updated_at 				TEXT,
			update_tasks_db 		INTEGER DEFAULT 0,
			update_calendar 		INTEGER DEFAULT 0,
			deleted					 	INTEGER DEFAULT 0,
			FK_tasks_task_id	INTEGER
		  FOREIGN KEY (FK_tasks_task_id)
			REFERENCES tasks(task_id)
		);`
	
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}




	db.MustExec(schema) // creates table if it doesn't exist
	return db, err
}

func DBGetTasks(db *sqlx.DB) ([]Task, error) {
	var tasks []Task
	query := `SELECT * FROM tasks WHERE deleted = 0`
	err := db.Select(&tasks, query)
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

func DBGetTask(db *sqlx.DB, taskID int64) (Task, error) {
	var task Task
	query := `SELECT * FROM tasks WHERE task_id =?`
	err := db.Get(&task, query, taskID)
	if err != nil {
		return task, err
	}
	return task, nil
}

func DBDeleteTask(db *sqlx.DB, taskID int64) error {
	// check if task has already been deleted
	_, err := DBGetTask(db, taskID)
	if err != nil {
		return fmt.Errorf("wrong ID or task has already been deleted %d %v", taskID, err)
	}

	query := `UPDATE tasks SET deleted = 1 WHERE task_id=?`
	_, err = db.Exec(query, taskID)
	if err != nil {
		return err
	}
	return nil
}

func DBAddTask(db *sqlx.DB, task Task) (int64, error) {
	query := `INSERT INTO tasks (
                description, status, created_at, updated_at, priority,
                assignee_id, do_date, final_due_date, start_time, end_time,
                completed_at, estimated_hours, progress, parent_task_id,deleted
            ) VALUES (
                :description, :status, :created_at, :updated_at, :priority,
                :assignee_id, :do_date, :final_due_date, :start_time, :end_time,
              	:completed_at, :estimated_hours, :progress, :parent_task_id,:deleted
            )`

	result, err := db.NamedExec(query, task)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func DBCompleteTask(db *sqlx.DB, taskID int64) error {
	query := `UPDATE tasks SET status = "COMPLETED" WHERE task_id=?`

	_, err := db.Exec(query, taskID)
	if err != nil {
		return err
	}

	return nil
}
