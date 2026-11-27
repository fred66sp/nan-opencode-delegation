package aifops

import (
	"encoding/json"
	"errors"
	"strings"

	aif "github.com/fred66sp/agent-interaction-forms"

	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/store"
)

// deckOperations is the delegation half of the catalog: the minimal cycle by
// need, not CRUD. Each description carries its usage rule on the first line.
func deckOperations(o *Ops) []*aif.Operation {
	return []*aif.Operation{
		{
			ID:          "deck_prepare",
			Description: "Registra un encargo calculando su huella y deniega duplicados abiertos; úsala antes de lanzar el subagente, nunca después.",
			Fields: []aif.Field{
				{Name: "agent", Required: true, Type: "string", Description: "Agente destinatario del encargo."},
				{Name: "goal", Required: true, Type: "string", Description: "Objetivo concreto del encargo, sin resumir."},
				{Name: "scope", Required: true, Type: "array", Description: "Ficheros o áreas de los que el encargo es propietario."},
				{Name: "checks", Required: true, Type: "array", Description: "Comprobaciones esperadas, en orden: la referencia del retorno."},
				{Name: "criteria", Required: true, Type: "string", Description: "Criterio de aceptación del encargo."},
				{Name: "task", Type: "string", Description: "Tarea del tablero a la que sirve, si existe."},
				requestIDField,
				sessionField,
			},
			Execute: o.prepare,
		},
		{
			ID:          "deck_launch",
			Description: "Confirma el lanzamiento ya efectivo del subagente y ata su sesión; úsala después de la acción de OpenCode, nunca antes.",
			Fields: []aif.Field{
				{Name: "id", Required: true, Type: "string", Description: "Identificador de la ejecución devuelto por deck_prepare."},
				{Name: "subagent_session", Type: "string", Description: "SessionID del subagente en OpenCode, si ya se conoce al confirmar."},
				sessionField,
				requestIDField,
			},
			Execute: o.launch,
		},
		{
			ID:          "deck_return",
			Description: "Registra el retorno, valida su forma y devuelve los pendientes en una sola invocación; úsala cuando el subagente termine.",
			Fields: []aif.Field{
				{Name: "id", Required: true, Type: "string", Description: "Identificador de la ejecución."},
				{Name: "status", Required: true, Type: "string", Allowed: []string{"completed", "partial", "blocked"}, Description: "Veredicto del subagente sobre su trabajo."},
				{Name: "checks", Required: true, Type: "array", Description: "Una entrada por comprobación: check y observed, o check y failed con el motivo."},
				{Name: "risks", Type: "string", Description: "Riesgos, límites y lo no verificado; obligatorio como motivo cuando status es blocked."},
				requestIDField,
				sessionField,
			},
			Execute: o.deckReturn,
		},
		{
			ID:          "deck_close",
			Description: "Cierra formalmente un encargo valorado; úsala tras el retorno, o declara observed en el propio cierre si no lo hay.",
			Fields: []aif.Field{
				{Name: "id", Required: true, Type: "string", Description: "Identificador de la ejecución."},
				{Name: "outcome", Required: true, Type: "string", Allowed: []string{"done", "superseded", "absorbed"}, Description: "Decisión del cierre: aceptado, sustituido o absorbido."},
				{Name: "observed", Type: "string", Description: "Contraste declarado del estado real en OpenCode (mínimo 20 caracteres); no se juzga, se registra."},
				{Name: "pendings", Type: "array", Description: "Pendientes finales que el cierre deja declarados."},
				requestIDField,
				sessionField,
			},
			Execute: o.close,
		},
		{
			ID:          "deck_note",
			Description: "Añade seguimiento o conciliación declarada sin cambiar el estado; úsala para atar subagent_session o registrar lo observado.",
			Fields: []aif.Field{
				{Name: "id", Required: true, Type: "string", Description: "Identificador de la ejecución."},
				{Name: "text", Required: true, Type: "string", Description: "Qué se anota; prosa, sin JSON."},
				{Name: "observed", Type: "boolean", Description: "Marca que el agente declara haber observado el hecho en OpenCode; es un aserto, no una verificación del runner."},
				{Name: "subagent_session", Type: "string", Description: "SessionID del subagente en OpenCode, si aún no estaba atado."},
				requestIDField,
			},
			Execute: o.note,
		},
		{
			ID:          "deck_list",
			Description: "Proyecta encargos abiertos, preparados sin lanzar o todos; úsala para saber qué queda sin conciliar, sin efectos.",
			Fields: []aif.Field{
				{Name: "state", Type: "string", Allowed: []string{"open", "orphans", "all"}, Description: "Proyección: abiertos (defecto), preparados sin lanzar, o todos."},
			},
			Execute: o.list,
		},
	}
}

