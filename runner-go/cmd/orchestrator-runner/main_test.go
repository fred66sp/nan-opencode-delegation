package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func execute(t *testing.T, args []string, stdin string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, strings.NewReader(stdin), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestVersion(t *testing.T) {
	code, stdout, _ := execute(t, []string{"--version"}, "")
	if code != 0 || !strings.Contains(stdout, "dev") {
		t.Fatalf("--version = %d, %q", code, stdout)
	}
}

func TestHelpListsEveryCatalogOperation(t *testing.T) {
	_, stdout, _ := execute(t, []string{"--help"}, "")
	for _, id := range []string{"phase_next", "phase_skip", "phase_reset", "recover"} {
		if !strings.Contains(stdout, id) {
			t.Fatalf("la ayuda no nombra %s:\n%s", id, stdout)
		}
	}
}

func TestJSONWithoutDatabaseIsEmptyAndCreatesNothing(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("OPENCODE_SESSION_ID", "")
	code, stdout, stderr := execute(t, []string{"--json"}, "")
	if code != 0 {
		t.Fatalf("--json = %d, %s", code, stderr)
	}
	var payload struct {
		Version int `json:"version"`
		Phases  []struct {
			Number int `json:"number"`
		} `json:"phases"`
		State struct {
			Exists   bool `json:"exists"`
			Phase    int  `json:"phase"`
			Valid    bool `json:"valid"`
			Evidence []struct {
				Phase int `json:"phase"`
			} `json:"evidence"`
		} `json:"state"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("salida no JSON: %v", err)
	}
	if payload.Version != 1 || len(payload.Phases) != 11 {
		t.Fatalf("contrato = %s", stdout)
	}
	if payload.State.Exists || payload.State.Phase != 1 || !payload.State.Valid || len(payload.State.Evidence) != 0 {
		t.Fatalf("estado vacío = %s", stdout)
	}
	if _, err := os.Stat(filepath.Join(root, ".run")); !os.IsNotExist(err) {
		t.Fatalf("--json creó .run: %v", err)
	}
}

func TestAIFDiscoveryCreatesNothingAndTransitionsPersist(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("OPENCODE_SESSION_ID", "ses_test")
	code, stdout, stderr := execute(t, []string{"aif"}, `{"aif":"0.2"}`)
	if code != 0 || !strings.Contains(stdout, "phase_next") {
		t.Fatalf("descubrimiento = %d, %s, %s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".run")); !os.IsNotExist(err) {
		t.Fatalf("el descubrimiento creó .run: %v", err)
	}

	code, stdout, _ = execute(t, []string{"aif"}, `{"aif":"0.2","operation":"phase_next","input":{"evidence":"leí memoria y tablero"}}`)
	if code != 0 || !strings.Contains(stdout, `"phase":2`) {
		t.Fatalf("phase_next = %d, %s", code, stdout)
	}
	code, stdout, _ = execute(t, []string{"--json"}, "")
	if code != 0 {
		t.Fatalf("--json = %d", code)
	}
	if !strings.Contains(stdout, `"exists": true`) || !strings.Contains(stdout, `"phase": 2`) {
		t.Fatalf("el estado no persistió: %s", stdout)
	}

	code, stdout, _ = execute(t, []string{"aif"}, `{"aif":"0.2","operation":"phase_next","input":{"evidence":""}}`)
	if code != 2 || !strings.Contains(stdout, `"invalid"`) {
		t.Fatalf("evidencia vacía = %d, %s", code, stdout)
	}
	code, stdout, _ = execute(t, []string{"aif"}, `{}`)
	if code != 3 || !strings.Contains(stdout, `"malformed_request"`) {
		t.Fatalf("petición malformada = %d, %s", code, stdout)
	}
}

func TestJSONWithoutSessionIsEmptyEvenWithState(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("OPENCODE_SESSION_ID", "")
	code, _, _ := execute(t, []string{"aif"}, `{"aif":"0.2","operation":"phase_next","input":{"evidence":"sin sesión"}}`)
	if code != 0 {
		t.Fatalf("phase_next = %d", code)
	}
	code, stdout, _ := execute(t, []string{"--json"}, "")
	if code != 0 || !strings.Contains(stdout, `"exists": false`) || !strings.Contains(stdout, `"phase": 1`) {
		t.Fatalf("estado sin sesión = %d, %s", code, stdout)
	}
}

func TestJSONLatestAnswersTheMostRecentSession(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("OPENCODE_SESSION_ID", "")

	// Sin sesiones con fase registrada, --latest es el mismo estado vacío del contrato.
	code, stdout, _ := execute(t, []string{"--json", "--latest"}, "")
	if code != 0 || !strings.Contains(stdout, `"exists": false`) || strings.Contains(stdout, `"session"`) {
		t.Fatalf("latest sin sesiones = %d, %s", code, stdout)
	}

	code, _, _ = execute(t, []string{"aif"}, `{"aif":"0.2","operation":"phase_next","input":{"session":"ses_a","evidence":"una"}}`)
	if code != 0 {
		t.Fatalf("phase_next ses_a = %d", code)
	}
	// updated_at has second precision: without a separation the order clause
	// does not discriminate chronology and the test would pass with the
	// tiebreaker alone.
	time.Sleep(1100 * time.Millisecond)
	code, _, _ = execute(t, []string{"aif"}, `{"aif":"0.2","operation":"phase_next","input":{"session":"ses_b","evidence":"dos"}}`)
	if code != 0 {
		t.Fatalf("phase_next ses_b = %d", code)
	}

	// La sesión que más recientemente avanzó se nombra en el estado: el panel
	// no presenta una fase sin decir de qué sesión viene.
	code, stdout, _ = execute(t, []string{"--json", "--latest"}, "")
	if code != 0 || !strings.Contains(stdout, `"exists": true`) || !strings.Contains(stdout, `"phase": 2`) {
		t.Fatalf("latest con sesiones = %d, %s", code, stdout)
	}
	if !strings.Contains(stdout, `"session": "ses_b"`) {
		t.Fatalf("latest no nombra la sesión más reciente: %s", stdout)
	}

	// El contrato sin --latest no cambia: sin entorno de sesión sigue vacío.
	code, stdout, _ = execute(t, []string{"--json"}, "")
	if code != 0 || !strings.Contains(stdout, `"exists": false`) {
		t.Fatalf("el contrato de --json cambió: %d, %s", code, stdout)
	}
}

func TestJSONLatestAnswersCorruptState(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("OPENCODE_SESSION_ID", "")
	if err := os.MkdirAll(filepath.Join(root, ".run"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".run", "orchestrator.db"), []byte("esto no es una base de datos"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Una base dañada responde el estado corrupto, no uno aparentemente nuevo.
	code, stdout, _ := execute(t, []string{"--json", "--latest"}, "")
	if code != 0 || !strings.Contains(stdout, `"exists": true`) || !strings.Contains(stdout, `"valid": false`) || strings.Contains(stdout, `"session"`) {
		t.Fatalf("latest con base dañada = %d, %s", code, stdout)
	}
}

func TestJSONRejectsUnknownArguments(t *testing.T) {
	t.Setenv("OPENCODE_SESSION_ID", "")
	code, _, stderr := execute(t, []string{"--json", "--pepe"}, "")
	if code != 1 || !strings.Contains(stderr, "--pepe") {
		t.Fatalf("argumento desconocido = %d, %s", code, stderr)
	}
}

func TestRecoverThenExport(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll(filepath.Join(root, ".run"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".run", "orchestrator.db"), []byte("basura"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := execute(t, []string{"recover"}, "")
	if code != 0 || !strings.Contains(stdout, "archived") || !strings.Contains(stdout, "rescued") {
		t.Fatalf("recover = %d, %s, %s", code, stdout, stderr)
	}
	code, _, stderr = execute(t, []string{"--json"}, "")
	if code != 0 || strings.Contains(stderr, "error") {
		t.Fatalf("--json tras recover = %d, %s", code, stderr)
	}

	code, stdout, stderr = execute(t, []string{"export-legacy"}, "")
	if code != 0 || !strings.Contains(stdout, "state_path") {
		t.Fatalf("export-legacy = %d, %s, %s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".run", "orchestrator-runner.json")); err != nil {
		t.Fatalf("falta el estado exportado: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".run", "delegations.jsonl")); err != nil {
		t.Fatalf("falta el ledger exportado: %v", err)
	}
}
