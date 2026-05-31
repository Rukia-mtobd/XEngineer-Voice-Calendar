package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Event struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	ScheduledAt string `json:"scheduledAt"`
	Reminded    bool   `json:"reminded"`
	CreatedAt   string `json:"createdAt"`
}

type Database struct {
	conn *sql.DB
}

func Open() (*Database, error) {
	dir, err := dataDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	path := filepath.Join(dir, "calendar.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)

	d := &Database{conn: conn}
	if err := d.migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return d, nil
}

func (d *Database) Close() error {
	return d.conn.Close()
}

func dataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "XEngineer-Voice-Calendar"), nil
}

func (d *Database) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS config (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT NOT NULL,
	scheduled_at TEXT NOT NULL,
	reminded INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT (datetime('now', 'localtime'))
);`
	_, err := d.conn.Exec(schema)
	return err
}

func (d *Database) GetConfig(key string) (string, error) {
	var value string
	err := d.conn.QueryRow(`SELECT value FROM config WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (d *Database) SetConfig(key, value string) error {
	_, err := d.conn.Exec(`
INSERT INTO config (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func (d *Database) CreateEvent(title string, scheduledAt time.Time) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO events (title, scheduled_at) VALUES (?, ?)`,
		title,
		scheduledAt.Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *Database) UpdateEvent(id int64, title string, scheduledAt time.Time) error {
	_, err := d.conn.Exec(
		`UPDATE events SET title = ?, scheduled_at = ? WHERE id = ?`,
		title,
		scheduledAt.Format(time.RFC3339),
		id,
	)
	return err
}

func (d *Database) DeleteEvent(id int64) error {
	_, err := d.conn.Exec(`DELETE FROM events WHERE id = ?`, id)
	return err
}

func (d *Database) ListEvents() ([]Event, error) {
	rows, err := d.conn.Query(`
SELECT id, title, scheduled_at, reminded, created_at
FROM events ORDER BY scheduled_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var reminded int
		if err := rows.Scan(&e.ID, &e.Title, &e.ScheduledAt, &reminded, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Reminded = reminded == 1
		events = append(events, e)
	}
	return events, rows.Err()
}

func (d *Database) ListEventsByDate(date time.Time) ([]Event, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	end := start.Add(24 * time.Hour)
	rows, err := d.conn.Query(`
SELECT id, title, scheduled_at, reminded, created_at
FROM events
WHERE scheduled_at >= ? AND scheduled_at < ?
ORDER BY scheduled_at ASC`,
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var reminded int
		if err := rows.Scan(&e.ID, &e.Title, &e.ScheduledAt, &reminded, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Reminded = reminded == 1
		events = append(events, e)
	}
	return events, rows.Err()
}

func (d *Database) ListDueEvents(now time.Time) ([]Event, error) {
	rows, err := d.conn.Query(`
SELECT id, title, scheduled_at, reminded, created_at
FROM events
WHERE reminded = 0 AND scheduled_at <= ?
ORDER BY scheduled_at ASC`,
		now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var reminded int
		if err := rows.Scan(&e.ID, &e.Title, &e.ScheduledAt, &reminded, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Reminded = reminded == 1
		events = append(events, e)
	}
	return events, rows.Err()
}

func (d *Database) MarkReminded(id int64) error {
	_, err := d.conn.Exec(`UPDATE events SET reminded = 1 WHERE id = ?`, id)
	return err
}

func (d *Database) GetEvent(id int64) (*Event, error) {
	var e Event
	var reminded int
	err := d.conn.QueryRow(`
SELECT id, title, scheduled_at, reminded, created_at
FROM events WHERE id = ?`, id).Scan(&e.ID, &e.Title, &e.ScheduledAt, &reminded, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("日程不存在: %d", id)
	}
	if err != nil {
		return nil, err
	}
	e.Reminded = reminded == 1
	return &e, nil
}
