INSERT INTO tasks (
    task_id, 
    description, 
    status, 
    created_at, 
    updated_at, 
    priority, 
    assignee_id, 
    do_date, 
    final_due_date, 
    start_time, 
    end_time, 
    completed_at, 
    estimated_hours, 
    progress, 
    parent_task_id
) VALUES 
('101', 'Initialize git repository', 'COMPLETED', '2024-03-01 10:00:00', '2024-03-01 10:30:00', 'HIGH', 1, '2024-03-01 00:00:00', '2024-03-02 23:59:59', '2024-03-01 10:05:00', '2024-03-01 10:25:00', '2024-03-01 10:30:00', 0.5, 100, NULL),
('102', 'Write unit tests for auth', 'IN_PROGRESS', '2024-03-02 09:00:00', '2024-03-02 11:45:00', 'MEDIUM', 2, '2024-03-03 00:00:00', '2024-03-05 17:00:00', '2024-03-02 09:15:00', NULL, NULL, 4.0, 65, NULL),
('103', 'Sub-task: Auth mock data', 'TODO', '2024-03-02 09:10:00', '2024-03-02 09:10:00', 'LOW', 2, '2024-03-03 00:00:00', '2024-03-04 12:00:00', NULL, NULL, NULL, 1.5, 0, '102');

