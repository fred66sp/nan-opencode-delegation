package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/ledger"
)

func samplePrepare() PrepareInput {
	return PrepareInput{
		Agent:    "general",
		Task:     "tk-1",
		Goal:     "Actualizar los tres documentos",
		Scope:    []string{"docs/runner/PRD.md", "docs/runner/DESIGN.md"},
		Checks:   []string{"grep de los ajustes", "wc -l"},
		Criteria: "Los documentos reflejan el cambio sin ampliar alcance.",
	}
}

func mustPrepare(t *testing.T, s *Store, in PrepareInput) (string, string) {
	t.Helper()
	raw, err := s.Prepare("ses_o", "", in)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	out := outcomeOf(t, raw)
	return out["id"].(string), out["fingerprint"].(string)
}

func mustLaunch(t *testing.T, s *Store, id string) {
	t.Helper()
	if _, err := s.Launch("ses_o", id, "", ""); err != nil {
		t.Fatalf("Launch: %v", err)
	}
}

func mustReturn(t *testing.T, s *Store, id, status, risks string, checks []ReturnCheck) map[string]any {
	t.Helper()
	raw, err := s.Return("ses_o", id, status, risks, "", checks)
	if err != nil {
		t.Fatalf("Return %s: %v", status, err)
	}
	return outcomeOf(t, raw)
}

func TestDeckCycle(t *testing.T) {
	s := openTemp(t)
	id, _ := mustPrepare(t, s, samplePrepare())
	mustLaunch(t, s, id)
	ret := mustReturn(t, s, id, "completed", "", []ReturnCheck{
		{Check: "grep de los ajustes", Observed: "5 ajustes en 3/3 documentos"},
		{Check: "wc -l", Observed: "DESIGN 149, PRD 78"},
	})
	if ret["state"] != ledger.StateReturned || len(ret["pendientes"].([]any)) != 0 {
		t.Fatalf("retorno = %v", ret)
	}
	raw, err := s.CloseDelegation("ses_o", id, "done", "", "", nil)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	closeOut := outcomeOf(t, raw)
	if closeOut["state"] != ledger.StateClosed || closeOut["outcome"] != "done" ||
		len(closeOut["pendientes"].([]any)) != 0 {
		t.Fatalf("cierre = %v", closeOut)
	}
	var state, outcome string
	if err := s.db.QueryRow("SELECT state, COALESCE(outcome,'') FROM delegations WHERE id = ?", id).Scan(&state, &outcome); err != nil {
		t.Fatal(err)
	}
	var returns int
	if err := s.db.QueryRow("SELECT count(*) FROM delegation_returns WHERE delegation_id = ?", id).Scan(&returns); err != nil {
		t.Fatal(err)
	}
	if state != ledger.StateClosed || outcome != "done" || returns != 1 {
		t.Fatalf("fila final = %s/%s, retornos %d", state, outcome, returns)
	}
}

func TestDeckDuplicateOnlyWhileOpen(t *testing.T) {
	s := openTemp(t)
	id, fingerprint := mustPrepare(t, s, samplePrepare())
	_, err := s.Prepare("ses_o", "", samplePrepare())
	var duplicate *DuplicateError
	if !errors.As(err, &duplicate) {
		t.Fatalf("duplicado abierto = %v", err)
	}
	if duplicate.ID != id || duplicate.State != ledger.StatePrepared {
		t.Fatalf("duplicado = %+v, esperaba %s", duplicate, id)
	}

	mustLaunch(t, s, id)
	mustReturn(t, s, id, "completed", "", []ReturnCheck{
		{Check: "grep de los ajustes", Observed: "5 ajustes"},
		{Check: "wc -l", Observed: "149 líneas"},
	})
	_, err = s.Prepare("ses_o", "", samplePrepare())
	if !errors.As(err, &duplicate) || duplicate.State != ledger.StateReturned {
		t.Fatalf("un retorno pendiente debía bloquear: %v", err)
	}
	_, err = s.Launch("ses_o", id, "", "")
	var field *FieldError
	if !errors.As(err, &field) {
		t.Fatalf("un retorno no puede lanzarse: %v", err)
	}
	if _, err := s.CloseDelegation("ses_o", id, "done", "", "", nil); err != nil {
		t.Fatal(err)
	}
	again, againFingerprint := mustPrepare(t, s, samplePrepare())
	if again == id {
		t.Fatalf("el re-encargo reutilizó el id %s", id)
	}
	if againFingerprint != fingerprint {
		t.Fatalf("huella distinta: %s vs %s", againFingerprint, fingerprint)
	}
}

