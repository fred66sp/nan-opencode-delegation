package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/phase"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir(), true)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func outcomeOf(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("outcome no es JSON: %v (%s)", err, raw)
	}
	return out
}

func TestNextAdvancesAndRegistersEvidence(t *testing.T) {
	s := openTemp(t)
	for i, text := range []string{"leí la memoria", "clasificado", "tarea tk-1"} {
		out, err := s.Next("ses_a", text, "")
		if err != nil {
			t.Fatalf("Next %d: %v", i, err)
		}
		if got := outcomeOf(t, out)["phase"]; got != float64(i+2) {
			t.Fatalf("fase tras %d avances = %v, quiero %d", i+1, got, i+2)
		}
	}
	state, err := s.SessionPhase("ses_a")
	if err != nil {
		t.Fatal(err)
	}
	if !state.Exists || state.Phase != 4 || len(state.Evidence) != 3 {
		t.Fatalf("estado = %+v", state)
	}
	if state.Evidence[0].Text != "leí la memoria" || state.Evidence[0].Phase != 1 {
		t.Fatalf("primera evidencia = %+v", state.Evidence[0])
	}
}

func TestPhaseElevenIsTerminal(t *testing.T) {
	s := openTemp(t)
	for i := 1; i < phase.Count; i++ {
		if _, err := s.Next("ses", "evidencia "+string(rune('a'+i)), ""); err != nil {
			t.Fatalf("Next %d: %v", i, err)
		}
	}
	out, err := s.Next("ses", "cierre", "")
	if err != nil {
		t.Fatal(err)
	}
	decoded := outcomeOf(t, out)
	if decoded["phase"] != float64(phase.Count) || decoded["terminal"] != true {
		t.Fatalf("fase 11 terminal = %v", decoded)
	}
	state, _ := s.SessionPhase("ses")
	if state.Phase != phase.Count || len(state.Evidence) != phase.Count-1 {
		t.Fatalf("el terminal no debe registrar evidencia: %+v", state)
	}
}

func TestSkipForwardOnlyAndOmitsPhases(t *testing.T) {
	s := openTemp(t)
	out, err := s.Skip("ses", 4, "las fases 2 y 3 no aportan", "")
	if err != nil {
		t.Fatal(err)
	}
	decoded := outcomeOf(t, out)
	skipped, _ := decoded["skipped"].([]any)
	if decoded["phase"] != float64(4) || len(skipped) != 2 {
		t.Fatalf("salto = %v", decoded)
	}

	for _, target := range []int{1, 4, 12} {
		_, err := s.Skip("ses", target, "motivo", "")
		var field *FieldError
		if !errors.As(err, &field) {
			t.Fatalf("salto a %d debía ser invalid, fue %v", target, err)
		}
	}
	// Fase 11: no hay transición.
	for i := 4; i < phase.Count; i++ {
		if _, err := s.Next("ses", "ev", ""); err != nil {
			t.Fatal(err)
		}
	}
	_, err = s.Skip("ses", 11, "ya está", "")
	var field *FieldError
	if !errors.As(err, &field) {
		t.Fatalf("salto desde la fase 11 debía ser invalid, fue %v", err)
	}
}

func TestSessionsDoNotTouchEachOther(t *testing.T) {
	s := openTemp(t)
	if _, err := s.Next("ses_a", "a1", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Next("ses_b", "b1", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Next("ses_a", "a2", ""); err != nil {
		t.Fatal(err)
	}
	a, _ := s.SessionPhase("ses_a")
	b, _ := s.SessionPhase("ses_b")
	if a.Phase != 3 || len(a.Evidence) != 2 {
		t.Fatalf("sesión A = %+v", a)
	}
	if b.Phase != 2 || len(b.Evidence) != 1 || b.Evidence[0].Text != "b1" {
		t.Fatalf("sesión B = %+v", b)
	}
	if _, err := s.Reset("ses_a", "trabajo nuevo", ""); err != nil {
		t.Fatal(err)
	}
	// Reset nunca toca las delegaciones del proyecto.
	if _, err := s.db.Exec(`INSERT INTO delegations(id, fingerprint, agent, state, created_at, updated_at)
		VALUES('abc123abc123', 'huella', 'general', 'launched', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reset("ses_a", "otra vez", ""); err != nil {
		t.Fatal(err)
	}
	var delegationCount int
	if err := s.db.QueryRow("SELECT count(*) FROM delegations").Scan(&delegationCount); err != nil || delegationCount != 1 {
		t.Fatalf("reset tocó las delegaciones: %d, %v", delegationCount, err)
	}
	a, _ = s.SessionPhase("ses_a")
	b, _ = s.SessionPhase("ses_b")
	if a.Exists || a.Phase != 1 || len(a.Evidence) != 0 {
		t.Fatalf("reset de A = %+v", a)
	}
	if b.Phase != 2 || len(b.Evidence) != 1 {
		t.Fatalf("el reset de A tocó B = %+v", b)
	}
}

func TestRequestIDReplaysExactly(t *testing.T) {
	s := openTemp(t)
	first, err := s.Next("ses", "una vez", "req-1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Next("ses", "reintento perdido", "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("respuesta distinta:\n%s\n%s", first, second)
	}
	state, _ := s.SessionPhase("ses")
	if state.Phase != 2 || len(state.Evidence) != 1 || state.Evidence[0].Text != "una vez" {
		t.Fatalf("el reintento tuvo efectos: %+v", state)
	}
	if _, err := s.Reset("ses", "otra operación", "req-1"); !errors.Is(err, ErrRequestIDReused) {
		t.Fatalf("request_id reutilizado = %v", err)
	}
}

func TestReadOnlyDoesNotCreateTheDatabase(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	state, err := s.SessionPhase("ses")
	if err != nil {
		t.Fatal(err)
	}
	if state.Exists || state.Phase != 1 || !state.Valid {
		t.Fatalf("estado sin base = %+v", state)
	}
	if _, err := os.Stat(filepath.Join(root, ".run")); !os.IsNotExist(err) {
		t.Fatalf(".run se creó en una consulta: %v", err)
	}
}
