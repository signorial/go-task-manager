package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jmoiron/sqlx"
	"github.com/lmittmann/tint"
	"github.com/lufraser/gotaskmanager/aitaskmanager"
	"github.com/lufraser/gotaskmanager/models"
	"github.com/rivo/tview"
	_ "modernc.org/sqlite"
)

// OneDarkProTheme creates a custom tview.Theme matching the Atom/VS Code style.
var OneDarkProTheme = tview.Theme{
	// Backgrounds
	PrimitiveBackgroundColor:    tcell.NewRGBColor(40, 44, 52), // #282c34 (Main Editor Bg)
	ContrastBackgroundColor:     tcell.NewRGBColor(33, 37, 43), // #21252b (Sidebar Bg)
	MoreContrastBackgroundColor: tcell.NewRGBColor(44, 50, 60), // #2c323c (Selection Highlight)

	// Borders and Accents
	BorderColor:   tcell.NewRGBColor(97, 175, 239),  // #61afef (One Dark Blue)
	TitleColor:    tcell.NewRGBColor(224, 108, 117), // #e06c75 (Soft Red)
	GraphicsColor: tcell.NewRGBColor(171, 178, 191), // #abb2bf (Standard White/Gray)

	// Typography / Text States
	PrimaryTextColor:   tcell.NewRGBColor(171, 178, 191), // #abb2bf (Main text)
	SecondaryTextColor: tcell.NewRGBColor(152, 195, 121), // #98c379 (Green / Active tabs)
	TertiaryTextColor:  tcell.NewRGBColor(92, 99, 112),   // #5c6370 (Comments / Muted gray)
	InverseTextColor:   tcell.NewRGBColor(40, 44, 52),    // #282c34 (Flipped dark contrast)

	// Actionable Text State Accents
	ContrastSecondaryTextColor: tcell.NewRGBColor(229, 192, 123), // #e5c07b (Yellow Accent)
}

func main() {
	tview.Styles = OneDarkProTheme
	// Setup logging
	logFile, err := os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	logger := slog.New(tint.NewHandler(logFile, &tint.Options{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	// Open database
	db, err := models.StartDatabase()
	defer db.Close()

	// Create tview app
	app := tview.NewApplication()

	// Main menu list
	menu := tview.NewList().
		AddItem("AI Task Manager", "", '1', nil).
		AddItem("Add Task", "", '2', nil).
		AddItem("List Tasks", "", '3', nil).
		AddItem("Complete Task", "", '4', nil).
		AddItem("Delete Task", "", '5', nil).
		AddItem("Quit", "", 'q', func() {
			app.Stop()
		})

	menu.SetBorder(true).SetTitle(" GO TASK MANAGER ")

	// Status bar at bottom
	statusBar := tview.NewTextView().
		SetText("u/d: navigate • enter: select • q: quit").
		SetTextAlign(tview.AlignCenter)

	// Layout: menu on top, status at bottom
	mainMenu := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(menu, 0, 1, true).
		AddItem(statusBar, 1, 0, false)

	// Set up menu selection handler
	menu.SetSelectedFunc(func(index int, name string, desc string, shortcut rune) {
		switch index {
		case 0: // AI Task Manager
			showAIChat(app, db, mainMenu)
		case 1: // Add Task
			showAddTaskForm(app, db, mainMenu, nil) // pass empty task for the add task form
		case 2: // List Tasks
			showTaskList(app, db, mainMenu)
		case 3: // Complete Task
			showCompleteTask(app, db, mainMenu)
		case 4: // Delete Task
			showDeleteTask(app, db, mainMenu)
		}
	})

	// Set up key handler for global keys
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			// Return to menu on ESC (handled by pages if needed)
		}
		return event
	})

	// Run the app
	if err := app.SetRoot(mainMenu, true).EnableMouse(true).Run(); err != nil {
		slog.Error("app error", "error", err)
		os.Exit(1)
	}
}

