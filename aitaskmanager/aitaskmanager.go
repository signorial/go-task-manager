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

// func AITaskManager() {
func Run(db *sqlx.DB) {
	ctx := context.Background()
	// Initialize Genkit with xAI
	g := genkit.Init(
		ctx,
		genkit.WithPlugins(&compat_oai.OpenAICompatible{
			Provider: "xai",
			APIKey:   os.Getenv("XAI_API_KEY"),
			BaseURL:  "https://api.x.ai/v1",
		}),
		genkit.WithDefaultModel("xai/grok-3"),
		genkit.WithPromptDir("../prompts/"),
		// genkit.WithPromptDir("./prompts"),
	)

	genkit.DefineSchemaFor[models.Task](g) // add schema to the AITaskManager
	model := "xai/grok-3"
	history := []*ai.Message{}
	scanner := bufio.NewScanner(os.Stdin)
	// setup grokpromt
	grokPrompt := genkit.LookupPrompt(g, "grok_chat")
	if grokPrompt == nil {
		slog.Debug("ERROR: could not find prompt file grok_chat.prompt")
		return
	}
	type NoInput struct{}
	AIListTasks := genkit.DefineTool(
		g,
		"AIListTasks",
		`this returns a slice of models.Task from the tasks databases. 
		which contains all the fields related to a task
		- CreatedAt: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")      
		- UpdatedAt: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")      
		- DoDate: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")         
		- FinalDueDate: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")   
		- StartTime: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")      
		- EndTime: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")        
		- CompletedAt: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")`,
		func(ctx *ai.ToolContext, _ NoInput) ([]models.Task, error) {
			tasks, err := models.DBGetTasks(db)
			if err != nil {
				slog.Debug("failed to fetch tasks %v", err)
				return nil, err
			}
			return tasks, err
		})
	AIDeleteTask := genkit.DefineTool(
		g,
		"AIDeleteTask",
		`Deletes a specified task from the tasks database by its ID.
		- CreatedAt: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")      
		- UpdatedAt: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")      
		- DoDate: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")         
		- FinalDueDate: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")   
		- StartTime: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")      
		- EndTime: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")        
		- CompletedAt: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")`,

		func(ctx *ai.ToolContext, input struct {
			TaskID int64 `jsonschema_description:"The unique ID of the task to delete"`
		},
		) (string, error) {
			err := models.DBDeleteTask(db, input.TaskID)
			if err != nil {
				slog.Error("failed to delete task", "taskID", input.TaskID, "error", err)
				return "", fmt.Errorf("failed to delete task: %w", err)
			}
			return fmt.Sprintf("Task %d has been successfully deleted.", input.TaskID), nil
		},
	)
	AIGetTask := genkit.DefineTool(
		g,
		"AIGetTask",
		`This returns a specified task from the tasks database by its ID.
		- CreatedAt: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")      
		- UpdatedAt: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")      
		- DoDate: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")         
		- FinalDueDate: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")   
		- StartTime: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")      
		- EndTime: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")        
		- CompletedAt: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")`,
		func(ctx *ai.ToolContext, input struct {
			TaskID int64 `jsonschema_description:"The unique ID of the task to get"`
		},
		) (models.Task, error) {
			task, err := models.DBGetTask(db, input.TaskID)
			if err != nil {
				slog.Error("failed to get task", "taskID", input.TaskID, "error", err)
				return task, fmt.Errorf("failed to get task: %w", err)
			}
			return task, err
		},
	)

	AICompleteTask := genkit.DefineTool(
		g,
		"AICompleteTask",
		`This marks a task completed based on its its ID.
		- CreatedAt: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")      
		- UpdatedAt: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")      
		- DoDate: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")         
		- FinalDueDate: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")   
		- StartTime: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")      
		- EndTime: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")        
		- CompletedAt: optional RFC3339 string (e.g. "2026-05-01T12:00:00Z")`,
		func(ctx *ai.ToolContext, input struct {
			TaskID int64 `jsonschema_description:"The unique ID of the task to get"`
		},
		) (string, error) {
			err := models.DBCompleteTask(db, input.TaskID)
			if err != nil {
				slog.Error("failed to mark task completed", "error", err)
				return "", fmt.Errorf("failed to complete task: %w", err)
			}
			return "", err
		},
	)
	AIAddTask := genkit.DefineTool(
		g,
		"AIAddTask",
		`Creates a new task. All time fields must be RFC3339 strings (e.g. "2026-04-15T14:30:00Z") or null. Provide description, status, priority, final_due_date and estimated_hours at minimum.`,
		func(ctx *ai.ToolContext, input struct {
			Description    string   `jsonschema_description:"Detailed description of what needs to be done"`
			Status         string   `jsonschema_description:"Current status (Pending, In Progress, COMPLETED)"`
			Priority       string   `jsonschema_description:"Priority level: Low, Regular, or High"`
			AssigneeID     *int64   `jsonschema_description:"ID of the person assigned to this task"`
			DoDate         *string  `jsonschema_description:"Preferred date to work on this task as RFC3339 (e.g. 2026-04-15T14:30:00Z)"`
			FinalDueDate   *string  `jsonschema_description:"Final deadline as RFC3339 string (e.g. 2026-04-15T14:30:00Z)"`
			StartTime      *string  `jsonschema_description:"When work actually began as RFC3339"`
			EndTime        *string  `jsonschema_description:"When work was completed as RFC3339"`
			CompletedAt    *string  `jsonschema_description:"Timestamp when marked complete as RFC3339"`
			EstimatedHours *float64 `jsonschema_description:"Estimated hours required to complete the task"`
			Progress       *int64   `jsonschema_description:"Progress percentage (0-100)"`
			ParentTaskID   *int64   `jsonschema_description:"ID of parent task if this is a subtask"`
		},
		) (int64, error) {
			var task models.Task
			task.Description = input.Description
			task.Status = input.Status
			task.Priority = input.Priority
			task.AssigneeID = input.AssigneeID
			task.EstimatedHours = input.EstimatedHours
			task.Progress = input.Progress
			task.ParentTaskID = input.ParentTaskID
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
			if input.StartTime != nil {
				if t, err := time.Parse(time.RFC3339, *input.StartTime); err == nil {
					task.StartTime = &t
				}
			}
			if input.EndTime != nil {
				if t, err := time.Parse(time.RFC3339, *input.EndTime); err == nil {
					task.EndTime = &t
				}
			}
			if input.CompletedAt != nil {
				if t, err := time.Parse(time.RFC3339, *input.CompletedAt); err == nil {
					task.CompletedAt = &t
				}
			}
			id := models.DBAddTask(db, task)
			if id == 0 {
				return 0, fmt.Errorf("failed to insert task into database")
			}
			return id, nil
		},
	)
	calleableFunctions := []ai.ToolRef{
		AICompleteTask,
		AIGetTask,
		AIDeleteTask,
		AIListTasks,
		AIAddTask,
	}
	fmt.Println("Chatting with %s via DotPrompt! Type 'exit' to quit.", model)

	for {
		fmt.Print("User: ")
		if !scanner.Scan() {
			break
		}
		input := scanner.Text()
		if strings.ToLower(input) == "exit" {
			break
		}

		resp, err := grokPrompt.Execute(
			ctx,
			ai.WithInput(map[string]any{"user_input": input}),
			ai.WithMessages(history...),
			ai.WithTools(calleableFunctions...),
		)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		responseText := resp.Text()
		fmt.Printf("Grok: %s\n", responseText)

		history = append(history, ai.NewUserTextMessage(input))
		history = append(history, ai.NewModelTextMessage(responseText))
	}
}
