package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
	_ "modernc.org/sqlite"
)

// SessionRecord stores the summary metrics of a completed guitar practice session.
type SessionRecord struct {
	ID                string    `json:"id"`
	StudentID         string    `json:"student_id"`
	StartedAt         time.Time `json:"started_at"`
	EndedAt           time.Time `json:"ended_at"`
	DurationSec       int       `json:"duration_sec"`
	TotalStrums       int       `json:"total_strums"`
	CleanStrums       int       `json:"clean_strums"`
	TuningOffsetCents float64   `json:"tuning_offset_cents"`
}

// MistakeSummary aggregates frequency counts of recurring physical mistakes.
type MistakeSummary struct {
	ChordName       string    `json:"chord_name"`
	MistakeType     string    `json:"mistake_type"`
	Details         string    `json:"details"`
	OccurrenceCount int       `json:"occurrence_count"`
	LastOccurredAt  time.Time `json:"last_occurred_at"`
}

// StudentSummary provides an overall profile of a student's practice history.
type StudentSummary struct {
	StudentID       string           `json:"student_id"`
	TotalSessions   int              `json:"total_sessions"`
	TotalPracticeSec int             `json:"total_practice_sec"`
	TotalStrums     int              `json:"total_strums"`
	CleanStrums     int              `json:"clean_strums"`
	OverallAccuracy float64          `json:"overall_accuracy"`
	TopMistakes     []MistakeSummary `json:"top_mistakes"`
	CurrentStepID   int              `json:"current_step_id"`
}

// SQLiteStore implements MemoryStore backed by an embedded SQLite database.
type SQLiteStore struct {
	db *sql.DB
	mu sync.Mutex
}

// NewSQLiteStore initializes SQLite at the given path and creates the schema.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %s: %w", dbPath, err)
	}

	// Enable WAL mode and configure connection pool for embedded reliability
	if _, err := db.Exec("PRAGMA journal_mode = WAL; PRAGMA foreign_keys = ON;"); err != nil {
		log.Printf("[SQLite] Warning: failed to set WAL mode: %v", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize sqlite schema: %w", err)
	}

	log.Printf("[SQLite] Database initialized successfully at %s", dbPath)
	return store, nil
}

func (s *SQLiteStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS conversation_turns (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_turns_session ON conversation_turns(session_id);

	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		student_id TEXT NOT NULL,
		started_at DATETIME NOT NULL,
		ended_at DATETIME NOT NULL,
		duration_sec INTEGER NOT NULL,
		total_strums INTEGER NOT NULL,
		clean_strums INTEGER NOT NULL,
		tuning_offset_cents REAL DEFAULT 0.0
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_student ON sessions(student_id);

	CREATE TABLE IF NOT EXISTS curriculum_progress (
		student_id TEXT PRIMARY KEY,
		current_step_id INTEGER NOT NULL DEFAULT 1,
		current_streak INTEGER NOT NULL DEFAULT 0,
		completed_steps_json TEXT NOT NULL DEFAULT '[]',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS student_mistakes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		student_id TEXT NOT NULL,
		chord_name TEXT NOT NULL,
		mistake_type TEXT NOT NULL,
		details TEXT NOT NULL,
		occurrence_count INTEGER NOT NULL DEFAULT 1,
		last_occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(student_id, chord_name, mistake_type, details)
	);
	CREATE INDEX IF NOT EXISTS idx_mistakes_student ON student_mistakes(student_id);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Close closes the underlying SQLite database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// ── MemoryStore Interface Implementation ────────────────────────────────────

// Read retrieves conversation turns for a session from the conversation_turns table.
func (s *SQLiteStore) Read(ctx context.Context, sessionID string) ([]openai.ChatCompletionMessage, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT role, content FROM conversation_turns WHERE session_id = ? ORDER BY id ASC",
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query turns: %w", err)
	}
	defer rows.Close()

	var messages []openai.ChatCompletionMessage
	for rows.Next() {
		var role, content string
		if err := rows.Scan(&role, &content); err != nil {
			return nil, fmt.Errorf("failed to scan turn: %w", err)
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    role,
			Content: content,
		})
	}
	return messages, rows.Err()
}

