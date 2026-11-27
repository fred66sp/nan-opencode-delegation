package aifops

import (
	"encoding/json"
	"strings"
	"testing"

	aif "github.com/fred66sp/agent-interaction-forms"

	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/store"
)

func exec(t *testing.T, s *store.Store, request string) (*aif.Response, map[string]any) {
	t.Helper()
	response := New(s, "").Handle([]byte(request))
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("respuesta no es JSON: %v", err)
	}
	return response, doc
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestDiscoveryMatchesCatalogAndHelp(t *testing.T) {
	response, doc := exec(t, newStore(t), `{"aif":"0.2"}`)
	if response.Result != "" || response.Discovery == nil {
		t.Fatalf("no es descubrimiento: %s", mustJSON(response))
	}
	operations, _ := doc["operations"].([]any)
	catalog := Operations(nil)
	if len(operations) != len(catalog) {
		t.Fatalf("descubrimiento %d operaciones, catálogo %d", len(operations), len(catalog))
	}
	help := Help(catalog)
	for i, op := range catalog {
		entry := operations[i].(map[string]any)
		if entry["id"] != op.ID || entry["description"] != op.Description {
			t.Fatalf("operación %d = %v, catálogo %s", i, entry, op.ID)
		}
		if !strings.Contains(help, op.ID) {
			t.Fatalf("la ayuda no nombra %s:\n%s", op.ID, help)
		}
	}
}

func TestNextRequiresEvidenceAndRejectsJSON(t *testing.T) {
	s := newStore(t)
	response, _ := exec(t, s, `{"aif":"0.2","operation":"phase_next","input":{}}`)
	if response.Result != aif.ResultInputNeeded || len(response.Missing) != 1 || response.Missing[0] != "evidence" {
		t.Fatalf("sin evidencia = %s", mustJSON(response))
	}
	response, _ = exec(t, s, `{"aif":"0.2","operation":"phase_next","input":{"evidence":"  "}}`)
	if response.Result != aif.ResultInvalid {
		t.Fatalf("evidencia vacía = %s", mustJSON(response))
	}
	response, _ = exec(t, s, `{"aif":"0.2","operation":"phase_next","input":{"evidence":"{\"a\":1}"}}`)
	if response.Result != aif.ResultInvalid || !strings.Contains(mustJSON(response), "JSON") {
		t.Fatalf("evidencia JSON = %s", mustJSON(response))
	}
}

func TestSkipRejectsNonIntegerAndBackwards(t *testing.T) {
	s := newStore(t)
	response, _ := exec(t, s, `{"aif":"0.2","operation":"phase_skip","input":{"target":"²","reason":"motivo"}}`)
	if response.Result != aif.ResultInvalid {
		t.Fatalf("target no numérico = %s", mustJSON(response))
	}
	response, _ = exec(t, s, `{"aif":"0.2","operation":"phase_skip","input":{"target":2.5,"reason":"motivo"}}`)
	if response.Result != aif.ResultInvalid {
		t.Fatalf("target fraccionario = %s", mustJSON(response))
	}
	response, _ = exec(t, s, `{"aif":"0.2","operation":"phase_skip","input":{"target":1,"reason":"atrás"}}`)
	if response.Result != aif.ResultInvalid || !strings.Contains(mustJSON(response), "no se retrocede") {
		t.Fatalf("salto hacia atrás = %s", mustJSON(response))
	}
	response, doc := exec(t, s, `{"aif":"0.2","operation":"phase_skip","input":{"target":4,"reason":"no aportan"}}`)
	if response.Result != aif.ResultExecuted {
		t.Fatalf("salto válido = %s", mustJSON(response))
	}
	outcome := doc["outcome"].(map[string]any)
	if outcome["phase"] != float64(4) || len(outcome["skipped"].([]any)) != 2 {
		t.Fatalf("outcome del salto = %v", outcome)
	}
}

func TestRequestIDReturnsTheSameResponseWithoutAdvancingTwice(t *testing.T) {
	s := newStore(t)
	request := `{"aif":"0.2","operation":"phase_next","input":{"evidence":"una vez","request_id":"req-1"}}`
	_, first := exec(t, s, request)
	_, second := exec(t, s, request)
	firstRaw, _ := json.Marshal(first)
	secondRaw, _ := json.Marshal(second)
	if string(firstRaw) != string(secondRaw) {
		t.Fatalf("respuestas distintas:\n%s\n%s", firstRaw, secondRaw)
	}
	state, _ := s.SessionPhase("")
	if state.Phase != 2 {
		t.Fatalf("el reintento avanzó otra vez: %+v", state)
	}
}

func TestTerminalPhaseDoesNotAdvance(t *testing.T) {
	s := newStore(t)
	for i := 1; i < 11; i++ {
		response, _ := exec(t, s, `{"aif":"0.2","operation":"phase_next","input":{"evidence":"paso"}}`)
		if response.Result != aif.ResultExecuted {
			t.Fatalf("paso %d = %s", i, mustJSON(response))
		}
	}
	response, doc := exec(t, s, `{"aif":"0.2","operation":"phase_next","input":{"evidence":"cierre"}}`)
	if response.Result != aif.ResultExecuted {
		t.Fatalf("fase 11 = %s", mustJSON(response))
	}
	outcome := doc["outcome"].(map[string]any)
	if outcome["phase"] != float64(11) || outcome["terminal"] != true {
		t.Fatalf("outcome terminal = %v", outcome)
	}
	response, _ = exec(t, s, `{"aif":"0.2","operation":"phase_skip","input":{"target":11,"reason":"x"}}`)
	if response.Result != aif.ResultInvalid {
		t.Fatalf("salto desde fase 11 = %s", mustJSON(response))
	}
}

func TestUnknownFieldAndOperation(t *testing.T) {
	s := newStore(t)
	response, _ := exec(t, s, `{"aif":"0.2","operation":"phase_next","input":{"evidence":"e","extra":1}}`)
	if response.Result != aif.ResultInvalid {
		t.Fatalf("campo desconocido = %s", mustJSON(response))
	}
	response, _ = exec(t, s, `{"aif":"0.2","operation":"deck_unknown","input":{}}`)
	if response.Result != aif.ResultUnknownOperation {
		t.Fatalf("operación desconocida = %s", mustJSON(response))
	}
}

func TestSessionFieldOverridesTheEnvironment(t *testing.T) {
	s := newStore(t)
	exec(t, s, `{"aif":"0.2","operation":"phase_next","input":{"evidence":"sesión explícita","session":"ses_x"}}`)
	state, _ := s.SessionPhase("ses_x")
	if state.Phase != 2 {
		t.Fatalf("sesión explícita = %+v", state)
	}
	other, _ := s.SessionPhase("")
	if other.Exists {
		t.Fatalf("la sesión por defecto no debía tocarse: %+v", other)
	}
}

func mustJSON(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
