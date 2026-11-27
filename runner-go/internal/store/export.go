package store

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/ledger"
	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/phase"
)

// ExportReport declares what export-legacy wrote and what the legacy format
// cannot carry back.
type ExportReport struct {
	StatePath   string   `json:"state_path"`
	LedgerPath  string   `json:"ledger_path"`
	Delegations int      `json:"delegations"`
	Backups     []string `json:"backups,omitempty"`
	Limitations []string `json:"limitations"`
}

var exportLimitations = []string{
	"un retorno pendiente se exporta como blocked: el ledger legado no tiene estado returned",
	"un encargo prepared se exporta como launched: el formato legado no tiene estado prepared",
	"el resultado del cierre (done/superseded/absorbed) se exporta como completed: el formato legado no lo distingue",
	"solo viaja el estado final de cada ejecución: retornos y notas intermedias no caben en el formato",
	"la sesión del subagente no tiene columna en el formato legado",
}

type exportLine struct {
	TS          string  `json:"ts"`
	Phase       int     `json:"phase"`
	Task        *string `json:"task"`
	Fingerprint string  `json:"fingerprint"`
	Agent       string  `json:"agent"`
	Status      string  `json:"status"`
	Session     string  `json:"session,omitempty"`
	Note        string  `json:"note,omitempty"`
}