func TestDeckCloseGuards(t *testing.T) {
	s := openTemp(t)
	id, _ := mustPrepare(t, s, samplePrepare())
	mustLaunch(t, s, id)

	_, err := s.CloseDelegation("ses_o", id, "superseded", "", "", nil)
	var guard *GuardError
	if !errors.As(err, &guard) {
		t.Fatalf("cierre desde launched sin retorno = %v", err)
	}
	var state string
	if err := s.db.QueryRow("SELECT state FROM delegations WHERE id = ?", id).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != ledger.StateLaunched {
		t.Fatalf("el cierre denegado mutó el estado: %s", state)
	}
	_, err = s.CloseDelegation("ses_o", id, "superseded", "no existe", "", nil)
	var field *FieldError
	if !errors.As(err, &field) {
		t.Fatalf("contraste corto = %v", err)
	}
	raw, err := s.CloseDelegation("ses_o", id, "superseded",
		"el subagente no existe en OpenCode: la sesión terminó sin lanzarlo", "", nil)
	if err != nil {
		t.Fatalf("cierre con contraste = %v", err)
	}
	if out := outcomeOf(t, raw); out["state"] != ledger.StateClosed || out["outcome"] != "superseded" {
		t.Fatalf("cierre con contraste = %v", out)
	}

	id2, _ := mustPrepare(t, s, PrepareInput{Agent: "general", Goal: "Otro encargo", Scope: []string{},
		Checks: []string{"go test ./..."}, Criteria: "verde"})
	mustLaunch(t, s, id2)
	mustReturn(t, s, id2, "partial", "faltó un fichero", []ReturnCheck{{Check: "go test ./...", Failed: "no ejecutado"}})
	if _, err := s.CloseDelegation("ses_o", id2, "absorbed", "", "", nil); err != nil {
		t.Fatalf("cierre desde returned = %v", err)
	}
}

func TestDeckReturnRejectsShapeAndDerivesPendings(t *testing.T) {
	s := openTemp(t)
	id, _ := mustPrepare(t, s, samplePrepare())
	mustLaunch(t, s, id)

	_, err := s.Return("ses_o", id, "completed", "", "", []ReturnCheck{
		{Check: "grep de los ajustes", Observed: "5 ajustes"},
	})
	var field *FieldError
	if !errors.As(err, &field) {
		t.Fatalf("retorno sin la comprobación esperada = %v", err)
	}
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM delegation_returns WHERE delegation_id = ?", id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("un retorno inválido se registró: %d filas", count)
	}

	partial := mustReturn(t, s, id, "partial", "", []ReturnCheck{
		{Check: "grep de los ajustes", Observed: "2 de 3 documentos"},
		{Check: "wc -l", Failed: "no ejecutado: presupuesto agotado"},
	})
	pendings := partial["pendientes"].([]any)
	if len(pendings) != 1 || !strings.Contains(pendings[0].(string), "wc -l") {
		t.Fatalf("pendientes parciales = %v", pendings)
	}
	classes := partial["checks"].([]any)
	if len(classes) != 2 {
		t.Fatalf("clasificación = %v", classes)
	}
	if classes[0].(map[string]any)["state"] != CheckExecuted || classes[1].(map[string]any)["state"] != CheckFailed {
		t.Fatalf("clasificación = %v", classes)
	}

	corrected := mustReturn(t, s, id, "completed", "", []ReturnCheck{
		{Check: "wc -l", Observed: "DESIGN 149, PRD 78"},
	})
	if corrected["returns"] != float64(2) || len(corrected["pendientes"].([]any)) != 0 {
		t.Fatalf("corrección = %v", corrected)
	}
	rows, err := s.db.Query("SELECT status FROM delegation_returns WHERE delegation_id = ? ORDER BY id", id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var statuses []string
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			t.Fatal(err)
		}
		statuses = append(statuses, status)
	}
	if strings.Join(statuses, ",") != "partial,completed" {
		t.Fatalf("versiones conservadas = %v", statuses)
	}
}

func TestDeckCompletedWithPendingsIsInvalid(t *testing.T) {
	s := openTemp(t)
	id, _ := mustPrepare(t, s, samplePrepare())
	mustLaunch(t, s, id)
	_, err := s.Return("ses_o", id, "completed", "", "", []ReturnCheck{
		{Check: "grep de los ajustes", Observed: "5 ajustes"},
		{Check: "wc -l", Failed: "no ejecutado"},
	})
	var field *FieldError
	if !errors.As(err, &field) || !strings.Contains(field.Reason, "completed") {
		t.Fatalf("completed con fallos = %v", err)
	}
}

