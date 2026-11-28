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

func TestExpectedCheckWithColonWithoutResult(t *testing.T) {
	// La línea es el comando esperado entero, ':' incluido: el lado izquierdo
	// del ':' solo casa por similitud, y la línea completa casa mejor, así que
	// no hay resultado observado y la comprobación no puede contar como cumplida.
	want := []string{"npm run very-long-check-name:unit"}
	text := "status: completed\n\nvalidation:\n  npm run very-long-check-name:unit\n"
	result := Validate(text, want)
	if result.OK || len(result.Checks) != 1 || result.Checks[0].Observed != "" {
		t.Fatalf("veredicto = %+v; want !OK con observed vacío", result)
	}
}

func TestObservedKeepsItsColons(t *testing.T) {
	// El separador es el que cierra el comando que mejor casa: el resultado
	// conserva sus ':'.
	want := []string{"bash check-package.sh"}
	text := "status: completed\n\nvalidation:\n  bash check-package.sh: 12 pasan: 0 fallan\n"
	result := Validate(text, want)
	if !result.OK || result.Checks[0].Observed != "12 pasan: 0 fallan" {
		t.Fatalf("veredicto = %+v", result)
	}
}

func TestExpectedAndObservedWithColons(t *testing.T) {
	// El ':' del comando esperado no se confunde con el separador ni con los
	// ':' del resultado.
	want := []string{"npm run test:unit"}
	text := "status: completed\n\nvalidation:\n  npm run test:unit: 12 pasan: 0 fallan\n"
	result := Validate(text, want)
	if !result.OK || result.Checks[0].Observed != "12 pasan: 0 fallan" {
		t.Fatalf("veredicto = %+v", result)
	}
}

func TestEmptyObservedIsRejected(t *testing.T) {
	// Separador presente y nada después: la línea no cumple, tanto si el
	// comando esperado tiene ':' como si no.
	cases := []struct{ want, line string }{
		{"bash check-package.sh", "bash check-package.sh:"},
		{"npm run test:unit", "npm run test:unit:"},
	}
	for _, c := range cases {
		text := "status: completed\n\nvalidation:\n  " + c.line + "\n"
		result := Validate(text, []string{c.want})
		if result.OK || len(result.Checks) != 1 || result.Checks[0].Observed != "" {
			t.Fatalf("%q: veredicto = %+v; want !OK sin resultado", c.line, result)
		}
		if !contains(result.Violations, "sin resultado observado") {
			t.Fatalf("%q: no nombra el resultado ausente: %v", c.line, result.Violations)
		}
	}
}

func TestGluedSeparatorWithExactCommandIsAccepted(t *testing.T) {
	// cmd:resultado, sin espacio tras el separador, sigue siendo válido: con un
	// comando sin ':' y con uno que lo lleva.
	cases := []struct{ expected, line, observed string }{
		{"bash check-package.sh", "bash check-package.sh:verde", "verde"},
		{"npm run test:unit", "npm run test:unit:12 pasan", "12 pasan"},
	}
	for _, c := range cases {
		text := "status: completed\n\nvalidation:\n  " + c.line + "\n"
		result := Validate(text, []string{c.expected})
		if !result.OK || len(result.Checks) != 1 || result.Checks[0].Observed != c.observed {
			t.Fatalf("%q: veredicto = %+v", c.line, result)
		}
	}
}

func TestGluedSeparatorKeepsTolerance(t *testing.T) {
	// El separador pegado conserva la tolerancia previa: errata en un comando
	// sin ':' y variación del comando en uno que lleva ':'.
	cases := []struct{ expected, line, observed string }{
		{"python3 -m py_compile agents/orchestrator-runner.py", "python3 -m pycompile agents/orchestrator-runner.py:sin salida", "sin salida"},
		{"npm run test:unit", "npm run tst:unit:12 pasan", "12 pasan"},
		{"npm run test:unit", "npm run test:uni:12 pasan", "12 pasan"},
	}
	for _, c := range cases {
		text := "status: completed\n\nvalidation:\n  " + c.line + "\n"
		result := Validate(text, []string{c.expected})
		if !result.OK || len(result.Checks) != 1 || result.Checks[0].Observed != c.observed {
			t.Fatalf("%q: veredicto = %+v", c.line, result)
		}
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
