package functions

import (
	"fmt"
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

func PrintTasks(db *sqlx.DB) tea.View {
	var s strings.Builder
	var tasks []models.Task
	var err error
	s.WriteString(titleStyle.Render("TASKS"))
	s.WriteString("\n")
	tasks, err = models.DBGetTasks(db)
	for index, tasks := range tasks {
		// numbering and styling logic
		label := fmt.Sprintf("%d. %s", i+1, choice)
		if m.cursor == i {
			s.WriteString(selectedItemStyle.Render("> " + label))
		} else {
			s.WriteString(itemStyle.Render(label))
		}
		s.WriteString("\n")
	}
	s.WriteString("\n" + lipgloss.NewStyle().Faint(true).Render("j/k: move • enter: select • q: quit"))
	// apply a global border to the entire view
	return tea.NewView(borderStyle.Render(s.String()))
}
