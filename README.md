# GoTaskManager

A terminal-based task manager built in Go with a bubbletea TUI with built in AI to manage tasks.

## Features

- **Interactive TUI**: Built with Bubble Tea and Lip Gloss
- **Task Management**: Add, list, complete, and delete tasks
- **AI Assistant**: Natural language interface that can create tasks, list them, mark complete, or delete using function calling
- **SQLite Database**: Persistent storage with automatic schema setup

## Prerequisites

- Go 1.23 or higher
- `XAI_API_KEY` environment variable (obtain from [x.ai](https://x.ai))

## Installation

```bash
# Clone the repository
git clone https://github.com/lufraser/gotaskmanager.git
cd gotaskmanager

# Install dependencies
go mod tidy
```

## Quick Start

```bash
# Set your API key
export XAI_API_KEY=your_key_here

# Run the application
go run .
```

## Usage

### Menu Options
1. **AI Task Manager** - Chat with Grok to manage tasks naturally
2. **Add Task** - Interactive form to create detailed tasks
3. **List Tasks** - View all tasks in the database
4. **Complete Task** - Mark a task as completed by ID
5. **Delete Task** - Remove a task by ID

### AI Task Manager
Launch an interactive chat where you can say things like:
- "Add tasks to prepare Q3 budget and schedule team review"
- "List all my high priority tasks"
- "Complete task 3"
- "What's on my plate for this week?"


