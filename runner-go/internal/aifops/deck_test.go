package aifops

import (
	"strings"
	"testing"

	aif "github.com/fred66sp/agent-interaction-forms"

	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/store"
)

const prepareRequest = `{"aif":"0.2","operation":"deck_prepare","input":{
	"agent":"general","task":"tk-9","goal":"Actualizar los documentos",
	"scope":["docs/runner/PRD.md","docs/runner/DESIGN.md"],
	"checks":["grep de los ajustes","wc -l"],"criteria":"Los documentos reflejan el cambio."}}`

func prepareAIF(t *testing.T, s *store.Store) (string, string) {
	t.Helper()
	response, doc := exec(t, s, prepareRequest)
	if response.Result != aif.ResultExecuted {
		t.Fatalf("prepare = %s", mustJSON(response))
	}
	outcome := doc["outcome"].(map[string]any)
	return outcome["id"].(string), outcome["fingerprint"].(string)
}

func TestDiscoveryListsTenOperations(t *testing.T) {
	response, doc := exec(t, newStore(t), `{"aif":"0.2"}`)
	if response.Discovery == nil {
		t.Fatalf("no es descubrimiento: %s", mustJSON(response))
	}
	operations := doc["operations"].([]any)
	if len(operations) != 10 {
		t.Fatalf("descubrimiento con %d operaciones, se esperan 10", len(operations))
	}
	wanted := []string{"deck_prepare", "deck_launch", "deck_return", "deck_close", "deck_note", "deck_list"}
	found := map[string]bool{}
	for _, entry := range operations {
		op := entry.(map[string]any)
		if strings.TrimSpace(op["description"].(string)) == "" {
			t.Fatalf("operación sin regla de uso: %v", op)
		}
		found[op["id"].(string)] = true
	}
	for _, id := range wanted {
		if !found[id] {
			t.Fatalf("el descubrimiento no lista %s: %v", id, found)
		}
	}
}

func TestDeckAIFCycle(t *testing.T) {
	s := newStore(t)
	id, _ := prepareAIF(t, s)

	response, doc := exec(t, s, `{"aif":"0.2","operation":"deck_launch","input":{"id":"`+id+`","subagent_session":"ses_sub"}}`)
	if response.Result != aif.ResultExecuted {
		t.Fatalf("launch = %s", mustJSON(response))
	}
	launch := doc["outcome"].(map[string]any)
	if launch["state"] != "launched" || launch["subagent_session"] != "ses_sub" {
		t.Fatalf("launch = %v", launch)
	}

	response, doc = exec(t, s, `{"aif":"0.2","operation":"deck_return","input":{"id":"`+id+`","status":"completed",
		"checks":[{"check":"grep de los ajustes","observed":"5 ajustes en 3/3 documentos"},
		{"check":"wc -l","observed":"DESIGN 149, PRD 78"}]}}`)
	if response.Result != aif.ResultExecuted {
		t.Fatalf("return = %s", mustJSON(response))
	}
	ret := doc["outcome"].(map[string]any)
	if len(ret["pendientes"].([]any)) != 0 || ret["returns"] != float64(1) {
		t.Fatalf("return = %v", ret)
	}

	response, doc = exec(t, s, `{"aif":"0.2","operation":"deck_close","input":{"id":"`+id+`","outcome":"done"}}`)
	if response.Result != aif.ResultExecuted {
		t.Fatalf("close = %s", mustJSON(response))
	}
	if doc["outcome"].(map[string]any)["state"] != "closed" {
		t.Fatalf("close = %v", doc["outcome"])
	}

	response, doc = exec(t, s, `{"aif":"0.2","operation":"deck_list","input":{"state":"all"}}`)
	if response.Result != aif.ResultExecuted {
		t.Fatalf("list = %s", mustJSON(response))
	}
	if len(doc["outcome"].(map[string]any)["delegations"].([]any)) != 1 {
		t.Fatalf("list = %v", doc["outcome"])
	}
}

func TestDeckAIFDuplicateIsDeniedWithTheExistingID(t *testing.T) {
	s := newStore(t)
	id, _ := prepareAIF(t, s)
	response, _ := exec(t, s, prepareRequest)
	if response.Result != aif.ResultDenied {
		t.Fatalf("duplicado = %s", mustJSON(response))
	}
	if !strings.Contains(response.Reason, "existing="+id) || !strings.Contains(response.Reason, "state=prepared") {
		t.Fatalf("denied sin el id existente: %q", response.Reason)
	}
}