func TestDeckLaunchAndReturnIdempotent(t *testing.T) {
	s := openTemp(t)
	id, _ := mustPrepare(t, s, samplePrepare())
	first, err := s.Launch("ses_o", id, "ses_sub", "req-launch")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Launch("ses_o", id, "ses_sub", "req-launch")
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("lanzamiento repetido distinto:\n%s\n%s", first, second)
	}

	checks := []ReturnCheck{
		{Check: "grep de los ajustes", Observed: "5 ajustes"},
		{Check: "wc -l", Observed: "149 líneas"},
	}
	first, err = s.Return("ses_o", id, "completed", "", "req-return", checks)
	if err != nil {
		t.Fatal(err)
	}
	second, err = s.Return("ses_o", id, "completed", "", "req-return", checks)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("retorno repetido distinto:\n%s\n%s", first, second)
	}
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM delegation_returns WHERE delegation_id = ?", id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("el reintento con request_id duplicó el retorno: %d", count)
	}

	again := mustReturn(t, s, id, "completed", "", checks)
	if again["returns"] != float64(2) {
		t.Fatalf("sin request_id, la corrección debía ser v2: %v", again)
	}
}

func TestDeckNoteNeverChangesState(t *testing.T) {
	s := openTemp(t)
	id, _ := mustPrepare(t, s, samplePrepare())
	mustLaunch(t, s, id)
	raw, err := s.Note("ses_o", id, "el subagente sigue trabajando", "ses_sub", "", true)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	if out := outcomeOf(t, raw); out["state"] != ledger.StateLaunched {
		t.Fatalf("la nota cambió el estado: %v", out)
	}
	var subagent, state string
	if err := s.db.QueryRow("SELECT subagent_session, state FROM delegations WHERE id = ?", id).Scan(&subagent, &state); err != nil {
		t.Fatal(err)
	}
	if subagent != "ses_sub" || state != ledger.StateLaunched {
		t.Fatalf("nota = subagente %q, estado %q", subagent, state)
	}
	var observed string
	if err := s.db.QueryRow("SELECT observed FROM delegation_notes WHERE delegation_id = ?", id).Scan(&observed); err != nil {
		t.Fatal(err)
	}
	if observed != "1" {
		t.Fatalf("observed de la nota = %q", observed)
	}
	_, err = s.Note("ses_o", id, "otra identidad", "ses_otro", "", false)
	var field *FieldError
	if !errors.As(err, &field) {
		t.Fatalf("identidad distinta = %v", err)
	}
}

func TestDeckListProjection(t *testing.T) {
	s := openTemp(t)
	orphan, _ := mustPrepare(t, s, samplePrepare())
	launched, _ := mustPrepare(t, s, PrepareInput{Agent: "docs", Goal: "Otro encargo",
		Scope: []string{"docs"}, Checks: []string{"grep"}, Criteria: "hecho"})
	mustLaunch(t, s, launched)

	list := func(filter string) []any {
		raw, err := s.ListProjection(filter)
		if err != nil {
			t.Fatalf("List %s: %v", filter, err)
		}
		return outcomeOf(t, raw)["delegations"].([]any)
	}
	if got := list("orphans"); len(got) != 1 || got[0].(map[string]any)["id"] != orphan {
		t.Fatalf("orphans = %v", got)
	}
	if got := list("open"); len(got) != 2 {
		t.Fatalf("open = %v", got)
	}
	mustReturn(t, s, launched, "partial", "pendiente", []ReturnCheck{{Check: "grep", Failed: "no ejecutado"}})
	if _, err := s.CloseDelegation("ses_o", launched, "superseded", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if got := list("orphans"); len(got) != 1 {
		t.Fatalf("orphans tras cierre = %v", got)
	}
	if got := list("open"); len(got) != 1 {
		t.Fatalf("open tras cierre = %v", got)
	}
	if got := list("all"); len(got) != 2 {
		t.Fatalf("all = %v", got)
	}
}

func TestDeckListWithoutDatabaseCreatesNothing(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	raw, err := s.ListProjection("all")
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomeOf(t, raw)["delegations"].([]any)) != 0 {
		t.Fatalf("proyección sin base = %s", raw)
	}
	if _, err := os.Stat(filepath.Join(root, ".run")); !os.IsNotExist(err) {
		t.Fatalf(".run se creó en una consulta: %v", err)
	}
}