// showAIChat displays the AI chat interface
func showAIChat(app *tview.Application, db *sqlx.DB, prevPage tview.Primitive) {
	session, err := aitaskmanager.NewSession(db)
	if err != nil {
		modal := tview.NewModal().
			SetText(fmt.Sprintf("Failed to start AI session: %v", err)).
			AddButtons([]string{"OK"}).
			SetDoneFunc(func(int, string) { app.SetRoot(prevPage, true) })
		app.SetRoot(modal, true)
		return
	}

	chatView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			app.Draw()
		})
	chatView.SetBorder(true).SetTitle(" AI TASK MANAGER ")

	chatView.SetText("[yellow]AI Task Manager ready[white]\nType your request and press Enter.\n\n")

	inputField := tview.NewInputField().
		SetLabel("Enter Request: ").
		SetFieldWidth(0)

	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			input := strings.TrimSpace(inputField.GetText())
			if input != "" && session != nil {
				chatView.Write([]byte(fmt.Sprintf("[green]You:[white] %s\n", input)))
				response, err := session.Execute(input)
				if err != nil {
					chatView.Write([]byte(fmt.Sprintf("[red]Error:[white] %s\n", err.Error())))
				} else {
					chatView.Write([]byte(fmt.Sprintf("[cyan]Grok:[white] %s\n", response)))
				}
				inputField.SetText("")
			}
		}
	})

	// ESC to go back
	inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			app.SetRoot(prevPage, true)
			return nil
		}
		return event
	})

	// Status bar at bottom
	statusBar := tview.NewTextView().
		SetText("Type Request • enter: run • esc: Main Menu").
		SetTextAlign(tview.AlignCenter)

	chatFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(chatView, 0, 1, false).
		AddItem(inputField, 1, 0, true).
		AddItem(statusBar, 2, 0, false)

	app.SetRoot(chatFlex, true)
	inputField.SetFocusFunc(func() {
		app.SetFocus(inputField)
	})
}

// showAddTaskForm displays a form to add a new task
func showAddTaskForm(app *tview.Application, db *sqlx.DB, prevPage tview.Primitive, task *models.Task) {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" ADD NEW TASK ")

	// variables to hold the models task
	Priority, Status := 0, 0

	var Description, CreatedAt_str, UpdatedAt_str string
	var AssigneeID_str, DoDate_str, FinalDueDate_str, StartTime_str, EndTime_str string
	var CompletedAt_str, EstimatedHours_str, Progress_str, ParentTaskID_str string

	if task != nil {
		Description = task.Description
		// status index
		for i, s := range statusOptions {
			if s == task.Status {
				Status = i
			}
		}
		if task.CreatedAt != nil {
			CreatedAt_str = task.CreatedAt.Format("2006-01-02")
		}
		UpdatedAt_str = time.Now().Format("2006-01-02 15:04:05")
		// priority index
		for i, p := range []string{"Low", "Regular", "High", "Urgent"} {
			if p == task.Priority {
				Priority = i
			}
		}
		if task.AssigneeID != nil {
			AssigneeID_str = fmt.Sprintf("%d", *task.AssigneeID)
		}
		if task.DoDate != nil {
			DoDate_str = task.DoDate.Format("2006-01-02")
		}
		if task.FinalDueDate != nil {
			FinalDueDate_str = task.FinalDueDate.Format("2006-01-02")
		}
		if task.StartTime != nil {
			StartTime_str = task.StartTime.Format("2006-01-02")
		}
		if task.EndTime != nil {
			EndTime_str = task.EndTime.Format("2006-01-02")
		}
		if task.CompletedAt != nil {
			CompletedAt_str = task.CompletedAt.Format("2006-01-02")
		}
		if task.EstimatedHours != nil {
			EstimatedHours_str = fmt.Sprintf("%d", *task.EstimatedHours)
		}
		if task.Progress != nil {
			Progress_str = fmt.Sprintf("%d", *task.Progress)
		}
		if task.ParentTaskID != nil {
			ParentTaskID_str = fmt.Sprintf("%d", *task.ParentTaskID)
		}

	}

	// to support dropdown selections
	statusOptions := []string{"Pending", "In Progress", "Completed", "Blocked"}
	priorityOptions := []string{"Low", "Regular", "High", "Urgent"}

	form.AddInputField("Description", Description, 50, nil, func(text string) { Description = text })
	form.AddDropDown("Status", statusOptions, Status, func(option string, index int) { Status = option })
	form.AddInputField("Task Creation Date (YYYY-MM-DD)", CreatedAt_str, 13, nil, func(text string) { CreatedAt_str = text })
	form.AddInputField("Task Update Date (YYYY-MM-DD)", UpdatedAt_str, 13, nil, func(text string) { UpdatedAt_str = text })
	form.AddDropDown("Priority", priorityOptions, Priority, func(option string, index int) { Priority = option })
	form.AddInputField("Do Date (YYYY-MM-DD)", "", DoDate_str, nil, func(text string) { DoDate_str = text })
	form.AddInputField("Final Due Date (YYYY-MM-DD)", FinalDueDate_str, 13, nil, func(text string) { FinalDueDate_str = text })
	form.AddInputField("Completed At (YYYY-MM-DD)", CompletedAt_str, 13, nil, func(text string) { CompletedAt_str = text })
	form.AddInputField("Start Time (HH:MM)", "", 13, StartTime_str, func(text string) { StartTime_str = text })
	form.AddInputField("End Time (HH:MM)", "", 13, EndTime_str, func(text string) { EndTime_str = text })
	form.AddInputField("Estimated Hours", "", 5, EstimatedHours_str, func(text string) { EstimatedHours_str = text })
	form.AddInputField("Progress (%)", "", 3, Progress_str, func(text string) { Progress_str = text })
	form.AddInputField("Assignee ID", "", 10, AssigneeID_str, func(text string) { AssigneeID_str = text })
	form.AddInputField("Parent Task ID", "", 10, ParentTaskID_str, func(text string) { ParentTaskID_str = text })

	form.AddButton("Save", func() {
		task := models.Task{
			Description: description,
			Status:      status,
			Priority:    priority,
			CreatedAt:   ptr(time.Now()),
			UpdatedAt:   ptr(time.Time{}),
		}

		// Parse optional date/time fields
		if t, err := time.Parse("2006-01-02", doDateStr); err == nil {
			task.DoDate = &t
		}
		if t, err := time.Parse("2006-01-02", finalDueDateStr); err == nil {
			task.FinalDueDate = &t
		}
		if completedAtStr != "" {
			if t, err := time.Parse("2006-01-02", completedAtStr); err == nil {
				task.CompletedAt = &t
			}
		}
		if t, err := time.Parse("15:04", startTimeStr); err == nil {
			task.StartTime = &t
		}
		if t, err := time.Parse("15:04", endTimeStr); err == nil {
			task.EndTime = &t
		}
		if f, err := strconv.ParseFloat(estimatedStr, 64); err == nil {
			task.EstimatedHours = &f
		}
		if i, err := strconv.ParseInt(progressStr, 10, 64); err == nil {
			task.Progress = &i
		}
		if i, err := strconv.ParseInt(assigneeStr, 10, 64); err == nil {
			task.AssigneeID = &i
		}
		if i, err := strconv.ParseInt(parentStr, 10, 64); err == nil {
			task.ParentTaskID = &i
		}

		id, err := models.DBAddTask(db, task)
		if err != nil {
			errString := fmt.Sprintf("Error: %v", err)
			modal := tview.NewModal().
				SetText(errString).
				AddButtons([]string{"OK"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					app.SetRoot(prevPage, true)
				})
			app.SetRoot(modal, true)
		} else {
			modal := tview.NewModal().
				SetText(fmt.Sprintf("✅ Task added successfully! ID: %d", id)).
				AddButtons([]string{"OK"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					app.SetRoot(prevPage, true)
				})
			app.SetRoot(modal, true)
		}
	})

	form.AddButton("Cancel", func() {
		app.SetRoot(prevPage, true)
	})

	form.SetCancelFunc(func() {
		app.SetRoot(prevPage, true)
	})

	app.SetRoot(form, true)
}