func (o *Ops) prepare(input map[string]json.RawMessage) (json.RawMessage, error) {
	scope, err := texts(input, "scope")
	if err != nil {
		return nil, aif.InvalidError{Fields: map[string]string{"scope": err.Error()}}
	}
	checks, err := texts(input, "checks")
	if err != nil {
		return nil, aif.InvalidError{Fields: map[string]string{"checks": err.Error()}}
	}
	goal := text(input, "goal")
	if strings.TrimSpace(goal) == "" {
		return nil, aif.InvalidError{Fields: map[string]string{"goal": "el objetivo no puede estar vacío"}}
	}
	criteria := text(input, "criteria")
	if strings.TrimSpace(criteria) == "" {
		return nil, aif.InvalidError{Fields: map[string]string{"criteria": "el criterio de aceptación no puede estar vacío"}}
	}
	out, err := o.store.Prepare(session(input, o.session), text(input, "request_id"), store.PrepareInput{
		Agent:    text(input, "agent"),
		Task:     text(input, "task"),
		Goal:     goal,
		Scope:    scope,
		Checks:   checks,
		Criteria: criteria,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return out, nil
}

func (o *Ops) launch(input map[string]json.RawMessage) (json.RawMessage, error) {
	out, err := o.store.Launch(session(input, o.session), text(input, "id"),
		text(input, "subagent_session"), text(input, "request_id"))
	if err != nil {
		return nil, mapError(err)
	}
	return out, nil
}

func (o *Ops) deckReturn(input map[string]json.RawMessage) (json.RawMessage, error) {
	checks, err := returnChecks(input)
	if err != nil {
		return nil, aif.InvalidError{Fields: map[string]string{"checks": err.Error()}}
	}
	out, err := o.store.Return(session(input, o.session), text(input, "id"), text(input, "status"),
		text(input, "risks"), text(input, "request_id"), checks)
	if err != nil {
		return nil, mapError(err)
	}
	return out, nil
}

func (o *Ops) close(input map[string]json.RawMessage) (json.RawMessage, error) {
	pendings, err := texts(input, "pendings")
	if err != nil {
		return nil, aif.InvalidError{Fields: map[string]string{"pendings": err.Error()}}
	}
	out, err := o.store.CloseDelegation(session(input, o.session), text(input, "id"), text(input, "outcome"),
		text(input, "observed"), text(input, "request_id"), pendings)
	if err != nil {
		return nil, mapError(err)
	}
	return out, nil
}

func (o *Ops) note(input map[string]json.RawMessage) (json.RawMessage, error) {
	note := text(input, "text")
	if strings.TrimSpace(note) == "" {
		return nil, aif.InvalidError{Fields: map[string]string{"text": "la nota no puede estar vacía"}}
	}
	var observed bool
	if raw, ok := input["observed"]; ok {
		_ = json.Unmarshal(raw, &observed)
	}
	out, err := o.store.Note(session(input, o.session), text(input, "id"), note,
		text(input, "subagent_session"), text(input, "request_id"), observed)
	if err != nil {
		return nil, mapError(err)
	}
	return out, nil
}

func (o *Ops) list(input map[string]json.RawMessage) (json.RawMessage, error) {
	filter := text(input, "state")
	if filter == "" {
		filter = "open"
	}
	out, err := o.store.ListProjection(filter)
	if err != nil {
		return nil, mapError(err)
	}
	return out, nil
}

// texts decodes a required-or-optional list of strings. A nil input answers an
// empty list so the fingerprint and the database never see null.
func texts(input map[string]json.RawMessage, name string) ([]string, error) {
	raw, ok := input[name]
	if !ok {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, errors.New("se espera una lista de cadenas")
	}
	if values == nil {
		values = []string{}
	}
	return values, nil
}

func returnChecks(input map[string]json.RawMessage) ([]store.ReturnCheck, error) {
	raw, ok := input["checks"]
	if !ok {
		return []store.ReturnCheck{}, nil
	}
	var checks []store.ReturnCheck
	if err := json.Unmarshal(raw, &checks); err != nil {
		return nil, errors.New("se espera una lista de objetos {check, observed, failed}")
	}
	var problems []string
	for _, check := range checks {
		switch {
		case strings.TrimSpace(check.Check) == "":
			problems = append(problems, "una comprobación no cita el comando")
		case strings.TrimSpace(check.Observed) != "" && strings.TrimSpace(check.Failed) != "":
			problems = append(problems, check.Check+": observed y failed son excluyentes")
		case strings.TrimSpace(check.Observed) == "" && strings.TrimSpace(check.Failed) == "":
			problems = append(problems, check.Check+": falta observed o failed")
		}
	}
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "; "))
	}
	return checks, nil
}
