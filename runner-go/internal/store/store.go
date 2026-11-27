package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/ledger"
	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/phase"
)

// ErrStateCorrupt is returned by every mutator when the database file is
// damaged. The only way out is the explicit recover operation.
var ErrStateCorrupt = errors.New("estado corrupto: ejecuta runner recover")

// ErrRequestIDReused reports the same request_id used for another operation.
var ErrRequestIDReused = errors.New("request_id ya usado con otra operación")

// Store is the project database: .run/orchestrator.db.
type Store struct {
	Root    string
	Path    string
	db      *sql.DB
	ro      bool
	corrupt bool
}

// PhaseState is the state of the phases of one orchestrator session.
type PhaseState struct {
	Exists        bool             `json:"exists"`
	Phase         int              `json:"phase"`
	Valid         bool             `json:"valid"`
	EvidenceValid bool             `json:"evidence_valid"`
	Evidence      []phase.Evidence `json:"evidence"`
	Corrupt       bool             `json:"-"`
}

const schema = `
CREATE TABLE IF NOT EXISTS meta (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
	session TEXT PRIMARY KEY,
	phase INTEGER NOT NULL CHECK (phase BETWEEN 1 AND 11),
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS phase_evidence (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session TEXT NOT NULL REFERENCES sessions(session) ON DELETE CASCADE,
	phase INTEGER NOT NULL CHECK (phase BETWEEN 1 AND 11),
	text TEXT NOT NULL,
	skip_to INTEGER CHECK (skip_to IS NULL OR (skip_to BETWEEN 2 AND 11)),
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS phase_evidence_session ON phase_evidence(session, id);
CREATE TABLE IF NOT EXISTS delegations (
	id TEXT PRIMARY KEY,
	fingerprint TEXT NOT NULL,
	agent TEXT NOT NULL,
	task TEXT NOT NULL DEFAULT '',
	goal TEXT NOT NULL DEFAULT '',
	scope TEXT NOT NULL DEFAULT '[]',
	checks TEXT NOT NULL DEFAULT '[]',
	criteria TEXT NOT NULL DEFAULT '',
	state TEXT NOT NULL CHECK (state IN ('prepared','launched','returned','closed')),
	outcome TEXT CHECK (outcome IS NULL OR outcome IN ('done','superseded','absorbed')),
	prepared_phase INTEGER,
	orchestrator_session TEXT NOT NULL DEFAULT '',
	subagent_session TEXT NOT NULL DEFAULT '',
	observed TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	CHECK ((state = 'closed') = (outcome IS NOT NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS delegations_open_fingerprint
	ON delegations(fingerprint) WHERE state IN ('prepared','launched','returned');
CREATE TABLE IF NOT EXISTS delegation_returns (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	delegation_id TEXT NOT NULL REFERENCES delegations(id) ON DELETE CASCADE,
	status TEXT NOT NULL CHECK (status IN ('completed','partial','blocked')),
	checks TEXT NOT NULL DEFAULT '[]',
	risks TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS delegation_notes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	delegation_id TEXT NOT NULL REFERENCES delegations(id) ON DELETE CASCADE,
	text TEXT NOT NULL,
	observed TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS requests (
	request_id TEXT PRIMARY KEY,
	operation TEXT NOT NULL,
	response TEXT NOT NULL,
	created_at TEXT NOT NULL
);
PRAGMA user_version = 1;
`

// Open opens the project database. Creation is deferred to the first mutation:
// a read-only open and a discovery never create the file or the .run directory,
// and they answer empty state. A damaged file marks the store corrupt instead
// of failing the query.
func Open(root string, writable bool) (*Store, error) {
	s := &Store{Root: root, Path: filepath.Join(root, ".run", "orchestrator.db"), ro: !writable}
	if _, err := os.Stat(s.Path); err != nil {
		return s, nil
	}
	if err := s.connect(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) connect() error {
	dsn := "file:" + s.Path + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	if s.ro {
		dsn += "&mode=ro"
	} else {
		dsn += "&_txlock=immediate"
		if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
			return err
		}
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	var n int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master").Scan(&n); err != nil {
		db.Close()
		if isCorruption(err) {
			s.corrupt = true
			return nil
		}
		return err
	}
	if s.ro {
		if !hasSchema(db) {
			s.corrupt = true
			db.Close()
			return nil
		}
	} else if _, err := db.Exec(schema); err != nil {
		db.Close()
		if isCorruption(err) {
			s.corrupt = true
			return nil
		}
		return err
	}
	s.db = db
	return nil
}

