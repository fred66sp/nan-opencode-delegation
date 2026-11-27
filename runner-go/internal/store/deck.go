package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/ledger"
	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/report"
)

// NotFoundError reports an operation on an id that does not exist.
type NotFoundError struct{ ID string }

func (e *NotFoundError) Error() string { return "no existe el encargo " + e.ID }

// DuplicateError reports an open execution with the same fingerprint.
type DuplicateError struct{ ID, State string }

func (e *DuplicateError) Error() string {
	return "ya hay un encargo abierto con la misma huella: " + e.ID + " (" + e.State + ")"
}

// GuardError reports a close that would elude an active execution.
type GuardError struct{ ID, State string }

func (e *GuardError) Error() string {
	return "cierre denegado: el encargo " + e.ID + " está en " + e.State +
		" sin retorno retenido; declara observed en el propio cierre con el contraste de OpenCode (mínimo 20 caracteres)"
}

// PrepareInput is the content of one assignment; the fingerprint covers it all.
type PrepareInput struct {
	Agent    string
	Task     string
	Goal     string
	Scope    []string
	Checks   []string
	Criteria string
}

// ReturnCheck is one check of a return: executed with its result or failed with
// the reason. The two members are mutually exclusive.
type ReturnCheck struct {
	Check    string `json:"check"`
	Observed string `json:"observed,omitempty"`
	Failed   string `json:"failed,omitempty"`
}

// Classification is the state of one expected check after the returns.
type Classification struct {
	Check    string `json:"check"`
	State    string `json:"state"`
	Observed string `json:"observed,omitempty"`
}

// Classification states of an expected check.
const (
	CheckExecuted = "ejecutada"
	CheckFailed   = "fallida"
	CheckNotRun   = "no_ejecutada"
)

// minObserved is the declared contrast the close guard requires.
const minObserved = 20

type preparedOutcome struct {
	ID          string `json:"id"`
	Fingerprint string `json:"fingerprint"`
	State       string `json:"state"`
}

type launchOutcome struct {
	State               string   `json:"state"`
	OrchestratorSession string   `json:"orchestrator_session,omitempty"`
	SubagentSession     string   `json:"subagent_session,omitempty"`
	Warnings            []string `json:"warnings,omitempty"`
}

type returnOutcome struct {
	State      string           `json:"state"`
	Returns    int              `json:"returns"`
	Checks     []Classification `json:"checks"`
	Pendientes []string         `json:"pendientes"`
	Warnings   []string         `json:"warnings,omitempty"`
}

type closeOutcome struct {
	State      string   `json:"state"`
	Outcome    string   `json:"outcome"`
	Pendientes []string `json:"pendientes"`
	Warnings   []string `json:"warnings,omitempty"`
}

type noteOutcome struct {
	State string `json:"state"`
}

type listOutcome struct {
	Delegations []ledger.Delegation `json:"delegations"`
}

// row is the full delegation as persisted, including the columns the AIF
// outcome does not project.
type row struct {
	ledger.Delegation
	Scope    string
	Checks   string
	Criteria string
	Observed string
}