// ptr is a helper to get pointers to values
func ptr[T any](v T) *T {
	return &v
}

// helper functions
func fmtTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

func fmtInt64(i *int64) string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%d", *i)
}

func fmtFloat64(f *float64) string {
	if f == nil {
		return ""
	}
	return fmt.Sprintf("%.1f", *f)
}

// showTaskList displays all tasks in a table
func showTaskList(app *tview.Application, db *sqlx.DB, prevPage tview.Primitive) {
	tasks, err := models.DBGetTasks(db)
	if err != nil {
		modal := tview.NewModal().
			SetText(fmt.Sprintf("Error fetching tasks: %v", err)).
			AddButtons([]string{"OK"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				app.SetRoot(prevPage, true)
			})
		app.SetRoot(modal, true)
		return
	}

	table := tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false)

		// Header
	headers := []string{"ID", "Description", "Status", "Priority", "Created", "Updated", "Assignee", "DoDate", "DueDate", "Start", "End", "Completed", "EstHrs", "Progress", "Parent"}
	for col, h := range headers {
		table.SetCell(0, col, tview.NewTableCell(h).SetTextColor(tcell.ColorYellow).SetSelectable(false))
	}
	for row, task := range tasks {
		table.SetCell(row+1, 0, tview.NewTableCell(fmt.Sprintf("%d", *task.TaskID)))
		table.SetCell(row+1, 1, tview.NewTableCell(task.Description))
		table.SetCell(row+1, 2, tview.NewTableCell(task.Status))
		table.SetCell(row+1, 3, tview.NewTableCell(task.Priority))
		table.SetCell(row+1, 4, tview.NewTableCell(fmtTime(task.CreatedAt)))
		table.SetCell(row+1, 5, tview.NewTableCell(fmtTime(task.UpdatedAt)))
		table.SetCell(row+1, 6, tview.NewTableCell(fmtInt64(task.AssigneeID)))
		table.SetCell(row+1, 7, tview.NewTableCell(fmtTime(task.DoDate)))
		table.SetCell(row+1, 8, tview.NewTableCell(fmtTime(task.FinalDueDate)))
		table.SetCell(row+1, 9, tview.NewTableCell(fmtTime(task.StartTime)))
		table.SetCell(row+1, 10, tview.NewTableCell(fmtTime(task.EndTime)))
		table.SetCell(row+1, 11, tview.NewTableCell(fmtTime(task.CompletedAt)))
		table.SetCell(row+1, 12, tview.NewTableCell(fmtFloat64(task.EstimatedHours)))
		table.SetCell(row+1, 13, tview.NewTableCell(fmtInt64(task.Progress)))
		table.SetCell(row+1, 14, tview.NewTableCell(fmtInt64(task.ParentTaskID)))
	}

	table.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			app.SetRoot(prevPage, true)
		}
	})

	// 3. Keep the top row (0) frozen/fixed at the top
	table.SetFixed(1, 0) // (rowsToFix, columnsToFix)

	table.SetBorder(true).SetTitle(" YOUR TASKS (ESC to return) ")

	app.SetRoot(table, true)
}