func hasSchema(db *sql.DB) bool {
	var n int
	err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='sessions'").Scan(&n)
	return err == nil && n == 1
}

func isCorruption(err error) bool {
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"not a database", "malformed", "database disk image is corrupted", "database disk image is corrupt"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// Close releases the connection.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// ReadOnly reports whether the store was opened for queries only.
func (s *Store) ReadOnly() bool { return s.ro }

// Corrupt reports whether the file is damaged.
func (s *Store) Corrupt() bool { return s.corrupt }

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// Mutate runs fn in one BEGIN IMMEDIATE transaction. With a request_id, a
// previous response for the same key is returned verbatim with no new effect.
func (s *Store) Mutate(session, requestID, operation string, fn func(*sql.Tx) (json.RawMessage, error)) (json.RawMessage, error) {
	if s.ro {
		return nil, errors.New("la base se abrió en solo lectura")
	}
	if s.corrupt {
		return nil, ErrStateCorrupt
	}
	if err := s.ensure(); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, s.fail(err)
	}
	defer tx.Rollback()

	if requestID != "" {
		var storedOp, stored string
		err := tx.QueryRow("SELECT operation, response FROM requests WHERE request_id = ?", requestID).Scan(&storedOp, &stored)
		if err == nil {
			if storedOp != operation {
				return nil, ErrRequestIDReused
			}
			return json.RawMessage(stored), nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, s.fail(err)
		}
	}

	renames, err := s.migrateLegacy(tx, session)
	if err != nil {
		return nil, s.fail(err)
	}
	out, err := fn(tx)
	if err != nil {
		return nil, s.fail(err)
	}
	if requestID != "" {
		if _, err := tx.Exec("INSERT INTO requests(request_id, operation, response, created_at) VALUES(?,?,?,?)",
			requestID, operation, string(out), now()); err != nil {
			return nil, s.fail(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, s.fail(err)
	}
	// After the commit: the legacy files are already imported, so a failed
	// rename only leaves a renamed backup pending, never duplicates.
	for _, path := range renames {
		_ = os.Rename(path, path+".migrated")
	}
	return out, nil
}

// ensure connects on first use, so creation stays deferred until a mutation.
func (s *Store) ensure() error {
	if s.db != nil {
		return nil
	}
	if s.corrupt {
		return ErrStateCorrupt
	}
	if err := s.connect(); err != nil {
		return s.fail(err)
	}
	if s.corrupt {
		return ErrStateCorrupt
	}
	return nil
}

func (s *Store) fail(err error) error {
	if isCorruption(err) {
		s.corrupt = true
		return fmt.Errorf("%w: %v", ErrStateCorrupt, err)
	}
	return err
}

// LatestSession names the session that transitioned most recently, skipping
// degenerate rows without a session. Empty when no session has a phase.
func (s *Store) LatestSession() (string, error) {
	if s.corrupt || s.db == nil {
		return "", nil
	}
	var session string
	err := s.db.QueryRow("SELECT session FROM sessions WHERE session <> '' ORDER BY updated_at DESC, session DESC LIMIT 1").Scan(&session)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		s.corrupt = true
		return "", fmt.Errorf("%w: %v", ErrStateCorrupt, err)
	}
	return session, nil
}

// SessionPhase returns the state of the phases of one session.
func (s *Store) SessionPhase(session string) (PhaseState, error) {
	state := PhaseState{Phase: 1, Valid: true, EvidenceValid: true, Evidence: []phase.Evidence{}}
	if s.corrupt {
		state.Exists, state.Valid, state.Corrupt = true, false, true
		return state, nil
	}
	if s.db == nil {
		return state, nil
	}
	var current int
	err := s.db.QueryRow("SELECT phase FROM sessions WHERE session = ?", session).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return state, nil
	}
	if err != nil {
		s.corrupt = true
		state.Exists, state.Valid, state.Corrupt = true, false, true
		return state, nil
	}
	state.Exists, state.Phase = true, current
	evidence, err := s.evidence(session)
	if err != nil {
		s.corrupt = true
		state.Valid, state.EvidenceValid, state.Corrupt = false, false, true
		return state, nil
	}
	state.Evidence = evidence
	return state, nil
}

