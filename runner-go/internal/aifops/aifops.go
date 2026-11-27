// Package aifops declares the runner operations served through AIF: phase
// transitions and the explicit recovery. The catalog is the single source of
// the operations and of the generated help.
package aifops

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	aif "github.com/fred66sp/agent-interaction-forms"

	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/phase"
	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/store"
)

// Ops binds the catalog to one store and one orchestrator session.
type Ops struct {
	store   *store.Store
	session string
}

// New builds the AIF executor over the given store. The session defaults to
// OPENCODE_SESSION_ID (or empty) and each request may override it.
func New(s *store.Store, session string) *aif.Executor {
	return &aif.Executor{Operations: Operations(&Ops{store: s, session: session})}
}

var requestIDField = aif.Field{
	Name:        "request_id",
	Type:        "string",
	Description: "Clave opcional de idempotencia: reintentar con la misma devuelve la primera respuesta sin efectos nuevos.",
}

var sessionField = aif.Field{
	Name:        "session",
	Type:        "string",
	Description: "Sesión del orquestador; por defecto OPENCODE_SESSION_ID.",
}

// Operations is the catalog: the four process operations and the six deck
// operations.
func Operations(o *Ops) []*aif.Operation {
	return append(phaseOperations(o), deckOperations(o)...)
}

// phaseOperations is the process half of the catalog.
func phaseOperations(o *Ops) []*aif.Operation {
	return []*aif.Operation{
		{
			ID:          "phase_next",
			Description: "Avanza una fase registrando su evidencia; úsala al terminar la fase actual con la evidencia que se espera de ella.",
			Fields: []aif.Field{
				{Name: "evidence", Required: true, Type: "string", Description: "Qué se hizo y se comprobó para dejar la fase; texto en prosa, no JSON."},
				requestIDField,
				sessionField,
			},
			Execute: o.next,
		},
		{
			ID:          "phase_skip",
			Description: "Salta a una fase posterior con motivo; úsala solo si las fases intermedias no aportan y el motivo lo justifica.",
			Fields: []aif.Field{
				{Name: "target", Required: true, Type: "number", Description: "Número de la fase posterior a la que saltar (2..11); omite al menos una."},
				{Name: "reason", Required: true, Type: "string", Description: "Por qué se omite el tramo; queda en el historial de la sesión."},
				requestIDField,
				sessionField,
			},
			Execute: o.skip,
		},
		{
			ID:          "phase_reset",
			Description: "Reinicia fase y evidencia de esta sesión sin tocar las delegaciones del proyecto; úsala al abrir trabajo nuevo o cerrar la sesión.",
			Fields: []aif.Field{
				{Name: "reason", Required: true, Type: "string", Description: "Por qué se reinicia la fase; no se guarda en el historial."},
				requestIDField,
				sessionField,
			},
			Execute: o.reset,
		},
		{
			ID:          "recover",
			Description: "Archiva una base dañada, rescata sus tablas legibles y reconstruye la fase de la sesión; úsala solo tras un fallo E_STATE_CORRUPT.",
			Fields:      []aif.Field{sessionField},
			Execute:     o.recover,
		},
	}
}

func (o *Ops) next(input map[string]json.RawMessage) (json.RawMessage, error) {
	evidence := text(input, "evidence")
	if problem := phase.EvidenceProblem(evidence); problem != nil {
		return nil, aif.InvalidError{Fields: map[string]string{problem.Field: problem.Reason}}
	}
	out, err := o.store.Next(session(input, o.session), evidence, text(input, "request_id"))
	if err != nil {
		return nil, mapError(err)
	}
	return out, nil
}

func (o *Ops) skip(input map[string]json.RawMessage) (json.RawMessage, error) {
	reason := text(input, "reason")
	if problem := phase.ReasonProblem("reason", reason); problem != nil {
		return nil, aif.InvalidError{Fields: map[string]string{problem.Field: problem.Reason}}
	}
	target, err := integer(input, "target")
	if err != nil {
		return nil, aif.InvalidError{Fields: map[string]string{"target": err.Error()}}
	}
	out, err := o.store.Skip(session(input, o.session), target, reason, text(input, "request_id"))
	if err != nil {
		return nil, mapError(err)
	}
	return out, nil
}

