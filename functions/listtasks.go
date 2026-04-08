package functions

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jmoiron/sqlx"
	"github.com/lufraser/gotaskmanager/models"
	_ "modernc.org/sqlite" // import driver for database/sql to use
)

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
)

func PrintTasks(db *sqlx.DB) []models.Task {
	var s strings.Builder
	var tasks []models.Task
	s.WriteString(titleStyle.Render("TASKS"))
	s.WriteString("\n")
	tasks, _ = models.DBGetTasks(db)
	for _, task := range tasks {
		dateStr := task.FinalDueDate.Format("2006-01-02")
		row := task.TaskID + " " + task.Description + " " + dateStr
		s.WriteString(selectedItemStyle.Render(row))
		s.WriteString("\n")
	}
	// apply a global border to the entire view
	tea.NewView(borderStyle.Render(s.String()))
	return tasks
}
