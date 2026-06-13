package memory

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Entry struct {
	ID        int64
	Role      string
	Content   string
	Language  string
	ExitCode  int
	CreatedAt time.Time
}

type Experience struct {
	ID          int64
	Prompt      string
	Response    string
	Code        string
	Language    string
	ExitCode    int
	Stdout      string
	Stderr      string
	Success     bool
	Project     string
	RelatedID   int64
	CreatedAt   time.Time
}

type Store struct {
	db *sql.DB
}

func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
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
			success INTEGER NOT NULL DEFAULT 0,
			project TEXT NOT NULL DEFAULT '',
			related_id INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_experiences_prompt ON experiences(prompt)`,
		`CREATE INDEX IF NOT EXISTS idx_experiences_language ON experiences(language)`,
		`CREATE INDEX IF NOT EXISTS idx_experiences_exit ON experiences(exit_code)`,
	}
	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}

	s.db.Exec(`ALTER TABLE experiences ADD COLUMN project TEXT`)
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_experiences_project ON experiences(project)`); err != nil {
		if !strings.Contains(err.Error(), "no such column") {
			return err
		}
	}

	return nil
}

func (s *Store) SaveExperience(prompt, response, code, language, stdout, stderr string, exitCode int, project string, relatedID int64) error {
	success := 0
	if exitCode == 0 {
		success = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO experiences (prompt, response, code, language, exit_code, stdout, stderr, success, project, related_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		prompt, response, code, language, exitCode, stdout, stderr, success, project, relatedID,
	)
	return err
}

func (s *Store) FindSimilar(prompt string, limit int) ([]Experience, error) {
	rows, err := s.db.Query(
		`SELECT id, prompt, response, code, language, exit_code, stdout, stderr, success, project, related_id, created_at
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
			&e.ExitCode, &e.Stdout, &e.Stderr, &e.Success, &e.Project, &e.RelatedID, &createdAt); err != nil {
			continue
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		exps = append(exps, e)
	}
	return exps, nil
}

func (s *Store) FindByExitCode(exitCode int, limit int) ([]Experience, error) {
	rows, err := s.db.Query(
		`SELECT id, prompt, response, code, language, exit_code, stdout, stderr, success, project, related_id, created_at
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
			&e.ExitCode, &e.Stdout, &e.Stderr, &e.Success, &e.Project, &e.RelatedID, &createdAt); err != nil {
			continue
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		exps = append(exps, e)
	}
	return exps, nil
}

func (s *Store) FindByProject(project string, limit int) ([]Experience, error) {
	rows, err := s.db.Query(
		`SELECT id, prompt, response, code, language, exit_code, stdout, stderr, success, project, related_id, created_at
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
			&e.ExitCode, &e.Stdout, &e.Stderr, &e.Success, &e.Project, &e.RelatedID, &createdAt); err != nil {
			continue
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		exps = append(exps, e)
	}
	return exps, nil
}

func (s *Store) GetStats() (total int, success int, projects int, langs map[string]int, err error) {
	langs = make(map[string]int)
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM experiences`).Scan(&total); err != nil {
		return
	}
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM experiences WHERE success = 1`).Scan(&success); err != nil {
		return
	}
	if err = s.db.QueryRow(`SELECT COUNT(DISTINCT project) FROM experiences WHERE project != ''`).Scan(&projects); err != nil {
		return
	}
	rows, err := s.db.Query(`SELECT language, COUNT(*) FROM experiences GROUP BY language`)
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