func (o *Ops) reset(input map[string]json.RawMessage) (json.RawMessage, error) {
	reason := text(input, "reason")
	if problem := phase.ReasonProblem("reason", reason); problem != nil {
		return nil, aif.InvalidError{Fields: map[string]string{problem.Field: problem.Reason}}
	}
	out, err := o.store.Reset(session(input, o.session), reason, text(input, "request_id"))
	if err != nil {
		return nil, mapError(err)
	}
	return out, nil
}

func (o *Ops) recover(input map[string]json.RawMessage) (json.RawMessage, error) {
	report, err := o.store.Recover(session(input, o.session))
	if err != nil {
		return nil, mapError(err)
	}
	return json.Marshal(report)
}

func mapError(err error) error {
	var field *store.FieldError
	var notFound *store.NotFoundError
	var duplicate *store.DuplicateError
	var guard *store.GuardError
	switch {
	case errors.As(err, &field):
		return aif.InvalidError{Fields: map[string]string{field.Field: field.Reason}}
	case errors.As(err, &notFound):
		return aif.DomainError{Message: notFound.Error(), Code: "E_NOT_FOUND"}
	case errors.As(err, &duplicate):
		return aif.DeniedError{Reason: "duplicado abierto: existing=" + duplicate.ID + " state=" + duplicate.State}
	case errors.As(err, &guard):
		return aif.DeniedError{Reason: guard.Error()}
	case errors.Is(err, store.ErrStateCorrupt):
		return aif.DomainError{Message: "estado corrupto: la mutación no se aplicó; ejecuta recover", Code: "E_STATE_CORRUPT"}
	case errors.Is(err, store.ErrRequestIDReused):
		return aif.DomainError{Message: err.Error(), Code: "E_REQUEST_ID_REUSED"}
	default:
		return aif.DomainError{Message: err.Error()}
	}
}

func text(input map[string]json.RawMessage, name string) string {
	var value string
	if raw, ok := input[name]; ok {
		_ = json.Unmarshal(raw, &value)
	}
	return value
}

func session(input map[string]json.RawMessage, fallback string) string {
	if value := text(input, "session"); value != "" {
		return value
	}
	return fallback
}

func integer(input map[string]json.RawMessage, name string) (int, error) {
	raw, ok := input[name]
	if !ok {
		return 0, errors.New("falta la fase de destino")
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return 0, errors.New("la fase de destino no es un número entero")
	}
	number, ok := value.(json.Number)
	if !ok {
		return 0, errors.New("la fase de destino no es un número entero")
	}
	n, err := strconv.Atoi(number.String())
	if err != nil {
		return 0, errors.New("la fase de destino no es un número entero")
	}
	return n, nil
}

// Help renders the conventional usage and the operations of the catalog.
func Help(operations []*aif.Operation) string {
	var b strings.Builder
	b.WriteString("runner — runner de orquestación de nan-opencode-delegation.\n\n")
	b.WriteString("Uso, desde la raíz del proyecto en el que se trabaja:\n")
	b.WriteString("  runner start            contexto operativo integrado (memoria, tablero, Git, fase y pendientes)\n")
	b.WriteString("  runner aif              descubrimiento y operaciones AIF: envelope JSON por stdin, respuesta por stdout\n")
	b.WriteString("  runner --json           estado de fases de la sesión, solo lectura, para plugins y TUI\n")
	b.WriteString("  runner recover          archiva una base dañada, rescata lo legible y reconstruye la fase\n")
	b.WriteString("  runner export-legacy    regenera .run/orchestrator-runner.json y .run/delegations.jsonl\n")
	b.WriteString("  runner --version        versión y commit del ejecutable\n")
	b.WriteString("  runner --help           esta ayuda\n\n")
	b.WriteString("Operaciones AIF (envelope {\"aif\":\"0.2\",\"operation\":\"<id>\",\"input\":{...}} por stdin):\n")
	for _, op := range operations {
		b.WriteString("  " + op.ID + " — " + op.Description + "\n")
	}
	b.WriteString("\nCódigos de salida: 0 executed/descubrimiento, 2 input_needed/invalid, 3 contrato, 4 denied, 1 failed.\n")
	return b.String()
}
