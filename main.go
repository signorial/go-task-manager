package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/jmoiron/sqlx"
	"github.com/lufraser/gotaskmanager/models"
	_ "modernc.org/sqlite" // import driver for database/sql to use
)

// Define styles using Lip Gloss
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			MarginBottom(1)
	itemStyle = lipgloss.NewStyle().
			PaddingLeft(2)
	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(0).
				Foreground(lipgloss.Color("#00EAD3")).
				Bold(true)
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(1, 3).
			Margin(1, 0)
	faintStyle = lipgloss.NewStyle().Faint(true)
)

type screen string

const (
	screenMenu     screen = "menu"
	screenTasks    screen = "tasks"
	screenDelete   screen = "delete"
	screenComplete screen = "complete"
	screenAddTask  screen = "addtask"
)

type model struct {
	db        *sqlx.DB
	form      *huh.Form
	cursor    int
	choices   []string
	selected  string
	screen    screen
	tasks     []models.Task
	task      models.Task
	TaskID    int64
	textInput textinput.Model
}

func initialModel(db *sqlx.DB) model {
	ti := textinput.New()
	ti.Placeholder = "Enter Task ID"
	ti.CharLimit = 10

	return model{
		db:     db,
		screen: screenMenu,
		choices: []string{
			"AI Task Manager",
			"Add Task",
			"List Tasks",
			"Complete Task",
			"Delete Task",
		},
		textInput: ti,
	}
}

func (m model) Init() tea.Cmd {
	if m.screen == screenAddTask && m.form != nil {
		return m.form.Init()
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	// Handle text input updates when on delete screen
	if m.screen == screenDelete {
		slog.Debug("detected delete screen and runs textInput update")
		m.textInput, cmd = m.textInput.Update(msg)
	}
	// Handle text input updates when on delete screen
	if m.screen == screenComplete {
		slog.Debug("detected complete screen and runs textInput update")
		m.textInput, cmd = m.textInput.Update(msg)
	}

	if m.screen == screenAddTask && m.form != nil {
		var formCmd tea.Cmd
		updatedForm, formCmd := m.form.Update(msg)
		if f, ok := updatedForm.(*huh.Form); ok {
			m.form = f
		} else {
			if f2, ok := updatedForm.(huh.Model); ok {
				if formPtr, ok := any(f2).(*huh.Form); ok {
					m.form = formPtr
				}
			}
		}

		if m.form.State == huh.StateCompleted {
			// save the task
			if _, err := models.DBAddTask(m.db, m.task); err != nil {
				m.selected = fmt.Sprintf("Error saving task: %v", err)
			} else {
				m.selected = "Task added successfully!"
			}
			// clean up ango back to main menu
			m.form = nil
			m.screen = screenMenu
			return m, nil
		}
		// If user aborted (esc / ctrl+c inside form)
		if m.form.State == huh.StateAborted {
			m.selected = "Task addition cancelled"
			m.form = nil
			m.screen = screenMenu
			return m, nil
		}
		return m, formCmd
	}

	// main menu
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.screen == screenMenu && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.screen == screenMenu && m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "b", "esc":
			if m.screen != screenMenu {
				m.screen = screenMenu
				m.textInput.Blur()
				return m, nil
			}
		case "enter":
			// call the appropriate function based on initialModel
			switch m.screen {
			case screenMenu:
				switch m.cursor {
				case 0: // AI task manager
					// m.selected = aiTaskManager()
					m.screen = screenMenu
				case 1: // Add task
					m.screen = screenAddTask
					cmd := m.initaddTaskForm()
					return m, cmd
				case 2: // List tasks
					tasks, err := models.DBGetTasks(m.db)
					if err != nil {
						slog.Debug("failed to fetch tasks")
						m.selected = "Error fetching tasks"
					} else {
						m.tasks = tasks
						m.selected = ""
					}
					m.screen = screenTasks
				case 3: // Complete Task
					m.screen = screenComplete
					m.textInput.Focus()
					m.textInput.SetValue("")
					return m, textinput.Blink
				case 4: // Delete task
					m.screen = screenDelete
					m.textInput.Focus()
					m.textInput.SetValue("")
					return m, textinput.Blink
				}
			case screenDelete:
				slog.Debug("entering delete case")
				taskIDStr := strings.TrimSpace(m.textInput.Value())
				if taskIDStr == "" {
					m.selected = "Error: Task ID cannot be empty"
					slog.Debug("error: taskID cannot be empty")
				} else {
					taskIDint, err := strconv.ParseInt(taskIDStr, 10, 64)
					if err != nil {
						m.selected = "Error: Task ID cannot be empty"
					} else {
						slog.Debug("running delete task")
						err := models.DBDeleteTask(m.db, taskIDint) // assuming it accepts string or int
						if err != nil {
							slog.Debug("Error deleting task")
							m.selected = fmt.Sprintf("Error deleting task %s: %v", taskIDStr, err)
						} else {
							m.selected = fmt.Sprintf("✅ Deleted task %s", taskIDStr)
						}
					}
				}
				m.screen = screenMenu
				m.textInput.Blur()
				m.textInput.Reset()
				return m, nil
			case screenComplete:
				slog.Debug("entering complete case")
				taskIDStr := strings.TrimSpace(m.textInput.Value())
				if taskIDStr == "" {
					m.selected = "Error: Task ID cannot be empty"
					slog.Debug("error: taskID cannot be empty")
				} else {
					taskIDint, err := strconv.ParseInt(taskIDStr, 10, 64)
					if err != nil {
						m.selected = "Error: Task ID cannot be empty"
					} else {
						slog.Debug("running complete task")
						err := models.DBCompleteTask(m.db, taskIDint) // assuming it accepts string or int
						if err != nil {
							slog.Debug("Error completing task")
							m.selected = fmt.Sprintf("Error marking task complete %s: %v", taskIDStr, err)
						} else {
							m.selected = fmt.Sprintf("✅ marked task complete %s", taskIDStr)
						}
					}
				}
				m.screen = screenMenu
				m.textInput.Blur()
				m.textInput.Reset()
				return m, nil
			}
		}
	}
	return m, cmd
}

