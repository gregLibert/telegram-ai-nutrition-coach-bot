-- Telegram AI Nutrition Coach Bot — SQLite schema (embedded copy for go:embed)

PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_id     INTEGER UNIQUE,
    chat_id         INTEGER,
    username        TEXT NOT NULL DEFAULT '',
    state           TEXT NOT NULL DEFAULT 'idle',
    state_data      TEXT NOT NULL DEFAULT '{}',
    timezone        TEXT NOT NULL DEFAULT 'Europe/Paris',
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS profiles (
    user_id             INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    age                 INTEGER NOT NULL,
    height_cm           REAL NOT NULL,
    weight_kg           REAL NOT NULL,
    target_weight_kg    REAL NOT NULL,
    gender              TEXT NOT NULL CHECK (gender IN ('male', 'female')),
    activity_level      TEXT NOT NULL CHECK (activity_level IN (
                            'sedentary', 'light', 'moderate', 'active', 'very_active'
                        )),
    bmr                 REAL NOT NULL,
    tdee                REAL NOT NULL,
    target_calories     REAL NOT NULL,
    target_protein_g    REAL NOT NULL,
    target_fat_g        REAL NOT NULL,
    target_carbs_g      REAL NOT NULL,
    weight_baseline_kg  REAL NOT NULL,
    diet_break_until    TEXT,
    forfait_adjustment  REAL NOT NULL DEFAULT 0,
    excluded_ingredients TEXT NOT NULL DEFAULT '',
    region              TEXT NOT NULL DEFAULT '',
    updated_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS weight_entries (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    weight_kg   REAL NOT NULL,
    recorded_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_weight_entries_user_time
    ON weight_entries(user_id, recorded_at DESC);

CREATE TABLE IF NOT EXISTS meals (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    description TEXT NOT NULL DEFAULT '',
    calories    REAL NOT NULL,
    protein_g   REAL NOT NULL,
    fat_g       REAL NOT NULL,
    carbs_g     REAL NOT NULL,
    source      TEXT NOT NULL DEFAULT 'text' CHECK (source IN ('text', 'voice', 'photo')),
    logged_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_meals_user_day
    ON meals(user_id, logged_at DESC);

CREATE TABLE IF NOT EXISTS forfait_entries (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    preset_key      TEXT NOT NULL,
    calories        REAL NOT NULL,
    days_remaining  INTEGER NOT NULL DEFAULT 3,
    daily_offset    REAL NOT NULL,
    logged_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS activity_sync (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider        TEXT NOT NULL CHECK (provider IN ('polar', 'google_fit')),
    access_token    TEXT NOT NULL DEFAULT '',
    refresh_token   TEXT NOT NULL DEFAULT '',
    last_sync_at    TEXT,
    active_calories REAL NOT NULL DEFAULT 0,
    synced_date     TEXT NOT NULL DEFAULT (date('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_activity_sync_user_provider
    ON activity_sync(user_id, provider);

CREATE TABLE IF NOT EXISTS llm_audit_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER,
    operation       TEXT NOT NULL,
    model           TEXT NOT NULL,
    prompt          TEXT NOT NULL,
    raw_response    TEXT NOT NULL,
    tokens_prompt   INTEGER NOT NULL DEFAULT 0,
    tokens_output   INTEGER NOT NULL DEFAULT 0,
    latency_ms      INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_llm_audit_user_time
    ON llm_audit_log(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS scheduler_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_type    TEXT NOT NULL,
    sent_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_scheduler_log_user_job
    ON scheduler_log(user_id, job_type, sent_at DESC);

CREATE TABLE IF NOT EXISTS user_integrations (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id          INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider         TEXT NOT NULL CHECK (provider IN ('polar', 'google_fit')),
    access_token     TEXT NOT NULL DEFAULT '',
    refresh_token    TEXT NOT NULL DEFAULT '',
    token_expiry     TEXT,
    external_user_id TEXT NOT NULL DEFAULT '',
    updated_at       TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(user_id, provider)
);

CREATE TABLE IF NOT EXISTS daily_logs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    log_date        TEXT NOT NULL,
    base_tdee       REAL NOT NULL DEFAULT 0,
    active_calories REAL NOT NULL DEFAULT 0,
    adjusted_tdee   REAL NOT NULL DEFAULT 0,
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(user_id, log_date)
);

CREATE INDEX IF NOT EXISTS idx_daily_logs_user_date
    ON daily_logs(user_id, log_date DESC);
