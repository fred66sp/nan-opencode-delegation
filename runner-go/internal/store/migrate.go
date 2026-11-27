package store

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/ledger"
	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/phase"
)

const (
	legacyStatePath  = ".run/orchestrator-runner.json"
	legacyLedgerPath = ".run/delegations.jsonl"
)

var migrationLimitations = []string{
	"el ledger legado no guarda subagent_session: queda vacío",
	"el ledger legado no guarda el historial de retornos: solo su último estado",
	"session falta en la mayoría de las líneas: se importan sin sesión del orquestador",
	"el fingerprint legado es prosa del encargo, no la huella sha256: la deduplicación nueva no lo iguala",
	"goal, scope, checks y criteria no son reconstruibles de las filas legadas",
	"el estado legado no identifica sesión: se importa en la sesión del entorno o en la vacía",
}

// migrateLegacy imports the legacy state and ledger exactly once, in the same
// transaction as the mutation that triggers it. It returns the paths to rename
// after the commit, so an interruption repeats the import without duplicates.
func (s *Store) migrateLegacy(tx *sql.Tx, session string) ([]string, error) {
	var marker string
	err := tx.QueryRow("SELECT value FROM meta WHERE key = 'migrated_legacy'").Scan(&marker)
	if err == nil {
		return nil, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	statePath := filepath.Join(s.Root, legacyStatePath)
	ledgerPath := filepath.Join(s.Root, legacyLedgerPath)
	var present []string
	for _, path := range []string{statePath, ledgerPath} {
		if _, err := os.Stat(path); err == nil {
			present = append(present, path)
		}
	}
	if len(present) == 0 {
		return nil, nil
	}

	report := MigrationReport{Limitations: migrationLimitations, Legacy: present}
	if err := importLegacyState(tx, statePath, session, &report); err != nil {
		return nil, err
	}
	if err := importLegacyLedger(tx, ledgerPath, &report); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec("INSERT INTO meta(key, value) VALUES('migrated_legacy', ?)", now()); err != nil {
		return nil, err
	}
	if _, err := tx.Exec("INSERT OR REPLACE INTO meta(key, value) VALUES('migration_report', ?)", string(raw)); err != nil {
		return nil, err
	}
	return present, nil
}

type legacyState struct {
	Phase    int              `json:"phase"`
	Evidence []phase.Evidence `json:"evidence"`
}

func importLegacyState(tx *sql.Tx, path, session string, report *MigrationReport) error {
	data, err := os.ReadFile(path)
	if err != nil {
		report.Skipped++
		return nil
	}
	var state legacyState
	if err := json.Unmarshal(data, &state); err != nil || state.Phase < 1 || state.Phase > phase.Count {
		report.Skipped++
		return nil
	}
	res, err := tx.Exec(`INSERT INTO sessions(session, phase, updated_at) VALUES(?,?,?)
		ON CONFLICT(session) DO NOTHING`, session, state.Phase, now())
	if err != nil {
		return err
	}
	inserted, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if inserted == 0 {
		// A newer state already exists for this session: keep it, only report.
		if err := tx.QueryRow("SELECT phase FROM sessions WHERE session = ?", session).Scan(&report.Phase); err != nil {
			return err
		}
		report.Skipped++
		return nil
	}
	report.Phase = state.Phase
	for _, entry := range state.Evidence {
		if entry.Phase < 1 || entry.Phase > phase.Count || strings.TrimSpace(entry.Text) == "" {
			report.Skipped++
			continue
		}
		var skip any
		if entry.Skip > entry.Phase+1 && entry.Skip <= phase.Count {
			skip = entry.Skip
		}
		if _, err := tx.Exec("INSERT INTO phase_evidence(session, phase, text, skip_to, created_at) VALUES(?,?,?,?,?)",
			session, entry.Phase, entry.Text, skip, now()); err != nil {
			return err
		}
		report.Evidence++
	}
	return nil
}

type legacyRow struct {
	TS          string `json:"ts"`
	Phase       int    `json:"phase"`
	Task        string `json:"task"`
	Fingerprint string `json:"fingerprint"`
	Agent       string `json:"agent"`
	Status      string `json:"status"`
	Session     string `json:"session"`
	Note        string `json:"note"`
}

func importLegacyLedger(tx *sql.Tx, path string, report *MigrationReport) error {
	data, err := os.ReadFile(path)
	if err != nil {
		report.Skipped++
		return nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var row legacyRow
		if err := json.Unmarshal([]byte(line), &row); err != nil ||
			strings.TrimSpace(row.TS) == "" || strings.TrimSpace(row.Agent) == "" ||
			strings.TrimSpace(row.Fingerprint) == "" || row.Phase < 1 || row.Phase > phase.Count {
			report.Skipped++
			continue
		}
		state, outcome := legacyLifecycle(row.Status)
		if state == "" {
			report.Skipped++
			continue
		}
		var outcomeValue any
		if outcome != "" {
			outcomeValue = outcome
		}
		id := ledger.NewID()
		res, err := tx.Exec(`INSERT INTO delegations(
			id, fingerprint, agent, task, goal, scope, checks, criteria, state, outcome,
			prepared_phase, orchestrator_session, subagent_session, observed, created_at, updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT DO NOTHING`,
			id, row.Fingerprint, row.Agent, row.Task, "", "[]", "[]", "", state, outcomeValue,
			row.Phase, row.Session, "", "", row.TS, row.TS)
		if err != nil {
			return err
		}
		inserted, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if inserted == 0 {
			report.Skipped++
			continue
		}
		report.Delegations++
		if row.Note != "" {
			if _, err := tx.Exec("INSERT INTO delegation_notes(delegation_id, text, created_at) VALUES(?,?,?)",
				id, row.Note, row.TS); err != nil {
				return err
			}
			report.Notes++
		}
	}
	return nil
}

// legacyLifecycle maps the legacy status onto the new cycle. The legacy ledger
// only knew launched, completed and blocked; a returned execution appeared as
// blocked when the subagent did not complete.
func legacyLifecycle(status string) (state, outcome string) {
	switch status {
	case "launched":
		return ledger.StateLaunched, ""
	case "completed":
		return ledger.StateClosed, ledger.OutcomeDone
	case "blocked", "partial":
		return ledger.StateReturned, ""
	}
	return "", ""
}
