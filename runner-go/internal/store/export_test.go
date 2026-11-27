package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Python ledger validator rejects an empty task: it must travel as null.
func TestExportWritesNullTaskAndRoundTripsLedger(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".run")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "orchestrator-runner.json"),
		[]byte(`{"phase": 2, "evidence": [{"phase": 1, "text": "memoria leída"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	lines := `{"ts": "2026-11-17T17:31:50Z", "phase": 2, "task": null, "fingerprint": "sin tarea", "agent": "general", "status": "launched"}
{"ts": "2026-11-17T17:36:10Z", "phase": 2, "task": "tk-1", "fingerprint": "con tarea", "agent": "verify", "status": "blocked", "note": "sin acceso"}
`
	if err := os.WriteFile(filepath.Join(dir, "delegations.jsonl"), []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Open(root, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Next("", "avance tras migrar", ""); err != nil {
		t.Fatal(err)
	}
	report, err := s.ExportLegacy("")
	if err != nil {
		t.Fatal(err)
	}
	if report.Delegations != 2 {
		t.Fatalf("delegaciones exportadas = %d", report.Delegations)
	}

	data, err := os.ReadFile(filepath.Join(root, ".run", "delegations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("línea no JSON: %v", err)
		}
		rows = append(rows, row)
	}
	if len(rows) != 2 {
		t.Fatalf("líneas exportadas = %d", len(rows))
	}
	if value, ok := rows[0]["task"]; !ok || value != nil {
		t.Fatalf("task sin referencia debía ser null: %v", rows[0]["task"])
	}
	if rows[1]["task"] != "tk-1" {
		t.Fatalf("task perdida: %v", rows[1]["task"])
	}
	if rows[1]["status"] != "blocked" || rows[1]["note"] != "sin acceso" {
		t.Fatalf("estado o nota perdidos: %v", rows[1])
	}
	if len(report.Limitations) == 0 {
		t.Fatal("faltan las limitaciones declaradas")
	}
}
