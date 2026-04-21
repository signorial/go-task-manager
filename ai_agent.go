package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai"
	"github.com/lufraser/gotaskmanager/models"
)

func AIQuery() {
	db := models.StartDatabase()
	defer db.Close() // close the database when main() finishes

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

	// Define an empty input type (this is the standard trick)
	type NoInput struct{}
	genkit.DefineTool(
		g,
		"AIListTasks",
		"this returns a slice of models.Task from the tasks databases.  which contains all the fields related to a task",
		func(ctx *ai.ToolContext, _ NoInput) ([]models.Task, error) {
			tasks, err := models.DBGetTasks(db)
			if err != nil {
				slog.Debug("failed to fetch tasks %v", err)
				return nil, err
			}
			return tasks, err
		})

	genkit.DefineTool(
		g,
		"AIDeleteTask",
		"Deletes a specified task from the tasks database by its ID.",
		func(ctx *ai.ToolContext, input struct {
			TaskID int64 `jsonschema_description:"The unique ID of the task to delete"`
		},
		) (string, error) { // ← Return (output, error)

			err := models.DBDeleteTask(db, input.TaskID)
			if err != nil {
				slog.Error("failed to delete task", "taskID", input.TaskID, "error", err)
				return "", fmt.Errorf("failed to delete task: %w", err) // return error to Genkit
			}

			return fmt.Sprintf("Task %d has been successfully deleted.", input.TaskID), nil
		},
	)

	genkit.DefineTool(
		g,
		"AIGetTask",
		"This returns a specified task from the tasks database by its ID.",
		func(ctx *ai.ToolContext, input struct {
			TaskID int64 `jsonschema_description:"The unique ID of the task to get"`
		},
		) (models.Task, error) { // ← Return (output, error)

			task, err := models.DBGetTask(db, input.TaskID)
			if err != nil {
				slog.Error("failed to get task", "taskID", input.TaskID, "error", err)
				return task, fmt.Errorf("failed to get task: %w", err) // return error to Genkit
			}

			return task, err
		},
	)

	genkit.DefineTool(
		g,
		"AICompleteTask",
		"This marks a task completed based on its its ID.",
		func(ctx *ai.ToolContext, input struct {
			TaskID int64 `jsonschema_description:"The unique ID of the task to get"`
		},
		) (string, error) { // ← Return (output, error)

			err := models.DBCompleteTask(db, input.TaskID)
			if err != nil {
				slog.Error("failed to mark task completed", "error", err)
				return "", fmt.Errorf("failed to get task: %w", err) // return error to Genkit
			}

			return "", err
		},
	)
}
