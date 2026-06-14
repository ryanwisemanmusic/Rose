package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Entry struct {
	ID        int64
	SessionID string
	Project   string
	Role      string
	Content   string
	Language  string
	ExitCode  int
	CreatedAt time.Time
}

type Experience struct {
	ID        int64
	Prompt    string
	Response  string
	Code      string
	Language  string
	ExitCode  int
	Stdout    string
	Stderr    string
	Changes   string
	Success   bool
	Project   string
	RelatedID int64
	CreatedAt time.Time
}

type Store struct {
	db *sql.DB
}

const dbOpTimeout = 2 * time.Second

func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA busy_timeout = 1000`); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func dbContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), dbOpTimeout)
}

func (s *Store) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS experiences (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			prompt TEXT NOT NULL,
			response TEXT NOT NULL,
			code TEXT NOT NULL DEFAULT '',
			language TEXT NOT NULL DEFAULT '',
			exit_code INTEGER NOT NULL DEFAULT 0,
			stdout TEXT NOT NULL DEFAULT '',
			stderr TEXT NOT NULL DEFAULT '',
			changes TEXT NOT NULL DEFAULT '',
			success INTEGER NOT NULL DEFAULT 0,
			project TEXT NOT NULL DEFAULT '',
			related_id INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS chat_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			project TEXT NOT NULL DEFAULT '',
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			language TEXT NOT NULL DEFAULT '',
			exit_code INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_experiences_prompt ON experiences(prompt)`,
		`CREATE INDEX IF NOT EXISTS idx_experiences_language ON experiences(language)`,
		`CREATE INDEX IF NOT EXISTS idx_experiences_exit ON experiences(exit_code)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_entries_session ON chat_entries(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_entries_project ON chat_entries(project)`,
	}
	for _, q := range queries {
		ctx, cancel := dbContext()
		_, err := s.db.ExecContext(ctx, q)
		cancel()
		if err != nil {
			return err
		}
	}

	if err := s.addColumnIfMissing("experiences", "project", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.addColumnIfMissing("experiences", "changes", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	ctx, cancel := dbContext()
	_, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_experiences_project ON experiences(project)`)
	cancel()
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) SaveExperience(prompt, response, code, language, stdout, stderr string, exitCode int, project string, relatedID int64) error {
	return s.SaveExperienceWithChanges(prompt, response, code, language, stdout, stderr, "", exitCode, project, relatedID)
}

func (s *Store) SaveExperienceWithChanges(prompt, response, code, language, stdout, stderr, changes string, exitCode int, project string, relatedID int64) error {
	success := 0
	if exitCode == 0 {
		success = 1
	}
	ctx, cancel := dbContext()
	defer cancel()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO experiences (prompt, response, code, language, exit_code, stdout, stderr, changes, success, project, related_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		prompt, response, code, language, exitCode, stdout, stderr, changes, success, project, relatedID,
	)
	return err
}

func (s *Store) SaveMessage(sessionID, project, role, content, language string, exitCode int) error {
	ctx, cancel := dbContext()
	defer cancel()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO chat_entries (session_id, project, role, content, language, exit_code)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID, project, role, content, language, exitCode,
	)
	return err
}

func (s *Store) RecentEntries(project string, limit int) ([]Entry, error) {
	ctx, cancel := dbContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, project, role, content, language, exit_code, created_at
		 FROM chat_entries
		 WHERE (? = '' OR project = ?)
		 ORDER BY created_at DESC
		 LIMIT ?`,
		project, project, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var createdAt string
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Project, &e.Role, &e.Content,
			&e.Language, &e.ExitCode, &createdAt); err != nil {
			continue
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		entries = append(entries, e)
	}
	return entries, nil
}

func (s *Store) FindSimilar(prompt string, limit int) ([]Experience, error) {
	ctx, cancel := dbContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, prompt, response, code, language, exit_code, stdout, stderr, changes, success, project, related_id, created_at
		 FROM experiences
		 WHERE prompt LIKE ? OR code LIKE ?
		 ORDER BY created_at DESC
		 LIMIT ?`,
		"%"+prompt[:min(len(prompt), 100)]+"%",
		"%"+prompt[:min(len(prompt), 100)]+"%",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exps []Experience
	for rows.Next() {
		var e Experience
		var createdAt string
		if err := rows.Scan(&e.ID, &e.Prompt, &e.Response, &e.Code, &e.Language,
			&e.ExitCode, &e.Stdout, &e.Stderr, &e.Changes, &e.Success, &e.Project, &e.RelatedID, &createdAt); err != nil {
			continue
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		exps = append(exps, e)
	}
	return exps, nil
}

func (s *Store) FindByExitCode(exitCode int, limit int) ([]Experience, error) {
	ctx, cancel := dbContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, prompt, response, code, language, exit_code, stdout, stderr, changes, success, project, related_id, created_at
		 FROM experiences WHERE exit_code = ? ORDER BY created_at DESC LIMIT ?`,
		exitCode, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exps []Experience
	for rows.Next() {
		var e Experience
		var createdAt string
		if err := rows.Scan(&e.ID, &e.Prompt, &e.Response, &e.Code, &e.Language,
			&e.ExitCode, &e.Stdout, &e.Stderr, &e.Changes, &e.Success, &e.Project, &e.RelatedID, &createdAt); err != nil {
			continue
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		exps = append(exps, e)
	}
	return exps, nil
}

func (s *Store) FindByProject(project string, limit int) ([]Experience, error) {
	ctx, cancel := dbContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, prompt, response, code, language, exit_code, stdout, stderr, changes, success, project, related_id, created_at
		 FROM experiences WHERE project = ? ORDER BY created_at DESC LIMIT ?`,
		project, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exps []Experience
	for rows.Next() {
		var e Experience
		var createdAt string
		if err := rows.Scan(&e.ID, &e.Prompt, &e.Response, &e.Code, &e.Language,
			&e.ExitCode, &e.Stdout, &e.Stderr, &e.Changes, &e.Success, &e.Project, &e.RelatedID, &createdAt); err != nil {
			continue
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		exps = append(exps, e)
	}
	return exps, nil
}

func (s *Store) GetStats() (total int, success int, projects int, langs map[string]int, err error) {
	langs = make(map[string]int)
	ctx, cancel := dbContext()
	defer cancel()
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM experiences`).Scan(&total); err != nil {
		return
	}
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM experiences WHERE success = 1`).Scan(&success); err != nil {
		return
	}
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT project) FROM experiences WHERE project != ''`).Scan(&projects); err != nil {
		return
	}
	rows, err := s.db.QueryContext(ctx, `SELECT language, COUNT(*) FROM experiences GROUP BY language`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var lang string
		var count int
		if rows.Scan(&lang, &count) == nil && lang != "" {
			langs[lang] = count
		}
	}
	return
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) addColumnIfMissing(table, column, definition string) error {
	ctx, cancel := dbContext()
	defer cancel()
	_, err := s.db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, definition))
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return err
	}
	return nil
}
