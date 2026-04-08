package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/lufraser/gotaskmanager/models"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jmoiron/sqlx"
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
	chevronStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00EAD3")).
			Bold(true)
)

func main() {
	db := models.StartDatabase()
	defer db.Close() // close the database when main() finishes
	if _, err := tea.NewProgram(initialModel(db)).Run(); err != nil {
		log.Fatal(err)
	}
}

type model struct {
	db       *sqlx.DB
	cursor   int
	choices  []string
	selected string
	screen   string
	tasks    []models.Task
}

func initialModel(db *sqlx.DB) model {
	return model{
		db:     db,
		screen: "menu",
		choices: []string{
			"AI Task Manager",
			"Add Task",
			"List Tasks",
			"Complete Task",
			"Delete Task",
		},
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "b", "esc":
			if m.screen == "tasks" {
				m.screen = "menu"
				return m, nil
			}
		case "enter":
			// call the appropriate function based on initialModel
			switch m.cursor {
			case 0:
				m.selected = aiTaskManager()
			case 1:
				m.selected = addTask()
			case 2:
				tasks, err := models.DBGetTasks(m.db)
				if err != nil {
					m.selected = "Error fetching records: " + err.Error()
					m.screen = "menu"
					return m, nil
				}
				m.tasks = tasks
				m.screen = "tasks"
				m.selected = ""
				return m, nil
			case 3:
				m.selected = completeTask()
			case 4:
				err := models.DBDeleteTask(m.db, 123)
				if err != nil {
					m.selected = fmt.Sprintf("couldn't find task")
				} else {
					m.selected = fmt.Sprintf("Deleted Task: %s", "123")
				}
			}
		}
	}
	return m, nil
}

func (m model) View() tea.View {
	var s strings.Builder
	switch m.screen {
	case "tasks":
		content := RenderTasks(m.tasks)
		s.WriteString(content)
		s.WriteString("\n\n")
		s.WriteString(lipgloss.NewStyle().Faint(true).Render("Press 'b' or 'esc' to go back to menu"))
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

// Better version - returns the rendered content as string
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
		dateStr := task.FinalDueDate.Format("2006-01-02")
		row := fmt.Sprintf("%s  %s  %s", task.TaskID, task.Description, dateStr)
		s.WriteString(selectedItemStyle.Render(row))
		s.WriteString("\n")
	}

	return s.String()
}

// Example functions for each selection
func aiTaskManager() string {
	return "✅ All systems operational."
}

func addTask() string {
	return "📜 Fetching latest logs..."
}

func completeTask() string {
	return "👥 Opening user directory..."
}

func exitProgram() string {
	return "⚙️ Loading configuration..."
}
