package models

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite" // the underscore means the functions aren't accessed directly but provides the driver for the database/sql import
)

type Task struct {
	TaskID         *int64     `db:"task_id"`
	Description    string     `db:"description"`
	Status         string     `db:"status"`
	CreatedAt      *time.Time `db:"created_at"`
	UpdatedAt      *time.Time `db:"updated_at"`
	Priority       string     `db:"priority"`
	AssigneeID     *int64     `db:"assignee_id"`
	DoDate         *time.Time `db:"do_date"`
	FinalDueDate   *time.Time `db:"final_due_date"`
	StartTime      *time.Time `db:"start_time"`
	EndTime        *time.Time `db:"end_time"`
	CompletedAt    *time.Time `db:"completed_at"`
	EstimatedHours *float64   `db:"estimated_hours"`
	Progress       *int64     `db:"progress"`
	ParentTaskID   *int64     `db:"parent_task_id"`
}

func StartDatabase() *sqlx.DB {
	db, err := sqlx.Open("sqlite", "./calendar_sync.db?_parseTime=true") // connect to the database
	if err != nil {
		log.Fatal(err) // exit if connection fails
	}
	schema := `
				CREATE TABLE IF NOT EXISTS tasks (
					task_id         INTEGER PRIMARY KEY AUTOINCREMENT,
					description     TEXT NOT NULL,
					status          TEXT NOT NULL,
					created_at      DATETIME,
					updated_at      DATETIME,
					priority        TEXT,
					assignee_id     INTEGER,
					do_date         DATETIME,
					final_due_date  DATETIME,
					start_time      DATETIME,
					end_time        DATETIME,
					completed_at    DATETIME,
					estimated_hours INTEGER,
					progress        INTEGER,
					parent_task_id  INTEGER
				);`

	db.MustExec(schema)
	return db
	// defer db.Close() // close the database when main() finishes
}

func DBGetTasks(db *sqlx.DB) ([]Task, error) {
	var tasks []Task
	query := `SELECT * FROM tasks`
	rows, err := db.Query(query) // runs the query and stores the rows in rows variable
	if err != nil {
		return nil, err
	}
	defer rows.Close() // this closes the rows at the end to prevent memory leaks
	for rows.Next() {  // loop through each reacord and copy to struct
		var t Task
		if err := rows.Scan(
			&t.TaskID,
			&t.Description,
			&t.Status,
			&t.CreatedAt,
			&t.UpdatedAt,
			&t.Priority,
			&t.AssigneeID,
			&t.DoDate,
			&t.FinalDueDate,
			&t.StartTime,
			&t.EndTime,
			&t.CompletedAt,
			&t.EstimatedHours,
			&t.Progress,
			&t.ParentTaskID,
		); err != nil {
			return nil, err
		}
		tasks = append(tasks, t) // add the task struct to the tasks slice
	}
	// Check if any errors occurred during the iteration
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return tasks, err // Return the completed slice
}

func DBDeleteTask(db *sqlx.DB, taskID int64) error {
	slog.Debug("Entering DBDeleteTask")
	slog.Debug("taskID: %d", taskID)
	query := `DELETE FROM tasks WHERE task_id = ?`
	_, err := db.Exec(query, taskID)
	if err == nil {
		slog.Debug("delete action completed successfully")
	}
	slog.Debug("completed error %s", err)
	return err
}

func DBGetTask(db *sqlx.DB, taskID int64) (Task, error) {
	slog.Debug("ENTER DBGetTask")
	var t Task
	query := `SELECT task_id,
									description, 
									status,
									created_at,
									updated_at, 
									priority, 
									assignee_id, 
									do_date, 
									final_due_date, 
									start_time, 
									end_time, 
									completed_at, 
									estimated_hours, 
									progress, 
									parent_task_id 
						FROM tasks WHERE task_id = ?`
	err := db.QueryRow(query, taskID).Scan(
		&t.TaskID,
		&t.Description,
		&t.Status,
		&t.CreatedAt,
		&t.UpdatedAt,
		&t.Priority,
		&t.AssigneeID,
		&t.DoDate,
		&t.FinalDueDate,
		&t.StartTime,
		&t.EndTime,
		&t.CompletedAt,
		&t.EstimatedHours,
		&t.Progress,
		&t.ParentTaskID) // runs the query and stores the rows in rows variable
	if err != nil {
		if err == sql.ErrNoRows { // if error is because the row isn't found
			slog.Debug("DBGetTasks ERROR:  couldn't find any rows %v", err)
			return t, fmt.Errorf("task with task_id %d not found", taskID) // returns row missing error
		}
		slog.Debug("DBGetTasks ERROR: some other error %v", err)
		return t, err // any other database error
	}
	slog.Debug("EXIT dbgetDBGetTasks")
	return t, nil
}

func DBAddTask(db *sqlx.DB, task Task) int64 {
	query := `INSERT INTO tasks (
                description, status, created_at, updated_at, priority,
                assignee_id, do_date, final_due_date, start_time, end_time,
                completed_at, estimated_hours, progress, parent_task_id
            ) VALUES (
                :description, :status, :created_at, :updated_at, :priority,
                :assignee_id, :do_date, :final_due_date, :start_time, :end_time,
                :completed_at, :estimated_hours, :progress, :parent_task_id
            )`

	result, err := db.NamedExec(query, task)
	if err != nil {
		slog.Debug("ERROR: unable to add task %v", err)
		return 0
	}
	id, err := result.LastInsertId()
	slog.Debug("completed ad task %d %s", id, err)
	return id
}

// odels/database.go:165 msg="ERROR: unable to add task %v"
// !BADKEY="could not find name createdAt in
// models.Task{TaskID:(*int64)(nil), Description:\"\", Status:\"\", CreatedAt:<nil>, UpdatedAt:<nil>, Priority:\"\", AssigneeID:(*int64)(nil), DoDate:<nil>, FinalDueDate:<nil>, StartTime:<nil>, EndTime:<nil>, CompletedAt:<nil>, EstimatedHours:(*int64)(nil), Progress:(*int64)(nil), ParentTaskID:(*int64)(nil)}"

// type Task struct {
// 	TaskID         *int64     `db:"task_id"`
// 	Description    string     `db:"description"`
// 	Status         string     `db:"status"`
// 	CreatedAt      *time.Time `db:"created_at"`
// 	UpdatedAt      *time.Time `db:"updated_at"`
// 	Priority       string     `db:"priority"`
// 	AssigneeID     *int64     `db:"assignee_id"`
// 	DoDate         *time.Time `db:"do_date"`
// 	FinalDueDate   *time.Time `db:"final_due_date"`
// 	StartTime      *time.Time `db:"start_time"`
// 	EndTime        *time.Time `db:"end_time"`
// 	CompletedAt    *time.Time `db:"completed_at"`
// 	EstimatedHours *int64     `db:"estimated_hours"`
// 	Progress       *int64     `db:"progress"`
// 	ParentTaskID   *int64     `db:"parent_task_id"`

func DBCompleteTask(db *sqlx.DB, taskID int64) error {
	slog.Debug("Entering DBCompleteTask")
	slog.Debug("taskID: %d", taskID)
	query := `UPDATE tasks 
								SET status = "COMPLETED" 
								WHERE task_id = ?`
	_, err := db.Exec(query, taskID)
	slog.Debug("error: %d", err)
	if err != nil {
		return err
	}
	return nil
}
