package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	_ "modernc.org/sqlite" // import driver for database/sql to use
)

func PrintTasks() tea.View {
	var s strings.Builder
	var tasks []Task
	s.WriteString(titleStyle.Render("TASKS"))
	s.WriteString("\n")
	tasks = DBGetTasks()
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