func (m model) View() tea.View {
	var s strings.Builder
	switch m.screen {
	case screenTasks:
		s.WriteString(RenderTasks(m.tasks))
		s.WriteString("\n\n")
		s.WriteString(faintStyle.Render("Press 'b' or 'esc' to go back to menu"))
	case screenAddTask:
		if m.form != nil {
			s.WriteString(titleStyle.Render("ADD NEW TASK"))
			s.WriteString("\n\n")
			s.WriteString(m.form.View()) // ← render the huh form
		} else {
			s.WriteString("Loading form...")
		}
	case screenComplete:
		s.WriteString(titleStyle.Render("COMPLETE TASK"))
		s.WriteString("\n\n")
		s.WriteString("Enter Task ID to mark complete:\n")
		s.WriteString("\n\n")
		s.WriteString(m.textInput.View())
		s.WriteString("\n\n")
		s.WriteString(lipgloss.NewStyle().Faint(true).Render("enter: confirm esc: cancel"))
	case screenDelete:
		s.WriteString(titleStyle.Render("DELETE TASK"))
		s.WriteString("\n\n")
		s.WriteString("Enter Task ID to delete:\n")
		s.WriteString("\n\n")
		s.WriteString(m.textInput.View())
		s.WriteString("\n\n")
		s.WriteString(lipgloss.NewStyle().Faint(true).Render("enter: confirm esc: cancel"))
	default: // menu
		s.WriteString(titleStyle.Render("TASK MANAGER"))
		s.WriteString("\n")
		for i, choice := range m.choices {
			// numbering and styling logic
			label := fmt.Sprintf("%d. %s", i+1, choice)
			if m.cursor == i {
				s.WriteString(selectedItemStyle.Render("> " + label))
			} else {
				s.WriteString(itemStyle.Render(label))
			}
			s.WriteString("\n")
		}
		s.WriteString("\n")
		s.WriteString("\n" + lipgloss.NewStyle().Faint(true).Render("j/k: move • enter: select • q: quit"))
	}
	// apply a global border to the entire view
	return tea.NewView(borderStyle.Render(s.String()))
}

func RenderTasks(tasks []models.Task) string {
	var s strings.Builder
	s.WriteString(titleStyle.Render("TASKS"))
	s.WriteString("\n\n")

	if len(tasks) == 0 {
		s.WriteString(itemStyle.Render("No tasks found."))
		s.WriteString("\n")
		return s.String()
	}
	for _, task := range tasks {
		var dateStr string
		if task.FinalDueDate != nil {
			dateStr = task.FinalDueDate.Format("2006-01-02")
		} else {
			dateStr = ""
		}
		var TaskIDStr string
		if task.TaskID != nil {
			TaskIDStr = fmt.Sprintf("%d", *task.TaskID)
		} else {
			TaskIDStr = ""
		}
		row := fmt.Sprintf("%s  %s  %s  %s", TaskIDStr, task.Status, task.Description, dateStr)
		s.WriteString(selectedItemStyle.Render(row))
		s.WriteString("\n")
	}
	return s.String()
}

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
// 	EstimatedHours *float64   `db:"estimated_hours"`
// 	Progress       *int64     `db:"progress"`
// 	ParentTaskID   *int64     `db:"parent_task_id"`
// }

func (m *model) initaddTaskForm() tea.Cmd {
	m.task = models.Task{} // clear m.task

	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Description").
				Prompt("Describe Task").
				Value(&m.task.Description).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("description cannot be empty")
					}
					return nil
				}),
			// add new fields here
			huh.NewConfirm().
				Title("Create this task?").
				Affirmative("Yes, save it!").
				Negative("Cancel"),
		),
	)
	return m.form.Init()
}

func main() {
	// Open (or create) a log file
	// tail -f debug.log to see the log in real time in another terminal
	logFile, err := os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		log.Fatal("Failed to open log file:", err)
	}
	defer logFile.Close()

	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}))
	slog.SetDefault(logger)
	slog.Info("Task manager started", "version", "1.0.0")
	slog.Debug("Debug info", "cursor", 3, "screen", "tasks")

	db := models.StartDatabase()
	defer db.Close() // close the database when main() finishes
	if _, err := tea.NewProgram(initialModel(db)).Run(); err != nil {
		log.Fatal(err)
	}
}
