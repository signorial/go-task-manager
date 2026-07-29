CREATE TABLE IF NOT EXISTS tasks (
    task_id INTEGER PRIMARY KEY AUTOINCREMENT,
    description TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TEXT,
    updated_at TEXT,
    priority TEXT,
    assignee_id INTEGER,
    do_date TEXT,
    final_due_date TEXT,
    start_time TEXT,
    end_time TEXT,
    completed_at TEXT,
    estimated_hours REAL,
    progress INTEGER,
    parent_task_id INTEGER,
    deleted INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sync_meta (
    key TEXT PRIMARY KEY,
    value TEXT
);

CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY,
    summary TEXT,
    description TEXT,
    start_timc TEXT,
    end_time TEXT,
    updated_at TEXT,
    update_tasks_db INTEGER DEFAULT 0,
    update_calendar INTEGER DEFAULT 0,
    deleted INTEGER DEFAULT 0,
    task_id INTEGER REFERENCES tasks (task_id)
);
select * from tasks


--TODO: need to figure out how to run queries directly in neovim
