package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func writeLegacy(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, ".run")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"phase": 2, "evidence": [{"phase": 1, "text": "memoria leída"}]}`
	if err := os.WriteFile(filepath.Join(dir, "orchestrator-runner.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	lines := `{"ts": "2026-11-17T17:31:50Z", "phase": 2, "task": "tk-1", "fingerprint": "uno", "agent": "general", "status": "completed"}
{"ts": "2026-11-17T17:36:10Z", "phase": 6, "task": "tk-2", "fingerprint": "dos", "agent": "verify", "status": "launched", "session": "ses_x"}
{"ts": "2026-11-17T17:38:20Z", "phase": 6, "task": "tk-3", "fingerprint": "tres", "agent": "docs", "status": "blocked", "note": "sin acceso"}
{"ts": "2026-11-17T17:40:00Z", "phase": 7, "task": "tk-4", "fingerprint": "cuatro", "agent": "general", "status": "completed"}
`
	if err := os.WriteFile(filepath.Join(dir, "delegations.jsonl"), []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
}

func count(t *testing.T, s *Store, query string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(query).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestMigrationImportsOnceAndKeepsBackups(t *testing.T) {
	root := t.TempDir()
	writeLegacy(t, root)
	s, err := Open(root, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// The first mutation imports and then applies its own effect.
	out, err := s.Next("", "avance tras migrar", "")
	if err != nil {
		t.Fatalf("Next con legado: %v", err)
	}
	if outcomeOf(t, out)["phase"] != float64(3) {
		t.Fatalf("la fase importada no se usó: %v", outcomeOf(t, out))
	}
	warnings, _ := outcomeOf(t, out)["warnings"].([]any)
	if len(warnings) == 0 || len(warnings) > 3 {
		t.Fatalf("la transición debía traer los encargos abiertos (≤3): %v", warnings)
	}
	state, _ := s.SessionPhase("")
	if state.Phase != 3 || len(state.Evidence) != 2 {
		t.Fatalf("estado tras migrar = %+v", state)
	}
	if n := count(t, s, "SELECT count(*) FROM delegations"); n != 4 {
		t.Fatalf("delegaciones importadas = %d", n)
	}
	if n := count(t, s, "SELECT count(*) FROM delegation_notes"); n != 1 {
		t.Fatalf("notas importadas = %d", n)
	}
	for _, name := range []string{"orchestrator-runner.json", "delegations.jsonl"} {
		if _, err := os.Stat(filepath.Join(root, ".run", name)); !os.IsNotExist(err) {
			t.Fatalf("%s debía renombrarse: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(root, ".run", name+".migrated")); err != nil {
			t.Fatalf("falta el respaldo %s.migrated: %v", name, err)
		}
	}

	// A later mutation sees the marker and does not duplicate.
	if _, err := s.Next("", "segundo avance", ""); err != nil {
		t.Fatal(err)
	}
	if n := count(t, s, "SELECT count(*) FROM delegations"); n != 4 {
		t.Fatalf("la reinvocación duplicó: %d delegaciones", n)
	}
	if n := count(t, s, "SELECT count(*) FROM phase_evidence"); n != 3 {
		t.Fatalf("evidencia duplicada: %d", n)
	}

	var rows *sql.Rows
	rows, err = s.db.Query("SELECT state, outcome, orchestrator_session FROM delegations ORDER BY created_at")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var states []string
	for rows.Next() {
		var state, session string
		var outcome sql.NullString
		if err := rows.Scan(&state, &outcome, &session); err != nil {
			t.Fatal(err)
		}
		states = append(states, state)
	}
	if len(states) != 4 || states[0] != "closed" || states[1] != "launched" || states[2] != "returned" || states[3] != "closed" {
		t.Fatalf("estados importados = %v", states)
	}

	report, err := s.MigrationReport()
	if err != nil || report == nil {
		t.Fatalf("informe de migración = %v, %v", report, err)
	}
	if report.Delegations != 4 || report.Evidence != 1 || len(report.Limitations) == 0 {
		t.Fatalf("informe = %+v", report)
	}
}

func TestWithoutLegacyNothingIsImported(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Next("ses", "sin legado", ""); err != nil {
		t.Fatal(err)
	}
	report, err := s.MigrationReport()
	if err != nil || report != nil {
		t.Fatalf("informe inesperado: %+v, %v", report, err)
	}
}