// ExportLegacy regenerates the two Python artifacts from the database so the
// rollback can continue where the Go runner left. Existing files are copied to
// a dated backup first and never deleted.
func (s *Store) ExportLegacy(session string) (*ExportReport, error) {
	if s.corrupt {
		return nil, ErrStateCorrupt
	}
	state, err := s.SessionPhase(session)
	if err != nil {
		return nil, err
	}
	report := &ExportReport{
		StatePath:   filepath.Join(s.Root, legacyStatePath),
		LedgerPath:  filepath.Join(s.Root, legacyLedgerPath),
		Limitations: exportLimitations,
	}
	var backups []string
	for _, path := range []string{report.StatePath, report.LedgerPath} {
		if _, err := os.Stat(path); err == nil {
			backup := path + ".pre-export-" + timestamp()
			if data, err := os.ReadFile(path); err == nil {
				if err := os.WriteFile(backup, data, 0o644); err == nil {
					backups = append(backups, backup)
				}
			}
		}
	}
	report.Backups = backups

	evidence := state.Evidence
	if evidence == nil {
		evidence = []phase.Evidence{}
	}
	stateJSON, err := json.Marshal(struct {
		Phase    int              `json:"phase"`
		Evidence []phase.Evidence `json:"evidence"`
	}{Phase: state.Phase, Evidence: evidence})
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(report.StatePath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(report.StatePath, append(stateJSON, '\n'), 0o644); err != nil {
		return nil, err
	}

	lines, err := s.exportLines()
	if err != nil {
		return nil, err
	}
	var builder strings.Builder
	for _, line := range lines {
		raw, err := json.Marshal(line)
		if err != nil {
			return nil, err
		}
		builder.Write(raw)
		builder.WriteByte('\n')
	}
	if err := os.WriteFile(report.LedgerPath, []byte(builder.String()), 0o644); err != nil {
		return nil, err
	}
	report.Delegations = len(lines)
	return report, nil
}

func (s *Store) exportLines() ([]exportLine, error) {
	if s.db == nil {
		return nil, nil
	}
	rows, err := s.db.Query(`SELECT d.fingerprint, d.agent, d.task, d.state, COALESCE(d.outcome,''),
		COALESCE(d.prepared_phase,1), d.orchestrator_session, d.created_at,
		COALESCE((SELECT text FROM delegation_notes n WHERE n.delegation_id = d.id ORDER BY n.id DESC LIMIT 1),'')
		FROM delegations d ORDER BY d.created_at, d.id`)
	if err != nil {
		return nil, s.fail(err)
	}
	defer rows.Close()
	var out []exportLine
	for rows.Next() {
		var line exportLine
		var state, outcome string
		var task sql.NullString
		if err := rows.Scan(&line.Fingerprint, &line.Agent, &task, &state, &outcome,
			&line.Phase, &line.Session, &line.TS, &line.Note); err != nil {
			return nil, err
		}
		if task.Valid && strings.TrimSpace(task.String) != "" {
			line.Task = &task.String
		}
		switch state {
		case ledger.StateClosed:
			line.Status = "completed"
		case ledger.StateReturned:
			line.Status = "blocked"
		default:
			line.Status = "launched"
		}
		out = append(out, line)
	}
	return out, rows.Err()
}

func timestamp() string { return time.Now().UTC().Format("20060102T150405") }

// RecoverReport declares what a damaged database kept, archived and lost.
type RecoverReport struct {
	Corrupt  bool           `json:"corrupt"`
	Database string         `json:"database"`
	Archived string         `json:"archived,omitempty"`
	Rescued  map[string]int `json:"rescued"`
	Lost     []string       `json:"lost,omitempty"`
	Phase    int            `json:"phase"`
	Session  string         `json:"session"`
	Note     string         `json:"note,omitempty"`
}

var rescueColumns = []struct {
	table string
	cols  []string
}{
	{"meta", []string{"key", "value"}},
	{"sessions", []string{"session", "phase", "updated_at"}},
	{"phase_evidence", []string{"id", "session", "phase", "text", "skip_to", "created_at"}},
	{"delegations", []string{"id", "fingerprint", "agent", "task", "goal", "scope", "checks", "criteria", "state", "outcome", "prepared_phase", "orchestrator_session", "subagent_session", "observed", "created_at", "updated_at"}},
	{"delegation_returns", []string{"id", "delegation_id", "status", "checks", "risks", "created_at"}},
	{"delegation_notes", []string{"id", "delegation_id", "text", "observed", "created_at"}},
	{"requests", []string{"request_id", "operation", "response", "created_at"}},
}

// Recover archives a damaged database, rescues every readable table and rebuilds
// the session phase. It never resets silently: the report says what was kept.
func (s *Store) Recover(session string) (*RecoverReport, error) {
	report := &RecoverReport{Corrupt: s.corrupt, Database: s.Path, Rescued: map[string]int{}, Session: session, Phase: 1}
	if !s.corrupt {
		if _, err := os.Stat(s.Path); err != nil {
			report.Note = "no hay base de datos: nada que recuperar"
		} else {
			report.Note = "la base no está dañada: nada que recuperar"
		}
		return report, nil
	}
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
	report.Archived = s.Path + ".corrupt-" + timestamp()
	if err := os.Rename(s.Path, report.Archived); err != nil {
		return nil, err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Rename(s.Path+suffix, report.Archived+suffix)
	}

	fresh, err := Open(s.Root, true)
	if err != nil {
		return nil, err
	}
	defer fresh.Close()
	if err := fresh.ensure(); err != nil {
		return nil, err
	}

	for _, table := range rescueColumns {
		kept, ok := rescueTable(report.Archived, fresh.db, table.table, table.cols)
		if ok {
			report.Rescued[table.table] = kept
		} else {
			report.Lost = append(report.Lost, table.table)
		}
	}

	if _, err := fresh.db.Exec(`INSERT INTO sessions(session, phase, updated_at) VALUES(?,?,?)
		ON CONFLICT(session) DO NOTHING`, session, 1, now()); err != nil {
		return nil, err
	}
	var rebuilt int
	if err := fresh.db.QueryRow("SELECT phase FROM sessions WHERE session = ?", session).Scan(&rebuilt); err == nil {
		report.Phase = rebuilt
	}
	return report, nil
}

// rescueTable copies the rows of one table that are still readable. A table
// whose header or pages are gone is declared lost; readable rows are kept.
func rescueTable(source string, target *sql.DB, table string, cols []string) (int, bool) {
	src, err := sql.Open("sqlite", "file:"+source+"?mode=ro&_pragma=busy_timeout(1000)")
	if err != nil {
		return 0, false
	}
	defer src.Close()
	columnList := strings.Join(cols, ", ")
	rows, err := src.Query("SELECT " + columnList + " FROM " + table)
	if err != nil {
		return 0, false
	}
	defer rows.Close()

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",")
	insert := "INSERT OR IGNORE INTO " + table + "(" + columnList + ") VALUES(" + placeholders + ")"
	kept := 0
	for rows.Next() {
		values := make([]any, len(cols))
		pointers := make([]any, len(cols))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return kept, true
		}
		if _, err := target.Exec(insert, values...); err == nil {
			kept++
		}
	}
	return kept, true
}