func (s *Store) evidence(session string) ([]phase.Evidence, error) {
	rows, err := s.db.Query("SELECT phase, text, COALESCE(skip_to, 0) FROM phase_evidence WHERE session = ? ORDER BY id", session)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []phase.Evidence{}
	for rows.Next() {
		var entry phase.Evidence
		if err := rows.Scan(&entry.Phase, &entry.Text, &entry.Skip); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

// OpenDelegations lists the executions that are still unclosed, oldest first.
func (s *Store) OpenDelegations(limit int) ([]ledger.Delegation, error) {
	if s.corrupt || s.db == nil {
		return nil, nil
	}
	query := `SELECT id, fingerprint, agent, task, goal, state, COALESCE(outcome,''), COALESCE(prepared_phase,0),
		orchestrator_session, subagent_session, created_at, updated_at
		FROM delegations WHERE state IN ('prepared','launched','returned') ORDER BY created_at, id`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, s.fail(err)
	}
	defer rows.Close()
	return scanDelegations(rows)
}

func scanDelegations(rows *sql.Rows) ([]ledger.Delegation, error) {
	var out []ledger.Delegation
	for rows.Next() {
		var d ledger.Delegation
		if err := rows.Scan(&d.ID, &d.Fingerprint, &d.Agent, &d.Task, &d.Goal, &d.State, &d.Outcome,
			&d.PreparedPhase, &d.OrchestratorSession, &d.SubagentSession, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type transition struct {
	Phase            int      `json:"phase"`
	Name             string   `json:"name"`
	EvidenceExpected string   `json:"evidence_expected"`
	Terminal         bool     `json:"terminal,omitempty"`
	Skipped          []int    `json:"skipped,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
	Reset            bool     `json:"reset,omitempty"`
}

func outcome(n int) transition {
	p := phase.Get(n)
	return transition{Phase: p.Number, Name: p.Name, EvidenceExpected: p.Evidence}
}

// Next advances one phase recording the evidence. The terminal phase does not
// advance; it answers with the terminal notice.
func (s *Store) Next(session, evidence, requestID string) (json.RawMessage, error) {
	return s.Mutate(session, requestID, "phase_next", func(tx *sql.Tx) (json.RawMessage, error) {
		current, err := sessionPhase(tx, session)
		if err != nil {
			return nil, err
		}
		result := outcome(current)
		if current >= phase.Count {
			result.Terminal = true
		} else {
			if err := setPhase(tx, session, current); err != nil {
				return nil, err
			}
			if err := appendEvidence(tx, session, current, evidence, 0); err != nil {
				return nil, err
			}
			if err := setPhase(tx, session, current+1); err != nil {
				return nil, err
			}
			result = outcome(current + 1)
		}
		result.Warnings, err = openWarnings(tx, 3)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})
}

// Skip jumps forward to a later phase recording the reason and the omitted
// phases. The transition rules match the Python runner.
func (s *Store) Skip(session string, target int, reason, requestID string) (json.RawMessage, error) {
	return s.Mutate(session, requestID, "phase_skip", func(tx *sql.Tx) (json.RawMessage, error) {
		current, err := sessionPhase(tx, session)
		if err != nil {
			return nil, err
		}
		if problem := phase.SkipProblem(current, target); problem != nil {
			return nil, &FieldError{Field: problem.Field, Reason: problem.Reason}
		}
		if err := setPhase(tx, session, current); err != nil {
			return nil, err
		}
		if err := appendEvidence(tx, session, current, reason, target); err != nil {
			return nil, err
		}
		if err := setPhase(tx, session, target); err != nil {
			return nil, err
		}
		result := outcome(target)
		result.Skipped = phase.SkippedRange(current, target)
		result.Warnings, err = openWarnings(tx, 3)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})
}

// Reset clears the phase and the evidence of this session only; delegations are
// project-wide and stay untouched.
func (s *Store) Reset(session, reason, requestID string) (json.RawMessage, error) {
	return s.Mutate(session, requestID, "phase_reset", func(tx *sql.Tx) (json.RawMessage, error) {
		if _, err := tx.Exec("DELETE FROM phase_evidence WHERE session = ?", session); err != nil {
			return nil, err
		}
		if _, err := tx.Exec("DELETE FROM sessions WHERE session = ?", session); err != nil {
			return nil, err
		}
		result := outcome(1)
		result.Reset = true
		return json.Marshal(result)
	})
}

// FieldError is a domain rejection of one input field.
type FieldError struct {
	Field  string
	Reason string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }

func sessionPhase(tx *sql.Tx, session string) (int, error) {
	var current int
	err := tx.QueryRow("SELECT phase FROM sessions WHERE session = ?", session).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return 1, nil
	}
	return current, err
}

func setPhase(tx *sql.Tx, session string, n int) error {
	_, err := tx.Exec(`INSERT INTO sessions(session, phase, updated_at) VALUES(?,?,?)
		ON CONFLICT(session) DO UPDATE SET phase = excluded.phase, updated_at = excluded.updated_at`,
		session, n, now())
	return err
}

// SetPhase registers the phase of one session without recording evidence; it
// is the public entry used by `runner start` to create the row in phase 1.
func (s *Store) SetPhase(session string, n int, requestID string) (json.RawMessage, error) {
	if n < 1 || n > phase.Count {
		return nil, &FieldError{Field: "phase", Reason: "fase fuera de rango"}
	}
	return s.Mutate(session, requestID, "phase_start", func(tx *sql.Tx) (json.RawMessage, error) {
		// A missing row is created; an existing row keeps its current phase.
		var current int
		err := tx.QueryRow("SELECT phase FROM sessions WHERE session = ?", session).Scan(&current)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if errors.Is(err, sql.ErrNoRows) {
			if err := setPhase(tx, session, n); err != nil {
				return nil, err
			}
		}
		return json.Marshal(outcome(n))
	})
}

func appendEvidence(tx *sql.Tx, session string, from int, text string, skip int) error {
	var skipValue any
	if skip > 0 {
		skipValue = skip
	}
	_, err := tx.Exec("INSERT INTO phase_evidence(session, phase, text, skip_to, created_at) VALUES(?,?,?,?,?)",
		session, from, text, skipValue, now())
	return err
}

func openWarnings(tx *sql.Tx, limit int) ([]string, error) {
	rows, err := tx.Query(`SELECT id, state, task, goal FROM delegations
		WHERE state IN ('prepared','launched','returned') ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lines []string
	var total int
	for rows.Next() {
		var id, state, task, goal string
		if err := rows.Scan(&id, &state, &task, &goal); err != nil {
			return nil, err
		}
		total++
		if len(lines) >= limit {
			continue
		}
		label := map[string]string{
			ledger.StatePrepared: "sin lanzar y sin contraste declarado",
			ledger.StateLaunched: "lanzado sin retorno",
			ledger.StateReturned: "retorno pendiente de integración",
		}[state]
		what := task
		if what == "" {
			what = truncate(goal, 40)
		}
		line := fmt.Sprintf("encargo %s (%s): %s %s", state, label, id, what)
		lines = append(lines, strings.TrimSpace(line))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if total > len(lines) && len(lines) > 0 {
		lines[len(lines)-1] += fmt.Sprintf(" (y %d más)", total-len(lines))
	}
	return lines, nil
}

func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}

// MigrationReport reports what the one-time import of the legacy files kept.
type MigrationReport struct {
	Legacy      []string `json:"legacy,omitempty"`
	Phase       int      `json:"phase,omitempty"`
	Evidence    int      `json:"evidence"`
	Delegations int      `json:"delegations"`
	Notes       int      `json:"notes"`
	Skipped     int      `json:"skipped"`
	Limitations []string `json:"limitations"`
}

// MigrationReport returns the report stored by the migration, if any.
func (s *Store) MigrationReport() (*MigrationReport, error) {
	if s.corrupt || s.db == nil {
		return nil, nil
	}
	var raw string
	err := s.db.QueryRow("SELECT value FROM meta WHERE key = 'migration_report'").Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, s.fail(err)
	}
	var report MigrationReport
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		return nil, nil
	}
	return &report, nil
}