func loadDelegation(tx *sql.Tx, id string) (*row, error) {
	var r row
	err := tx.QueryRow(`SELECT id, fingerprint, agent, task, goal, scope, checks, criteria, state,
		COALESCE(outcome,''), COALESCE(prepared_phase,0), orchestrator_session, subagent_session, observed,
		created_at, updated_at FROM delegations WHERE id = ?`, id).Scan(
		&r.ID, &r.Fingerprint, &r.Agent, &r.Task, &r.Goal, &r.Scope, &r.Checks, &r.Criteria,
		&r.State, &r.Outcome, &r.PreparedPhase, &r.OrchestratorSession, &r.SubagentSession,
		&r.Observed, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &NotFoundError{ID: id}
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (r *row) expected() []string {
	var checks []string
	if err := json.Unmarshal([]byte(r.Checks), &checks); err != nil {
		return nil
	}
	return checks
}

// Prepare registers one execution and denies an open duplicate of the same
// fingerprint. Closed executions do not block: re-assigning is a new execution.
func (s *Store) Prepare(session, requestID string, in PrepareInput) (json.RawMessage, error) {
	fingerprint := ledger.Fingerprint(in.Agent, in.Task, in.Goal, in.Scope, in.Checks, in.Criteria)
	if in.Scope == nil {
		in.Scope = []string{}
	}
	if in.Checks == nil {
		in.Checks = []string{}
	}
	return s.Mutate(session, requestID, "deck_prepare", func(tx *sql.Tx) (json.RawMessage, error) {
		var existingID, existingState string
		err := tx.QueryRow(`SELECT id, state FROM delegations
			WHERE fingerprint = ? AND state IN ('prepared','launched','returned')`, fingerprint).Scan(&existingID, &existingState)
		if err == nil {
			return nil, &DuplicateError{ID: existingID, State: existingState}
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		id := ledger.NewID()
		scope, _ := json.Marshal(in.Scope)
		checks, _ := json.Marshal(in.Checks)
		current, err := sessionPhase(tx, session)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`INSERT INTO delegations(id, fingerprint, agent, task, goal, scope, checks,
			criteria, state, prepared_phase, orchestrator_session, created_at, updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			id, fingerprint, in.Agent, in.Task, in.Goal, string(scope), string(checks), in.Criteria,
			ledger.StatePrepared, current, session, now(), now()); err != nil {
			return nil, err
		}
		return json.Marshal(preparedOutcome{ID: id, Fingerprint: fingerprint, State: ledger.StatePrepared})
	})
}

// Launch confirms the already effective launch. Repeating it on a launched
// execution re-confirms it; it never moves a returned execution.
func (s *Store) Launch(session, id, subagent, requestID string) (json.RawMessage, error) {
	return s.Mutate(session, requestID, "deck_launch", func(tx *sql.Tx) (json.RawMessage, error) {
		r, err := loadDelegation(tx, id)
		if err != nil {
			return nil, err
		}
		switch r.State {
		case ledger.StateReturned:
			return nil, &FieldError{Field: "id", Reason: "el encargo ya tiene retorno (returned): no se puede lanzar"}
		case ledger.StateClosed:
			return nil, &FieldError{Field: "id", Reason: "el encargo está cerrado"}
		}
		if r.OrchestratorSession != "" && session != "" && r.OrchestratorSession != session {
			return nil, &FieldError{Field: "id", Reason: "la sesión del orquestador no cambia: registrada " + r.OrchestratorSession}
		}
		if r.SubagentSession != "" && subagent != "" && r.SubagentSession != subagent {
			return nil, &FieldError{Field: "subagent_session", Reason: "la identidad del subagente no cambia: registrada " + r.SubagentSession}
		}
		orchestrator, bound := r.OrchestratorSession, r.SubagentSession
		if orchestrator == "" {
			orchestrator = session
		}
		if bound == "" {
			bound = subagent
		}
		if r.State == ledger.StatePrepared || orchestrator != r.OrchestratorSession || bound != r.SubagentSession {
			if _, err := tx.Exec(`UPDATE delegations SET state = 'launched', orchestrator_session = ?,
				subagent_session = ?, updated_at = ? WHERE id = ?`, orchestrator, bound, now(), id); err != nil {
				return nil, err
			}
		}
		out := launchOutcome{State: ledger.StateLaunched, OrchestratorSession: orchestrator, SubagentSession: bound}
		if out.Warnings, err = openWarnings(tx, 3); err != nil {
			return nil, err
		}
		return json.Marshal(out)
	})
}

// returnRow is one retained version of the return.
type returnRow struct {
	Status string
	Checks []ReturnCheck
}

func loadReturns(tx *sql.Tx, id string) ([]returnRow, error) {
	rows, err := tx.Query(`SELECT status, checks FROM delegation_returns
		WHERE delegation_id = ? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []returnRow{}
	for rows.Next() {
		var status, raw string
		if err := rows.Scan(&status, &raw); err != nil {
			return nil, err
		}
		var checks []ReturnCheck
		if err := json.Unmarshal([]byte(raw), &checks); err != nil {
			return nil, err
		}
		out = append(out, returnRow{Status: status, Checks: checks})
	}
	return out, rows.Err()
}

// Return validates the form of the report, retains every version and answers
// with the pendings the software can determine. A correction keeps the evidence
// of the previous versions: only the expected checks without a previous result
// must appear again.
func (s *Store) Return(session, id, status, risks, requestID string, submitted []ReturnCheck) (json.RawMessage, error) {
	return s.Mutate(session, requestID, "deck_return", func(tx *sql.Tx) (json.RawMessage, error) {
		r, err := loadDelegation(tx, id)
		if err != nil {
			return nil, err
		}
		switch r.State {
		case ledger.StatePrepared:
			return nil, &FieldError{Field: "id", Reason: "el encargo no está lanzado: estado prepared"}
		case ledger.StateClosed:
			return nil, &FieldError{Field: "id", Reason: "el encargo está cerrado"}
		}
		if status == "blocked" && strings.TrimSpace(risks) == "" {
			return nil, &FieldError{Field: "status", Reason: "status blocked exige el motivo del bloqueo en risks"}
		}
		prior, err := loadReturns(tx, id)
		if err != nil {
			return nil, err
		}
		expected := r.expected()
		remaining := uncovered(expected, prior)
		if result := report.Validate(reportText(status, submitted, risks), remaining); !result.OK {
			return nil, &FieldError{Field: "checks", Reason: strings.Join(result.Violations, "; ")}
		}
		class, pendings := classify(expected, prior, submitted)
		if status == "completed" && len(pendings) > 0 {
			return nil, &FieldError{Field: "checks", Reason: "status completed con pendientes: " + strings.Join(pendings, "; ")}
		}
		raw, err := json.Marshal(submitted)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`INSERT INTO delegation_returns(delegation_id, status, checks, risks, created_at)
			VALUES(?,?,?,?,?)`, id, status, string(raw), risks, now()); err != nil {
			return nil, err
		}
		if _, err := tx.Exec("UPDATE delegations SET state = 'returned', updated_at = ? WHERE id = ?", now(), id); err != nil {
			return nil, err
		}
		out := returnOutcome{State: ledger.StateReturned, Returns: len(prior) + 1, Checks: class, Pendientes: pendings}
		if out.Warnings, err = openWarnings(tx, 3); err != nil {
			return nil, err
		}
		return json.Marshal(out)
	})
}

// Close ends the execution. From prepared or launched it is denied unless this
// same invocation declares the observed contrast; from returned it is allowed.
func (s *Store) CloseDelegation(session, id, outcome, observed, requestID string, pendings []string) (json.RawMessage, error) {
	return s.Mutate(session, requestID, "deck_close", func(tx *sql.Tx) (json.RawMessage, error) {
		r, err := loadDelegation(tx, id)
		if err != nil {
			return nil, err
		}
		if r.State == ledger.StateClosed {
			return nil, &FieldError{Field: "id", Reason: "el encargo ya está cerrado; el cierre no reabre"}
		}
		if r.State != ledger.StateReturned {
			contrast := strings.TrimSpace(observed)
			if contrast == "" {
				return nil, &GuardError{ID: id, State: r.State}
			}
			if utf8.RuneCountInString(contrast) < minObserved {
				return nil, &FieldError{Field: "observed", Reason: "el contraste declarado necesita al menos 20 caracteres"}
			}
		}
		if len(pendings) > 0 {
			if _, err := tx.Exec(`INSERT INTO delegation_notes(delegation_id, text, created_at) VALUES(?,?,?)`,
				id, "pendientes al cerrar: "+strings.Join(pendings, "; "), now()); err != nil {
				return nil, err
			}
		}
		if _, err := tx.Exec(`UPDATE delegations SET state = 'closed', outcome = ?, observed = ?,
			updated_at = ? WHERE id = ?`, outcome, observed, now(), id); err != nil {
			return nil, err
		}
		prior, err := loadReturns(tx, id)
		if err != nil {
			return nil, err
		}
		_, derived := classify(r.expected(), prior, nil)
		if len(pendings) > 0 {
			derived = append([]string{}, pendings...)
		}
		out := closeOutcome{State: ledger.StateClosed, Outcome: outcome, Pendientes: derived}
		if out.Warnings, err = openWarnings(tx, 3); err != nil {
			return nil, err
		}
		return json.Marshal(out)
	})
}

// Note records tracking or declared conciliation without ever changing the
// state. It binds the subagent identity when it is still unknown.
func (s *Store) Note(session, id, text, subagent, requestID string, observed bool) (json.RawMessage, error) {
	return s.Mutate(session, requestID, "deck_note", func(tx *sql.Tx) (json.RawMessage, error) {
		r, err := loadDelegation(tx, id)
		if err != nil {
			return nil, err
		}
		if r.SubagentSession != "" && subagent != "" && r.SubagentSession != subagent {
			return nil, &FieldError{Field: "subagent_session", Reason: "la identidad del subagente no cambia: registrada " + r.SubagentSession}
		}
		flag := ""
		if observed {
			flag = "1"
		}
		if _, err := tx.Exec(`INSERT INTO delegation_notes(delegation_id, text, observed, created_at)
			VALUES(?,?,?,?)`, id, text, flag, now()); err != nil {
			return nil, err
		}
		if r.SubagentSession == "" && subagent != "" {
			if _, err := tx.Exec("UPDATE delegations SET subagent_session = ?, updated_at = ? WHERE id = ?",
				subagent, now(), id); err != nil {
				return nil, err
			}
		}
		return json.Marshal(noteOutcome{State: r.State})
	})
}

// ListProjection projects the executions of the project. It is read-only: with
// no database it answers empty without creating anything.
func (s *Store) ListProjection(filter string) (json.RawMessage, error) {
	if s.corrupt {
		return nil, ErrStateCorrupt
	}
	out := listOutcome{Delegations: []ledger.Delegation{}}
	if s.db == nil {
		return json.Marshal(out)
	}
	where := "state IN ('prepared','launched','returned')"
	switch filter {
	case "orphans":
		where = "state = 'prepared'"
	case "all":
		where = "1 = 1"
	}
	rows, err := s.db.Query(`SELECT id, fingerprint, agent, task, goal, state, COALESCE(outcome,''),
		COALESCE(prepared_phase,0), orchestrator_session, subagent_session, created_at, updated_at
		FROM delegations WHERE ` + where + ` ORDER BY created_at, id`)
	if err != nil {
		return nil, s.fail(err)
	}
	defer rows.Close()
	list, err := scanDelegations(rows)
	if err != nil {
		return nil, err
	}
	if list != nil {
		out.Delegations = list
	}
	return json.Marshal(out)
}

// uncovered returns the expected checks that no previous version reported.
func uncovered(expected []string, prior []returnRow) []string {
	covered := map[string]bool{}
	for _, ret := range prior {
		for _, check := range ret.Checks {
			for _, want := range expected {
				if report.SameCheck(want, check.Check) {
					covered[want] = true
				}
			}
		}
	}
	var out []string
	for _, want := range expected {
		if !covered[want] {
			out = append(out, want)
		}
	}
	return out
}

// classify replays the retained versions and the new one so the latest result
// per expected check decides. Only what the software can determine is pending.
func classify(expected []string, prior []returnRow, submitted []ReturnCheck) ([]Classification, []string) {
	latest := map[string]ReturnCheck{}
	bind := func(checks []ReturnCheck) {
		for _, check := range checks {
			for _, want := range expected {
				if report.SameCheck(want, check.Check) {
					latest[want] = check
				}
			}
		}
	}
	for _, ret := range prior {
		bind(ret.Checks)
	}
	bind(submitted)
	class := make([]Classification, 0, len(expected))
	pendings := []string{}
	for _, want := range expected {
		check, found := latest[want]
		switch {
		case !found:
			class = append(class, Classification{Check: want, State: CheckNotRun})
			pendings = append(pendings, want+" sin ejecutar")
		case check.Failed != "":
			class = append(class, Classification{Check: want, State: CheckFailed, Observed: check.Failed})
			pendings = append(pendings, want+" falló: "+check.Failed)
		default:
			class = append(class, Classification{Check: want, State: CheckExecuted, Observed: check.Observed})
		}
	}
	return class, pendings
}

// reportText rebuilds the textual report the checker parses, so one set of
// rules validates both the structured return and a markdown report.
func reportText(status string, checks []ReturnCheck, risks string) string {
	var b strings.Builder
	b.WriteString("status: " + status + "\n")
	b.WriteString("validation\n")
	for _, check := range checks {
		result := check.Observed
		if check.Failed != "" {
			result = "no ejecutado — " + check.Failed
		}
		b.WriteString(check.Check + ": " + result + "\n")
	}
	if risks != "" {
		b.WriteString("\n" + risks + "\n")
	}
	return b.String()
}