func TestDeckAIFMissingAndUnknownMembers(t *testing.T) {
	s := newStore(t)
	response, doc := exec(t, s, `{"aif":"0.2","operation":"deck_launch","input":{}}`)
	if response.Result != aif.ResultInputNeeded || doc["missing"].([]any)[0] != "id" {
		t.Fatalf("falta id = %s", mustJSON(response))
	}
	response, _ = exec(t, s, `{"aif":"0.2","operation":"deck_prepare","input":{
		"agent":"general","goal":"objetivo","scope":[],"checks":[],"criteria":"hecho","extra":1}}`)
	if response.Result != aif.ResultInvalid || !strings.Contains(mustJSON(response), "miembro desconocido") {
		t.Fatalf("miembro desconocido = %s", mustJSON(response))
	}
	response, _ = exec(t, s, `{"aif":"0.2","operation":"deck_return","input":{"id":"0123456789ab","status":"partial","status":"blocked","checks":[]}}`)
	if response.Result != aif.ResultMalformedRequest {
		t.Fatalf("claves duplicadas = %s", mustJSON(response))
	}
}

func TestDeckAIFCloseGuardAndObserved(t *testing.T) {
	s := newStore(t)
	id, _ := prepareAIF(t, s)
	exec(t, s, `{"aif":"0.2","operation":"deck_launch","input":{"id":"`+id+`"}}`)

	response, _ := exec(t, s, `{"aif":"0.2","operation":"deck_close","input":{"id":"`+id+`","outcome":"superseded"}}`)
	if response.Result != aif.ResultDenied {
		t.Fatalf("cierre desde launched sin retorno = %s", mustJSON(response))
	}
	response, _ = exec(t, s, `{"aif":"0.2","operation":"deck_close","input":{"id":"`+id+`","outcome":"superseded",
		"observed":"el subagente no existe en OpenCode: la sesión terminó sin lanzarlo"}}`)
	if response.Result != aif.ResultExecuted {
		t.Fatalf("cierre con observed = %s", mustJSON(response))
	}
}

func TestDeckAIFNotFoundAndWrongState(t *testing.T) {
	s := newStore(t)
	response, _ := exec(t, s, `{"aif":"0.2","operation":"deck_note","input":{"id":"ffffffffffff","text":"nota"}}`)
	if response.Result != aif.ResultFailed || response.Code != "E_NOT_FOUND" {
		t.Fatalf("id inexistente = %s", mustJSON(response))
	}
	id, _ := prepareAIF(t, s)
	response, _ = exec(t, s, `{"aif":"0.2","operation":"deck_return","input":{"id":"`+id+`","status":"partial",
		"checks":[{"check":"grep de los ajustes","failed":"no ejecutado"},{"check":"wc -l","failed":"no ejecutado"}]}}`)
	if response.Result != aif.ResultInvalid || !strings.Contains(mustJSON(response), "no está lanzado") {
		t.Fatalf("retorno desde prepared = %s", mustJSON(response))
	}
}

func TestDeckAIFOrphansAndPhaseWarnings(t *testing.T) {
	s := newStore(t)
	id, _ := prepareAIF(t, s)
	response, doc := exec(t, s, `{"aif":"0.2","operation":"deck_list","input":{"state":"orphans"}}`)
	if response.Result != aif.ResultExecuted || len(doc["outcome"].(map[string]any)["delegations"].([]any)) != 1 {
		t.Fatalf("orphans = %s", mustJSON(response))
	}
	response, doc = exec(t, s, `{"aif":"0.2","operation":"phase_next","input":{"evidence":"clasificado"}}`)
	if response.Result != aif.ResultExecuted {
		t.Fatalf("phase_next = %s", mustJSON(response))
	}
	warnings, _ := doc["outcome"].(map[string]any)["warnings"].([]any)
	joined := mustJSON(warnings)
	if !strings.Contains(joined, id) {
		t.Fatalf("la transición no avisa del encargo abierto: %s", mustJSON(response))
	}
}

func TestDeckAIFNoteKeepsState(t *testing.T) {
	s := newStore(t)
	id, _ := prepareAIF(t, s)
	exec(t, s, `{"aif":"0.2","operation":"deck_launch","input":{"id":"`+id+`"}}`)
	response, doc := exec(t, s, `{"aif":"0.2","operation":"deck_note","input":{"id":"`+id+`",
		"text":"el subagente sigue trabajando","observed":true,"subagent_session":"ses_sub"}}`)
	if response.Result != aif.ResultExecuted || doc["outcome"].(map[string]any)["state"] != "launched" {
		t.Fatalf("nota = %s", mustJSON(response))
	}
}
