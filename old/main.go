//go:build !tview
// +build !tview

package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/jmoiron/sqlx"
	"github.com/lmittmann/tint"
	"github.com/lufraser/gotaskmanager/aitaskmanager"
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
	screenMenu          screen = "menu"
	screenTasks         screen = "tasks"
	screenDelete        screen = "delete"
	screenComplete      screen = "complete"
	screenAddTask       screen = "addtask"
	screenAITaskManager screen = "AITaskManager"
)

type model struct {
	db       *sqlx.DB      // used for all database operations
	form     *huh.Form     // used in the add task form flow
	cursor   int           // used for menu navigation
	choices  []string      // rendering the menu choices
	selected string        // not used
	screen   screen        // core screen state machine
	tasks    []models.Task // list of tasks for the task form
	task     *models.Task  // reference to an individual task to be added to the database
	// temporary strings used only while the huh form is active (date/time/number fields)
	doDateStr, finalDueDateStr, completedAtStr        string
	startTimeStr, endTimeStr                          string
	estimatedStr, progressStr, assigneeStr, parentStr string
	// TaskID     int64										//not used
	textInput  textinput.Model        // used for the complete and delete task screens to hold the taskID
	aiSession  *aitaskmanager.Session // holds the aitaskmanager session.
	aiMessages []string               // holds the AI chat history
	aiInput    textinput.Model        // holds the input field to submit to the AI
	// err        error										//not used
	viewport viewport.Model // viewport to hold the view tasks to enable scrolling
	ready    bool           // used in the window size handler
}

func initialModel(db *sqlx.DB) model {
	ti := textinput.New()
	ti.Placeholder = "Enter Task ID"
	ti.CharLimit = 10

	aiTi := textinput.New()
	aiTi.Placeholder = "Enter AI request"
	aiTi.CharLimit = 500

	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))

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
		textInput:  ti,
		aiInput:    aiTi,
		aiMessages: []string{"AI Task Manager ready"},
		aiSession:  aitaskmanager.NewSession(db),
		viewport:   vp,
	}
}

