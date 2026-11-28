package phase

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Problem describes why a field of a transition was rejected.
type Problem struct {
	Field  string
	Reason string
}

// EvidenceProblem validates the evidence of phase_next. The Python runner
// rejects empty text and text starting with `--`; a pasted JSON object or array
// is rejected too because it is not the prose evidence the phase expects.
func EvidenceProblem(text string) *Problem {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return &Problem{"evidence", "la evidencia no puede estar vacía ni ser solo espacios"}
	}
	if strings.HasPrefix(text, "--") {
		return &Problem{"evidence", "la evidencia no puede empezar por --"}
	}
	if (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) && json.Valid([]byte(trimmed)) {
		return &Problem{"evidence", "la evidencia no puede ser JSON: declara en prosa qué se hizo y qué se comprobó"}
	}
	return nil
}

// ReasonProblem validates the mandatory reason of a skip or a reset.
func ReasonProblem(field, text string) *Problem {
	if strings.TrimSpace(text) == "" {
		return &Problem{field, "el motivo no puede estar vacío ni ser solo espacios"}
	}
	if strings.HasPrefix(text, "--") {
		return &Problem{field, "el motivo no puede empezar por --"}
	}
	return nil
}

// SkipProblem validates a jump from current to target with the same rules as
// the Python runner: forward, inside the flow, omitting at least one phase and
// never leaving the terminal phase. Every rejection names the current phase and
// the valid range so a consumer can retry with the response alone.
func SkipProblem(current, target int) *Problem {
	if current >= Count {
		return &Problem{"target", fmt.Sprintf("no hay transición: la fase actual %d (%s) es la última", current, Get(current).Name)}
	}
	lo := current + 2
	valid := ""
	if lo <= Count {
		valid = fmt.Sprintf("el destino válido va de %d (%s) a %d (%s)",
			lo, Get(lo).Name, Count, Get(Count).Name)
	} else {
		valid = fmt.Sprintf("desde la fase %d (%s) ningún salto omite una fase: usa next", current, Get(current).Name)
	}
	if target > Count {
		return &Problem{"target", fmt.Sprintf("la fase de destino %d no existe: el flujo tiene %d fases; desde la fase %d (%s), %s",
			target, Count, current, Get(current).Name, valid)}
	}
	if target <= current {
		return &Problem{"target", fmt.Sprintf("no se retrocede: la fase actual es %d (%s); %s",
			current, Get(current).Name, valid)}
	}
	if target == current+1 {
		return &Problem{"target", fmt.Sprintf("la fase actual es %d (%s); el salto no omitiría ninguna fase: la siguiente es %d (%s); %s",
			current, Get(current).Name, current+1, Get(current+1).Name, valid)}
	}
	return nil
}

// SkippedRange returns the omitted phase numbers of a jump.
func SkippedRange(from, to int) []int {
	var out []int
	for n := from + 1; n < to; n++ {
		out = append(out, n)
	}
	return out
}

// SkippedNotes renders the skipped phases recorded in the history, with the
// same wording as the Python runner. A jump over Commit keeps the human
// confirmation warning.
func SkippedNotes(entries []Evidence) []string {
	var notes []string
	for _, entry := range entries {
		if entry.Skip <= entry.Phase+1 {
			continue
		}
		var line string
		if entry.Skip-entry.Phase == 2 {
			line = fmt.Sprintf("Omitidas: fase %d (salto de la %d a la %d) · %s", entry.Phase+1, entry.Phase, entry.Skip, entry.Text)
		} else {
			line = fmt.Sprintf("Omitidas: fases %d-%d (salto de la %d a la %d) · %s", entry.Phase+1, entry.Skip-1, entry.Phase, entry.Skip, entry.Text)
		}
		if entry.Phase <= 9 && 9 < entry.Skip {
			line += " — Commit omitido: la confirmación explícita de la persona sigue pendiente."
		}
		notes = append(notes, line)
	}
	return notes
}