// showCompleteTask shows a form to mark a task complete
func showCompleteTask(app *tview.Application, db *sqlx.DB, prevPage tview.Primitive) {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" COMPLETE TASK ")

	var taskIDStr string
	form.AddInputField("Task ID", "", 10, nil, func(text string) { taskIDStr = text })
	form.AddButton("Complete", func() {
		taskIDint, err := strconv.ParseInt(strings.TrimSpace(taskIDStr), 10, 64)
		if err != nil {
			modal := tview.NewModal().
				SetText("Error: Invalid Task ID").
				AddButtons([]string{"OK"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					app.SetRoot(prevPage, true)
				})
			app.SetRoot(modal, true)
			return
		}

		err = models.DBCompleteTask(db, taskIDint)
		if err != nil {
			modal := tview.NewModal().
				SetText(fmt.Sprintf("Error completing task: %v", err)).
				AddButtons([]string{"OK"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					app.SetRoot(prevPage, true)
				})
			app.SetRoot(modal, true)
		} else {
			modal := tview.NewModal().
				SetText(fmt.Sprintf("✅ Marked task %d complete", taskIDint)).
				AddButtons([]string{"OK"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					app.SetRoot(prevPage, true)
				})
			app.SetRoot(modal, true)
		}
	})

	form.AddButton("Cancel", func() {
		app.SetRoot(prevPage, true)
	})

	form.SetCancelFunc(func() {
		app.SetRoot(prevPage, true)
	})

	app.SetRoot(form, true)
}

// showDeleteTask shows a form to delete a task
func showDeleteTask(app *tview.Application, db *sqlx.DB, prevPage tview.Primitive) {
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" DELETE TASK ")

	var taskIDStr string
	form.AddInputField("Task ID", "", 10, nil, func(text string) { taskIDStr = text })

	form.AddButton("Delete", func() {
		taskIDint, err := strconv.ParseInt(strings.TrimSpace(taskIDStr), 10, 64)
		if err != nil {
			modal := tview.NewModal().
				SetText("Error: Invalid Task ID").
				AddButtons([]string{"OK"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					app.SetRoot(prevPage, true)
				})
			app.SetRoot(modal, true)
			return
		}

		err = models.DBDeleteTask(db, taskIDint)
		if err != nil {
			modal := tview.NewModal().
				SetText(fmt.Sprintf("Error deleting task: %v", err)).
				AddButtons([]string{"OK"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					app.SetRoot(prevPage, true)
				})
			app.SetRoot(modal, true)
		} else {
			modal := tview.NewModal().
				SetText(fmt.Sprintf("✅ Deleted task %d", taskIDint)).
				AddButtons([]string{"OK"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					app.SetRoot(prevPage, true)
				})
			app.SetRoot(modal, true)
		}
	})

	form.AddButton("Cancel", func() {
		app.SetRoot(prevPage, true)
	})

	form.SetCancelFunc(func() {
		app.SetRoot(prevPage, true)
	})

	app.SetRoot(form, true)
}
