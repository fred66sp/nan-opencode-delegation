package phase

import (
	"strings"
	"testing"
)

func TestEvidenceProblem(t *testing.T) {
	cases := map[string]bool{
		"leí la memoria y el tablero": true,
		"":                            false,
		"   ":                         false,
		"--help":                      false,
		`{"phase":2}`:                 false,
	}
	for text, valid := range cases {
		if problem := EvidenceProblem(text); (problem == nil) != valid {
			t.Fatalf("EvidenceProblem(%q) = %v, válido esperado %v", text, problem, valid)
		}
	}
}

func TestSkipProblem(t *testing.T) {
	if problem := SkipProblem(3, 5); problem != nil {
		t.Fatalf("salto válido: %v", problem)
	}
	for _, pair := range [][2]int{{3, 3}, {3, 2}, {3, 4}, {3, 12}, {11, 11}} {
		if problem := SkipProblem(pair[0], pair[1]); problem == nil {
			t.Fatalf("salto %d→%d debía rechazarse", pair[0], pair[1])
		}
	}
}

func TestSkippedNotes(t *testing.T) {
	entries := []Evidence{
		{Phase: 2, Skip: 4, Text: "una omitida"},
		{Phase: 8, Skip: 11, Text: "commit fuera"},
	}
	notes := SkippedNotes(entries)
	if len(notes) != 2 {
		t.Fatalf("notas = %v", notes)
	}
	if !strings.Contains(notes[0], "Omitidas: fase 3") {
		t.Fatalf("nota simple = %q", notes[0])
	}
	if !strings.Contains(notes[1], "Omitidas: fases 9-10") || !strings.Contains(notes[1], "Commit omitido") {
		t.Fatalf("nota con commit = %q", notes[1])
	}
}

func TestPhaseTextsMatchTheElevenPhases(t *testing.T) {
	if len(Phases) != Count {
		t.Fatalf("%d fases", len(Phases))
	}
	for i, p := range Phases {
		if p.Number != i+1 || p.Name == "" || p.When == "" || p.Evidence == "" || len(p.Steps) == 0 || len(p.Warnings) == 0 {
			t.Fatalf("fase %d incompleta: %+v", i+1, p)
		}
	}
}

// TestPhaseStepsDoNotTeachRetiredFlow impide que vuelvan las instrucciones del
// runner anterior: arranque con `acm start`, libro manual y reset sin nombre.
func TestPhaseStepsDoNotTeachRetiredFlow(t *testing.T) {
	if first := strings.Join(Phases[0].Steps, " "); !strings.Contains(first, "runner start") {
		t.Fatalf("la fase 1 no nombra `runner start`: %v", Phases[0].Steps)
	}
	for _, p := range Phases {
		lines := append([]string{p.Evidence}, p.Steps...)
		for _, line := range lines {
			for _, retired := range []string{"acm start", "delegations.jsonl", "`reset`"} {
				if strings.Contains(line, retired) {
					t.Fatalf("la fase %d enseña el flujo retirado %q: %q", p.Number, retired, line)
				}
			}
		}
	}
	if last := strings.Join(Phases[Count-1].Steps, " "); !strings.Contains(last, "phase_reset") {
		t.Fatalf("la fase terminal no nombra `phase_reset`: %v", Phases[Count-1].Steps)
	}
}
