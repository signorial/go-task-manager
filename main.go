package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai"
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
)

func main() {
	db := StartDatabase()
	defer db.Close() // close the database when main() finishes
	if _, err := tea.NewProgram(initialModel(db)).Run(); err != nil {
		log.Fatal(err)
	}
}

func aiConnector() {
	ctx := context.Background()

	// Initialize Genkit with xAI
	g := genkit.Init(ctx,
		genkit.WithPlugins(&compat_oai.OpenAICompatible{
			Provider: "xai",
			APIKey:   os.Getenv("XAI_API_KEY"),
			BaseURL:  "https://api.x.ai/v1",
		}),
		genkit.WithDefaultModel("xai/grok-3"),
	)

	// Define the flow and capture the returned Flow object
	grokHelloFlow := genkit.DefineFlow(g, "grokHello",
		func(ctx context.Context, subject string) (string, error) {
			resp, err := genkit.Generate(ctx, g,
				ai.WithModelName("xai/grok-3"),
				ai.WithPrompt(fmt.Sprintf("Tell me a fun fact about %s.", subject)),
			)
			if err != nil {
				return "", err
			}
			return resp.Text(), nil
		},
	)

	// Run the flow using the .Run() method on the flow object
	result, err := grokHelloFlow.Run(ctx, "Grok and xAI")
	if err != nil {
		log.Fatalf("Error running flow: %v", err)
	}

	fmt.Println("Response from Grok:")
	fmt.Println(result)
}

type model struct {
	db       *sqlx.DB
	cursor   int
	choices  []string
	selected string
}

func initialModel(db *sqlx.DB) model {
	return model{
		db: db,
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
		case "enter":
			// call the appropriate function based on initialModel
			switch m.cursor {
			case 0:
				m.selected = aiTaskManager()
			case 1:
				m.selected = addTask()
			case 2:
				tasks, err := listTasks(m.db)
				if err != nil {
					m.selected = fmt.Sprintf("tasks failed to load")
				} else {
					result := "found tasks:\n"
					for _, t := range tasks {
						result += fmt.Sprintf("- %s\n", t.Description)
					}
					m.selected = result
				}
			case 3:
				m.selected = completeTask()
			case 4:
				err := DeleteTask(m.db, 123)
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

	s.WriteString("\n" + lipgloss.NewStyle().Faint(true).Render("j/k: move • enter: select • q: quit"))

	// apply a global border to the entire view
	return tea.NewView(borderStyle.Render(s.String()))
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
