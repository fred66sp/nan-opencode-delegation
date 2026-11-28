package report

import (
	"strings"
	"testing"
)

func checks() []string {
	return []string{"python3 -m py_compile agents/orchestrator-runner.py", "bash check-package.sh"}
}

const conforming = `status: completed

Se revisó el fichero agents/orchestrator-runner.py y compila.

validation:
  python3 -m py_compile agents/orchestrator-runner.py: sin salida, código 0
  bash check-package.sh: Todo en verde., código 0
`

func TestConformingReport(t *testing.T) {
	result := Validate(conforming, checks())
	if !result.OK || len(result.Violations) != 0 || result.Status != "completed" {
		t.Fatalf("veredicto = %+v", result)
	}
	if len(result.Checks) != 2 || result.Checks[0].Observed == "" || result.Checks[1].Observed == "" {
		t.Fatalf("comprobaciones = %+v", result.Checks)
	}
}

func TestMissingStatus(t *testing.T) {
	text := "validation:\n  python3 -m py_compile agents/orchestrator-runner.py: sin salida, código 0\n  bash check-package.sh: verde\n"
	result := Validate(text, checks())
	if result.OK || !contains(result.Violations, "falta la línea") {
		t.Fatalf("veredicto = %+v", result)
	}
}

func TestMissingExpectedCheck(t *testing.T) {
	text := "status: partial\n\nvalidation:\n  bash check-package.sh: rojo, 2 comprobaciones fallidas\n"
	result := Validate(text, checks())
	if result.OK || result.Status != "partial" {
		t.Fatalf("veredicto = %+v", result)
	}
	if !contains(result.Violations, "orchestrator-runner.py") {
		t.Fatalf("no nombra la comprobación ausente: %v", result.Violations)
	}
}

func TestContradictoryStatus(t *testing.T) {
	text := conforming + "\nstatus: partial\n"
	result := Validate(text, checks())
	if result.OK || !contains(result.Violations, "contradictorios") {
		t.Fatalf("veredicto = %+v", result)
	}
}

func TestAnaphoraWithoutPath(t *testing.T) {
	text := conforming + "\nSe revisó el fichero y quedó listo.\n"
	result := Validate(text, checks())
	if result.OK || !contains(result.Violations, "referencia indefinida") {
		t.Fatalf("veredicto = %+v", result)
	}
}

func TestQuestionLineIsRejected(t *testing.T) {
	text := conforming + "\npregunta: ¿se publica?\n"
	result := Validate(text, checks())
	if result.OK || !contains(result.Violations, "pregunta") {
		t.Fatalf("veredicto = %+v", result)
	}
}

func TestCommandTolerance(t *testing.T) {
	// Prefijo, `./` y variación trivial del comando esperado.
	text := `status: completed

validation:
  ./python3 -m pycompile agents/orchestrator-runner.py: sin salida
  bash check-package.sh 2>&1: verde
`
	result := Validate(text, checks())
	if !result.OK {
		t.Fatalf("veredicto = %+v", result)
	}
}

func TestAnaphoraWithPathIsAccepted(t *testing.T) {
	text := conforming + "\nSe revisó el fichero docs/runner/DESIGN.md por completo.\n"
	if result := Validate(text, checks()); !result.OK {
		t.Fatalf("la ruta resuelve la anáfora: %+v", result)
	}
}

func TestExpectedCheckWithColon(t *testing.T) {
	// El check esperado lleva ':' y el resultado también: el corte es el que casa.
	want := []string{"npm run test:unit"}
	text := "status: completed\n\nvalidation:\n  npm run test:unit: 12 pasan, 0 fallan\n"
	result := Validate(text, want)
	if !result.OK || result.Checks[0].Observed != "12 pasan, 0 fallan" {
		t.Fatalf("veredicto = %+v", result)
	}
}

func contains(values []string, fragment string) bool {
	for _, value := range values {
		if strings.Contains(value, fragment) {
			return true
		}
	}
	return false
}
