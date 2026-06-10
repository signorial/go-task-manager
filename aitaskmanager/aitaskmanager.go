package aitaskmanager

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai"
	"github.com/jmoiron/sqlx"
	"github.com/lufraser/gotaskmanager/models"
	_ "modernc.org/sqlite"
)

type Session struct {
	ctx        context.Context
	grokPrompt ai.Prompt
	tools      []ai.ToolRef
	history    []*ai.Message
	db         *sqlx.DB
}

func NewSession(db *sqlx.DB) (*Session, error) {
	ctx := context.Background()

	g := genkit.Init(
		ctx,
		genkit.WithPlugins(&compat_oai.OpenAICompatible{
			Provider: "xai",
			APIKey:   os.Getenv("XAI_API_KEY"),
			BaseURL:  "https://api.x.ai/v1",
		}),
		genkit.WithDefaultModel("xai/grok-3"),
		genkit.WithPromptDir("./prompts/"),
	)

	genkit.DefineSchemaFor[models.Task](g)

	grokPrompt := genkit.LookupPrompt(g, "grok_chat")
	if grokPrompt == nil {
		return nil, fmt.Errorf("could not find prompt file grok_chat.prompt")
	}

	type NoInput struct{}

	AIListTasks := genkit.DefineTool(
		g,
		"AIListTasks",
		`this returns a slice of models.Task from the tasks databases.`,
		func(ctx *ai.ToolContext, _ NoInput) ([]models.Task, error) {
			slog.Debug("ENTER AIListTasks")
			tasks, err := models.DBGetTasks(db)
			if err != nil {
				slog.Debug("failed to fetch tasks %v", err)
				return nil, err
			}
			slog.Debug("EXIT AIListTasks")
			return tasks, nil
		})

	AIDeleteTask := genkit.DefineTool(
		g,
		"AIDeleteTask",
		`Deletes a specified task from the tasks database by its ID.`,
		func(ctx *ai.ToolContext, input struct {
			TaskID int64 `jsonschema_description:"The unique ID of the task to delete"`
		},
		) (bool, error) {
			slog.Debug("ENTER AIDeleteTask")
			err := models.DBDeleteTask(db, input.TaskID)
			if err != nil {
				slog.Debug("failed to delete task %v", err)
				return false, err
			}
			slog.Debug("EXIT AIDeleteTask")
			return true, nil
		})

	AIGetTask := genkit.DefineTool(
		g,
		"AIGetTask",
		`Returns a single task by its ID.`,
		func(ctx *ai.ToolContext, input struct {
			TaskID int64 `jsonschema_description:"The unique ID of the task"`
		},
		) (models.Task, error) {
			slog.Debug("ENTER AIGetTask")
			task, err := models.DBGetTask(db, input.TaskID)
			if err != nil {
				slog.Debug("failed to get task %v", err)
				return models.Task{}, err
			}
			slog.Debug("EXIT AIGetTask")
			return task, nil
		})

	AICompleteTask := genkit.DefineTool(
		g,
		"AICompleteTask",
		`Marks a task as completed.`,
		func(ctx *ai.ToolContext, input struct {
			TaskID int64 `jsonschema_description:"The unique ID of the task to complete"`
		},
		) (bool, error) {
			slog.Debug("ENTER AICompleteTask")
			err := models.DBCompleteTask(db, input.TaskID)
			if err != nil {
				slog.Debug("failed to complete task %v", err)
				return false, err
			}
			slog.Debug("EXIT AICompleteTask")
			return true, nil
		})

	AIAddTask := genkit.DefineTool(
		g,
		"AIAddTask",
		`Adds a new task to the database.`,
		func(ctx *ai.ToolContext, input struct {
			Description    string   `jsonschema_description:"Task description"`
			Status         string   `jsonschema_description:"Task status"`
			Priority       string   `jsonschema_description:"Task priority"`
			DoDate         *string  `jsonschema_description:"Optional do date in RFC3339"`
			FinalDueDate   *string  `jsonschema_description:"Optional final due date in RFC3339"`
			EstimatedHours *float64 `jsonschema_description:"Optional estimated hours"`
		},
		) (int64, error) {
			slog.Debug("ENTER AIAddTask")

			task := models.Task{
				Description: input.Description,
				Status:      input.Status,
				Priority:    input.Priority,
			}

			if input.DoDate != nil {
				if t, err := time.Parse(time.RFC3339, *input.DoDate); err == nil {
					task.DoDate = &t
				}
			}
			if input.FinalDueDate != nil {
				if t, err := time.Parse(time.RFC3339, *input.FinalDueDate); err == nil {
					task.FinalDueDate = &t
				}
			}
			if input.EstimatedHours != nil {
				task.EstimatedHours = input.EstimatedHours
			}

			id, err := models.DBAddTask(db, task)
			if err != nil {
				slog.Debug("failed to insert task %v", err)
				return 0, err
			}
			slog.Debug("EXIT AIAddTask")
			return id, nil
		},
	)

	tools := []ai.ToolRef{
		AICompleteTask,
		AIGetTask,
		AIDeleteTask,
		AIListTasks,
		AIAddTask,
	}

	return &Session{
		ctx:        ctx,
		grokPrompt: grokPrompt,
		tools:      tools,
		history:    []*ai.Message{},
		db:         db,
	}, nil
}

// Start begins the interactive AI chat session.
func (s *Session) Start() error {
	return s.RunChat()
}

// Execute sends a single message to the AI and returns the response.
func (s *Session) Execute(input string) (string, error) {
	resp, err := s.grokPrompt.Execute(
		s.ctx,
		ai.WithInput(map[string]any{"user_input": input}),
		ai.WithMessages(s.history...),
		ai.WithTools(s.tools...),
		ai.WithMaxTurns(10),
	)
	if err != nil {
		return "", err
	}

	text := resp.Text()
	s.history = append(s.history, ai.NewUserTextMessage(input))
	s.history = append(s.history, ai.NewModelTextMessage(text))
	return text, nil
}

// RunChat runs the main interactive chat loop.
func (s *Session) RunChat() error {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Chatting with Grok via DotPrompt! Type 'exit' to quit.")

	for {
		fmt.Print("User: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if strings.ToLower(input) == "exit" {
			break
		}

		resp, err := s.grokPrompt.Execute(
			s.ctx,
			ai.WithInput(map[string]any{"user_input": input}),
			ai.WithMessages(s.history...),
			ai.WithTools(s.tools...),
			ai.WithMaxTurns(10),
		)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		responseText := resp.Text()
		fmt.Printf("Grok: %s\n", responseText)

		s.history = append(s.history, ai.NewUserTextMessage(input))
		s.history = append(s.history, ai.NewModelTextMessage(responseText))
	}

	return nil
}
