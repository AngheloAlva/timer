CREATE TABLE projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    color       TEXT,
    archived    INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE INDEX idx_projects_archived ON projects(archived);

CREATE TABLE tasks (
    id            TEXT PRIMARY KEY,
    project_id    TEXT NOT NULL,
    title         TEXT NOT NULL,
    description   TEXT,
    status        TEXT NOT NULL DEFAULT 'todo'
                    CHECK (status IN ('todo', 'in_progress', 'done', 'archived')),
    external_ref  TEXT,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX idx_tasks_project_status ON tasks(project_id, status);
CREATE INDEX idx_tasks_external_ref ON tasks(external_ref);

CREATE TABLE timers (
    id                TEXT PRIMARY KEY,
    task_id           TEXT NOT NULL UNIQUE,
    started_at        INTEGER NOT NULL,
    note              TEXT,
    source            TEXT NOT NULL
                        CHECK (source IN ('cli', 'tui', 'mcp', 'manual')),
    paused_at         INTEGER,
    paused_total_sec  INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE TABLE time_entries (
    id           TEXT PRIMARY KEY,
    task_id      TEXT NOT NULL,
    started_at   INTEGER NOT NULL,
    ended_at     INTEGER NOT NULL,
    duration_sec INTEGER NOT NULL,
    note         TEXT,
    source       TEXT NOT NULL
                   CHECK (source IN ('cli', 'tui', 'mcp', 'manual')),
    created_at   INTEGER NOT NULL,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX idx_time_entries_started ON time_entries(started_at);
CREATE INDEX idx_time_entries_task_started ON time_entries(task_id, started_at);

CREATE TABLE tags (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    color      TEXT NOT NULL DEFAULT '#94a3b8',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE task_tags (
    task_id TEXT NOT NULL,
    tag_id  TEXT NOT NULL,
    PRIMARY KEY (task_id, tag_id),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE TABLE timer_tags (
    timer_id TEXT NOT NULL,
    tag_id   TEXT NOT NULL,
    PRIMARY KEY (timer_id, tag_id),
    FOREIGN KEY (timer_id) REFERENCES timers(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE TABLE time_entry_tags (
    time_entry_id TEXT NOT NULL,
    tag_id        TEXT NOT NULL,
    PRIMARY KEY (time_entry_id, tag_id),
    FOREIGN KEY (time_entry_id) REFERENCES time_entries(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);
