"""SQLite connection helpers and schema initialisation for CronFetch."""
import os
import sqlite3

BASE_DIR = os.path.abspath(os.path.dirname(__file__))
INSTANCE_DIR = os.path.join(BASE_DIR, "instance")
DB_PATH = os.path.join(INSTANCE_DIR, "cronfetch.db")

SCHEMA = """
CREATE TABLE IF NOT EXISTS sites (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    name              TEXT NOT NULL,
    url               TEXT NOT NULL,
    css_selector      TEXT,
    item_selector     TEXT,
    selector_source   TEXT,
    min_experience    INTEGER,
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    last_checked      TEXT,
    last_content_hash TEXT
);

CREATE TABLE IF NOT EXISTS jobs (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id        INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    title          TEXT,
    url            TEXT,
    job_key        TEXT,
    experience_min REAL,
    experience_max REAL,
    found_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS history (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    message TEXT NOT NULL,
    sent_at TEXT NOT NULL DEFAULT (datetime('now')),
    status  TEXT
);
"""

# Created after migrations so the columns they reference are guaranteed to exist.
INDEXES = """
CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_site_key ON jobs(site_id, job_key);
"""

# Idempotent column adds for databases created before these fields existed.
# SQLite ignores nothing here, so each is guarded by a table_info check.
MIGRATIONS = {
    "sites": [
        ("item_selector", "ALTER TABLE sites ADD COLUMN item_selector TEXT"),
        ("selector_source", "ALTER TABLE sites ADD COLUMN selector_source TEXT"),
        ("min_experience", "ALTER TABLE sites ADD COLUMN min_experience INTEGER"),
    ],
    "jobs": [
        ("job_key", "ALTER TABLE jobs ADD COLUMN job_key TEXT"),
        ("experience_min", "ALTER TABLE jobs ADD COLUMN experience_min REAL"),
        ("experience_max", "ALTER TABLE jobs ADD COLUMN experience_max REAL"),
    ],
}


def get_connection():
    """Return a new SQLite connection with Row access and FK enforcement."""
    os.makedirs(INSTANCE_DIR, exist_ok=True)
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")
    return conn


def _run_migrations(conn):
    for table, changes in MIGRATIONS.items():
        existing = {row["name"] for row in conn.execute(f"PRAGMA table_info({table})")}
        for column, ddl in changes:
            if column not in existing:
                conn.execute(ddl)


def init_db():
    """Create tables if missing, then apply any pending column migrations."""
    conn = get_connection()
    try:
        conn.executescript(SCHEMA)
        _run_migrations(conn)
        conn.executescript(INDEXES)
        conn.commit()
    finally:
        conn.close()