// Write replaces the conversation history for a session.
func (s *SQLiteStore) Write(ctx context.Context, sessionID string, history []openai.ChatCompletionMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM conversation_turns WHERE session_id = ?", sessionID); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO conversation_turns (session_id, role, content) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, msg := range history {
		if _, err := stmt.ExecContext(ctx, sessionID, msg.Role, msg.Content); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Append appends one or more messages to the conversation history.
func (s *SQLiteStore) Append(ctx context.Context, sessionID string, messages ...openai.ChatCompletionMessage) error {
	if len(messages) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO conversation_turns (session_id, role, content) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, msg := range messages {
		if _, err := stmt.ExecContext(ctx, sessionID, msg.Role, msg.Content); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ── Guitar Pedagogy Persistence Methods ─────────────────────────────────────

// SaveSession logs a completed practice session record.
func (s *SQLiteStore) SaveSession(rec SessionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO sessions (id, student_id, started_at, ended_at, duration_sec, total_strums, clean_strums, tuning_offset_cents)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.Exec(query,
		rec.ID, rec.StudentID, rec.StartedAt, rec.EndedAt,
		rec.DurationSec, rec.TotalStrums, rec.CleanStrums, rec.TuningOffsetCents,
	)
	return err
}

// SaveCurriculumProgress records a student's current position and completed steps.
func (s *SQLiteStore) SaveCurriculumProgress(studentID string, stepID, streak int, completedSteps []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(completedSteps)
	if err != nil {
		return err
	}

	query := `
	INSERT INTO curriculum_progress (student_id, current_step_id, current_streak, completed_steps_json, updated_at)
	VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(student_id) DO UPDATE SET
		current_step_id = excluded.current_step_id,
		current_streak = excluded.current_streak,
		completed_steps_json = excluded.completed_steps_json,
		updated_at = CURRENT_TIMESTAMP
	`
	_, err = s.db.Exec(query, studentID, stepID, streak, string(data))
	return err
}

// GetCurriculumProgress retrieves the saved curriculum step and completed steps.
func (s *SQLiteStore) GetCurriculumProgress(studentID string) (stepID, streak int, completedSteps []int, err error) {
	query := "SELECT current_step_id, current_streak, completed_steps_json FROM curriculum_progress WHERE student_id = ?"
	var jsonStr string
	err = s.db.QueryRow(query, studentID).Scan(&stepID, &streak, &jsonStr)
	if err == sql.ErrNoRows {
		return 1, 0, nil, nil
	}
	if err != nil {
		return 0, 0, nil, err
	}

	if jsonStr != "" {
		_ = json.Unmarshal([]byte(jsonStr), &completedSteps)
	}
	return stepID, streak, completedSteps, nil
}

// RecordMistake upserts a mistake occurrence for a student, incrementing its count.
func (s *SQLiteStore) RecordMistake(studentID, chordName, mistakeType, details string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO student_mistakes (student_id, chord_name, mistake_type, details, occurrence_count, last_occurred_at)
	VALUES (?, ?, ?, ?, 1, CURRENT_TIMESTAMP)
	ON CONFLICT(student_id, chord_name, mistake_type, details) DO UPDATE SET
		occurrence_count = student_mistakes.occurrence_count + 1,
		last_occurred_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(query, studentID, chordName, mistakeType, details)
	return err
}

// GetTopMistakes returns the most frequent mistakes made by a student.
func (s *SQLiteStore) GetTopMistakes(studentID string, limit int) ([]MistakeSummary, error) {
	if limit <= 0 {
		limit = 5
	}
	query := `
	SELECT chord_name, mistake_type, details, occurrence_count, last_occurred_at
	FROM student_mistakes
	WHERE student_id = ?
	ORDER BY occurrence_count DESC
	LIMIT ?
	`
	rows, err := s.db.Query(query, studentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mistakes []MistakeSummary
	for rows.Next() {
		var m MistakeSummary
		if err := rows.Scan(&m.ChordName, &m.MistakeType, &m.Details, &m.OccurrenceCount, &m.LastOccurredAt); err != nil {
			return nil, err
		}
		mistakes = append(mistakes, m)
	}
	return mistakes, rows.Err()
}

// GetStudentSummary aggregates practice stats across all sessions for a student.
func (s *SQLiteStore) GetStudentSummary(studentID string) (*StudentSummary, error) {
	summary := &StudentSummary{StudentID: studentID}

	// 1. Practice sessions aggregate
	row := s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(duration_sec), 0), COALESCE(SUM(total_strums), 0), COALESCE(SUM(clean_strums), 0)
		FROM sessions WHERE student_id = ?
	`, studentID)
	if err := row.Scan(&summary.TotalSessions, &summary.TotalPracticeSec, &summary.TotalStrums, &summary.CleanStrums); err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if summary.TotalStrums > 0 {
		summary.OverallAccuracy = float64(summary.CleanStrums) / float64(summary.TotalStrums)
	}

	// 2. Curriculum step
	stepID, _, _, err := s.GetCurriculumProgress(studentID)
	if err == nil {
		summary.CurrentStepID = stepID
	}

	// 3. Top mistakes
	topMistakes, err := s.GetTopMistakes(studentID, 3)
	if err == nil {
		summary.TopMistakes = topMistakes
	}

	return summary, nil
}
