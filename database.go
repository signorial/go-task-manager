package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite" // the underscore means the functions aren't accessed directly but provides the driver for the database/sql import
)

type Task struct {
	TaskID         string    `db:"task_id"`
	Description    string    `db:"description"`
	Status         string    `db:"status"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	Priority       string    `db:"priority"`
	AssigneeID     int64     `db:"assignee_id"`
	DoDate         time.Time `db:"do_date"`
	FinalDueDate   time.Time `db:"final_due_date"`
	StartTime      time.Time `db:"start_time"`
	EndTime        time.Time `db:"end_time"`
	CompletedAt    time.Time `db:"completed_at"`
	EstimatedHours float64   `db:"estimated_hours"`
	Progress       int64     `db:"progress"`
	ParentTaskID   int64     `db:"parent_task_id"`
}

func StartDatabase() *sqlx.DB {
	db, err := sqlx.Open("sqlite", "./calendar_sync.db") // connect to the database
	if err != nil {
		log.Fatal(err) // exit if connection fails
	}

	return db
	// defer db.Close() // close the database when main() finishes
}

func listTasks(db *sqlx.DB) ([]Task, error) {
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

func DeleteTask(db *sqlx.DB, task_id int64) error {
	query := `DELETE FROM tasks WHERE task_id = ?`
	_, err := db.Exec(query, task_id) // runs the query and stores the rows in rows variable
	return err
}

func GetTask(db *sqlx.DB, task_id int64) (Task, error) {
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
	err := db.QueryRow(query, task_id).Scan(
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
			return t, fmt.Errorf("task with task_id %d not found", task_id) // returns row missing error
		}
		return t, err // any other database error
	}
	return t, nil
}

func AddTask(db *sqlx.DB, task Task) (int64, error) {
	query := `INSERT INTO tasks ( 
								task_id,description, status, created_at,updated_at,priority, 
								assignee_id,do_date,final_due_date,start_time,end_time, 
								completed_at,estimated_hours,progress,parent_task_id)
						VALUES ( 
								task_id, :description, :status, :created_at, :updated_at, :priority,  
								:assignee_id, :do_date, :final_due_date, :start_time, :end_time, 
								:completed_at, :estimated_hours, :progress, :parent_task_id)`
	result, err := db.NamedExec(query, task)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func CompleteTask(db *sqlx.DB, task Task) (int64, error) {
	query := `UPDATE tasks ( 
								task_id,description, status, created_at,updated_at,priority, 
								assignee_id,do_date,final_due_date,start_time,end_time, 
								completed_at,estimated_hours,progress,parent_task_id)
						VALUES ( 
								task_id, :description, :status, :created_at, :updated_at, :priority,  
								:assignee_id, :do_date, :final_due_date, :start_time, :end_time, 
								:completed_at, :estimated_hours, :progress, :parent_task_id)`
	result, err := db.NamedExec(query, task)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}