func (m model) Init() tea.Cmd {
	if m.screen == screenAddTask && m.form != nil {
		return m.form.Init()
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	slog.Debug("Update received message", "type", fmt.Sprintf("%T", msg), "msg", msg)
	var cmd tea.Cmd

	// Global keys first (ESC, quit) — works on every screen
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			if m.screen != screenMenu {
				m.screen = screenMenu
				m.tasks = nil
				m.viewport.SetContent("")
				m.textInput.Blur()
				m.aiInput.Blur()
				m.aiMessages = m.aiMessages[:0]
				return m, nil
			}
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	// Handle text input updates when on delete screen
	if m.screen == screenDelete {
		slog.Debug("detected delete screen and runs textInput update")
		m.textInput, cmd = m.textInput.Update(msg)
	}
	// Handle text input updates when on complete screen
	if m.screen == screenComplete {
		slog.Debug("detected complete screen and runs textInput update")
		m.textInput, cmd = m.textInput.Update(msg)
	}

	// Handle AI Task Manager input
	if m.screen == screenAITaskManager {
		var aiCmd tea.Cmd
		m.aiInput, aiCmd = m.aiInput.Update(msg) // ensures typing works
		cmd = aiCmd
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			switch keyMsg.String() {
			case "enter":
				input := strings.TrimSpace(m.aiInput.Value())
				if input != "" && m.aiSession != nil {
					m.aiMessages = append(m.aiMessages, "You: "+input)
					response, err := m.aiSession.Execute(input)
					if err != nil {
						m.aiMessages = append(m.aiMessages, "Error: "+err.Error())
					} else {
						m.aiMessages = append(m.aiMessages, "Grok: "+response)
					}
					m.aiInput.SetValue("")
					// NEW: History limit (prevents unbounded growth)
					if len(m.aiMessages) > 40 {
						m.aiMessages = m.aiMessages[len(m.aiMessages)-40:]
					}
				}
				// return m, nil
			case "ctrl+c", "q":
				return m, tea.Quit
			}
		}
		return m, cmd
	}

	if m.screen == screenAddTask && m.form != nil {
		updatedForm, formCmd := m.form.Update(msg)
		if f, ok := updatedForm.(*huh.Form); ok {
			m.form = f
		}

		// Handle completion FIRST (before returning any formCmd)
		if m.form.State == huh.StateCompleted {
			slog.Debug("Form completed - saving task", "task", fmt.Sprintf("%+v", m.task))

			// parse the temporary string fields back into the real task pointers
			if t, err := time.Parse("2006-01-02", m.doDateStr); err == nil {
				m.task.DoDate = &t
			}
			if t, err := time.Parse("2006-01-02", m.finalDueDateStr); err == nil {
				m.task.FinalDueDate = &t
			}
			if m.completedAtStr != "" {
				if t, err := time.Parse("2006-01-02", m.completedAtStr); err == nil {
					m.task.CompletedAt = &t
				}
			}
			if t, err := time.Parse("15:04", m.startTimeStr); err == nil {
				m.task.StartTime = &t
			}
			if t, err := time.Parse("15:04", m.endTimeStr); err == nil {
				m.task.EndTime = &t
			}
			if f, err := strconv.ParseFloat(m.estimatedStr, 64); err == nil {
				m.task.EstimatedHours = &f
			}
			if i, err := strconv.ParseInt(m.progressStr, 10, 64); err == nil {
				m.task.Progress = &i
			}
			if i, err := strconv.ParseInt(m.assigneeStr, 10, 64); err == nil {
				m.task.AssigneeID = &i
			}
			if i, err := strconv.ParseInt(m.parentStr, 10, 64); err == nil {
				m.task.ParentTaskID = &i
			}

			id := models.DBAddTask(m.db, *m.task)
			if id == 0 {
				m.selected = "Error saving task"
				slog.Error("Failed to save task")
			} else {
				m.selected = fmt.Sprintf("✅ Task added successfully! ID: %d", id)
				slog.Info("Task added", "id", id)
			}

			// Clean up
			m.task = nil
			m.form = nil
			m.doDateStr = ""
			m.finalDueDateStr = ""
			m.completedAtStr = ""
			m.startTimeStr = ""
			m.endTimeStr = ""
			m.estimatedStr = ""
			m.progressStr = ""
			m.assigneeStr = ""
			m.parentStr = ""
			m.screen = screenMenu
			return m, nil
		}

		if m.form.State == huh.StateAborted {
			m.selected = "Task addition cancelled"
			m.task = nil
			m.form = nil
			m.doDateStr = ""
			m.finalDueDateStr = ""
			m.completedAtStr = ""
			m.startTimeStr = ""
			m.endTimeStr = ""
			m.estimatedStr = ""
			m.progressStr = ""
			m.assigneeStr = ""
			m.parentStr = ""
			m.screen = screenMenu
			return m, nil
		}
		// Only return the form command if we're still active
		return m, formCmd
	}

	// if m.screen == screenTasks {
	// 	slog.Debug("task screen is active")
	// }

	// main menu
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.SetWidth(msg.Width)
		m.viewport.SetHeight(msg.Height - 10)
		m.aiInput.SetWidth(max(40, msg.Width-20))
		m.textInput.SetWidth(20)
		if m.screen == screenTasks {
			m.viewport.SetContent(RenderTasks(m.tasks))
		}

	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.screen == screenMenu && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.screen == screenMenu && m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			// call the appropriate function based on initialModel
			switch m.screen {
			case screenMenu:
				switch m.cursor {
				case 0: // AI task manager
					if m.aiSession == nil {
						m.aiSession = aitaskmanager.NewSession(m.db)
						if m.aiSession == nil {
							m.aiMessages = append(m.aiMessages, "Error: failed to initialize AI session")
							m.screen = screenMenu
							return m, nil
						}
					}
					m.screen = screenAITaskManager
					m.aiInput.Focus()
					m.aiInput.SetValue("")
					return m, nil
				case 1: // Add task
					slog.Debug("Enter init add task")
					m.screen = screenAddTask

					m.task = &models.Task{
						Description:    "",
						Status:         "Pending",
						CreatedAt:      ptr(time.Now()),
						UpdatedAt:      ptr(time.Time{}),
						Priority:       "Regular",
						AssigneeID:     nil,
						DoDate:         ptr(time.Now().AddDate(0, 0, 7)),
						FinalDueDate:   ptr(time.Now().AddDate(0, 0, 14)),
						StartTime:      ptr(time.Time{}),
						EndTime:        ptr(time.Time{}),
						CompletedAt:    ptr(time.Time{}),
						EstimatedHours: ptr[float64](4),
						Progress:       ptr[int64](0),
						ParentTaskID:   nil,
					}

					m.doDateStr = datePtrToString(m.task.DoDate)
					m.finalDueDateStr = datePtrToString(m.task.FinalDueDate)
					m.completedAtStr = datePtrToString(m.task.CompletedAt)
					m.startTimeStr = timePtrToString(m.task.StartTime)
					m.endTimeStr = timePtrToString(m.task.EndTime)
					m.estimatedStr = floatPtrToString(m.task.EstimatedHours)
					m.progressStr = int64PtrToString(m.task.Progress)
					m.assigneeStr = int64PtrToString(m.task.AssigneeID)
					m.parentStr = int64PtrToString(m.task.ParentTaskID)

					m.form = huh.NewForm(
						huh.NewGroup(
							huh.NewInput().
								Title("Description").
								Value(&m.task.Description).
								Validate(func(s string) error {
									if strings.TrimSpace(s) == "" {
										return fmt.Errorf("description cannot be empty")
									}
									return nil
								}),
							huh.NewSelect[string]().
								Title("Status").
								Options(
									huh.NewOption("Pending", "Pending"),
									huh.NewOption("In Progress", "In Progress"),
									huh.NewOption("Completed", "Completed"),
									huh.NewOption("Blocked", "Blocked"),
								).
								Value(&m.task.Status),
							huh.NewSelect[string]().
								Title("Priority").
								Options(
									huh.NewOption("Low", "Low"),
									huh.NewOption("Regular", "Regular"),
									huh.NewOption("High", "High"),
									huh.NewOption("Urgent", "Urgent"),
								).
								Value(&m.task.Priority),
							huh.NewInput().Title("Do Date (YYYY-MM-DD)").Placeholder("2026-04-26").Value(&m.doDateStr).Validate(validateDate),
							huh.NewInput().Title("Final Due Date (YYYY-MM-DD)").Placeholder("2026-05-03").Value(&m.finalDueDateStr).Validate(validateDate),
							huh.NewInput().Title("Completed At (YYYY-MM-DD)").Placeholder("leave empty if not completed").Value(&m.completedAtStr).Validate(validateDateOptional),
							huh.NewInput().Title("Start Time (HH:MM)").Placeholder("09:00").Value(&m.startTimeStr).Validate(validateTimeOptional),
							huh.NewInput().Title("End Time (HH:MM)").Placeholder("17:00").Value(&m.endTimeStr).Validate(validateTimeOptional),
							huh.NewInput().Title("Estimated Hours").Placeholder("1.0").Value(&m.estimatedStr).Validate(validateFloat),
							huh.NewInput().Title("Progress (%)").Placeholder("0").Value(&m.progressStr).Validate(validateProgress),
							huh.NewInput().Title("Assignee ID (optional)").Placeholder("ID").Value(&m.assigneeStr),
							huh.NewInput().Title("Parent Task ID (optional)").Placeholder("leave empty for top-level task").Value(&m.parentStr),
						),
					)
					slog.Debug("Exit init add task")
					return m, m.form.Init()
				case 2: // List tasks
					tasks, err := models.DBGetTasks(m.db)
					if err != nil {
						slog.Debug("failed to fetch tasks", "error", err)
						m.selected = "Error fetching tasks"
					} else {
						m.tasks = tasks
						m.selected = ""
					}
					m.aiInput.Blur()
					m.textInput.Blur()
					m.viewport.SetContent(RenderTasks(m.tasks))
					m.viewport.GotoTop()
					m.screen = screenTasks
					return m, nil
				case 3: // Complete Task
					m.screen = screenComplete
					m.textInput.Focus()
					m.textInput.SetValue("")
					return m, nil

				case 4: // Delete task
					m.screen = screenDelete
					m.textInput.Focus()
					m.textInput.SetValue("")
					return m, nil
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
				m.textInput.Reset()
				return m, nil
			}
		}
	}
	if m.screen == screenTasks {
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) View() tea.View {
	var s strings.Builder
	switch m.screen {
	case screenAITaskManager:
		s.WriteString(titleStyle.Render("AI TASK MANAGER"))
		s.WriteString("\n")
		for _, msg := range m.aiMessages {
			s.WriteString(itemStyle.Render(msg) + "\n")
		}
		s.WriteString("\n" + m.aiInput.View() + "\n")
		s.WriteString("\n")
		s.WriteString(lipgloss.NewStyle().Faint(true).Render("enter: send • esc: back to menu"))
	case screenTasks:
		s.WriteString(titleStyle.Render("Your Tasks"))
		s.WriteString("\n")
		s.WriteString(m.viewport.View())
		s.WriteString("\n\n")
		s.WriteString(faintStyle.Render("Press 'esc' to go back to menu • Use arrows to scroll"))
	case screenAddTask:
		slog.Debug("enter View.screenAddTask")
		if m.form != nil {
			slog.Debug("task", "task", m.task)
			s.WriteString(titleStyle.Render("ADD NEW TASK"))
			s.WriteString("\n\n")
			s.WriteString(m.form.View()) // ← render the huh form
		} else {
			s.WriteString("Loading form...")
		}
		slog.Debug("Exit View.screenAddTask")
	case screenComplete:
		s.WriteString(titleStyle.Render("COMPLETE TASK"))
		s.WriteString("\n\n")
		s.WriteString("Enter Task ID to mark complete:\n")
		s.WriteString("\n\n")
		s.WriteString(m.textInput.View())
		s.WriteString("\n\n")
		s.WriteString(lipgloss.NewStyle().Faint(true).Render("enter: confirm esc: cancel"))
		s.WriteString("\n\n")
		if m.selected != "" {
			s.WriteString("\n" + m.selected)
		}
	case screenDelete:
		s.WriteString(titleStyle.Render("DELETE TASK"))
		s.WriteString("\n\n")
		s.WriteString("Enter Task ID to delete:\n")
		s.WriteString("\n\n")
		s.WriteString(m.textInput.View())
		s.WriteString("\n\n")
		s.WriteString(lipgloss.NewStyle().Faint(true).Render("enter: confirm esc: cancel"))
		s.WriteString("\n\n")
		if m.selected != "" {
			s.WriteString("\n" + m.selected)
		}
	default: // menu
		s.WriteString(titleStyle.Render("TASK MANAGER"))
		s.Wr