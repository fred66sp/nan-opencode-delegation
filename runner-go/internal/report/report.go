// Package report ports the rules of agents/delegation-report-checker.py: the
// structure of a subagent return, not the truth of the work.
package report

import (
	"regexp"
	"strings"
)

var statuses = map[string]bool{"completed": true, "partial": true, "blocked": true}

var (
	statusLine   = regexp.MustCompile(`(?im)^\s*(?:[-*]\s+)?\**status\**\s*:\s*(.*)$`)
	questionLine = regexp.MustCompile(`(?im)^\s*(?:[-*]\s+)?\**(?:pregunta|question)\**\s*:`)
	anaphora     = regexp.MustCompile(`(?i)\b(?:el|la|ese|esa|este|esta|dicho|dicha|dichos|dichas)\s+(?:fichero|archivo|paso|l[ií]nea|funci[oó]n|m[oó]dulo|tabla|secci[oó]n|comando|script|campo)s?\b`)
	pathLike     = regexp.MustCompile(`(?:[\w.-]+/[\w./-]+|[\w.-]+\.(?:py|sh|md|json|jsonl|jsonc|tsx?|txt|ya?ml|toml|cfg|ini)|\b[\w.-]+:\d+\b)`)
)

// Check is one expected verification and the result observed in the report.
type Check struct {
	Expected string `json:"expected"`
	Observed string `json:"observed"`
}

// Result is the structural verdict, with the same fields the checker emits.
type Result struct {
	OK         bool     `json:"ok"`
	Status     string   `json:"status"`
	Checks     []Check  `json:"checks"`
	Violations []string `json:"violations"`
}

// Validate applies the checker rules to a report against the expected checks.
func Validate(text string, expected []string) Result {
	violations := []string{}

	var values []string
	for _, match := range statusLine.FindAllStringSubmatch(text, -1) {
		value := strings.TrimSpace(match[1])
		value = strings.Trim(value, "*`")
		value = strings.Trim(value, ".,;")
		values = append(values, strings.ToLower(value))
	}
	if len(values) == 0 {
		violations = append(violations, "falta la línea `status: completed|partial|blocked`")
	}
	for _, value := range values {
		if !statuses[value] {
			violations = append(violations, "status fuera del conjunto: «"+value+"»; se espera completed, partial o blocked")
		}
	}
	if distinct(values) > 1 {
		quoted := make([]string, len(values))
		for i, value := range values {
			quoted[i] = "«" + value + "»"
		}
		violations = append(violations, "status contradictorios: "+strings.Join(quoted, ", ")+"; se espera un único completed, partial o blocked")
	}
	if questionLine.MatchString(text) {
		violations = append(violations, "el informe declara una `pregunta:`: un bloqueo de decisión se devuelve como pregunta al humano, no como estado")
	}

	heading, lines := validationSection(text)
	if !heading {
		violations = append(violations, "falta la sección `validation` con una línea por comprobación esperada")
	}

	checks := make([]Check, 0, len(expected))
	for _, want := range expected {
		observed := ""
		found := false
		for _, line := range lines {
			if result, ok := line.resultOf(want); ok {
				observed, found = result, true
				break
			}
		}
		checks = append(checks, Check{Expected: want, Observed: observed})
		if !found {
			violations = append(violations, "validation no cita la comprobación esperada: "+want)
		} else if observed == "" {
			violations = append(violations, "validation cita el comando sin resultado observado: "+want)
		}
	}

	for _, line := range strings.Split(text, "\n") {
		mention := anaphora.FindString(line)
		if mention != "" && !pathLike.MatchString(line) {
			violations = append(violations, "referencia indefinida sin ruta en la misma línea: «"+mention+"»")
		}
	}

	status := ""
	if len(values) > 0 {
		status = values[0]
	}
	return Result{OK: len(violations) == 0, Status: status, Checks: checks, Violations: violations}
}

type validationLine struct{ text string }

// resultOf returns the result observed after the separator that ends want.
// The expected check may contain ':' itself (npm run test:unit), so every
// colon is a candidate separator and the one whose left side is the closest
// command wins: the existing tolerance, a separator glued to the result
// (cmd:resultado) and the observed result's own colons all survive. When the
// whole line matches want better than its best command prefix, the colon is
// inside the command and the line is a bare expected check with ':': there is
// no result to report.
func (l validationLine) resultOf(want string) (string, bool) {
	runes := []rune(l.text)
	wantNorm := normalize(want)
	bestAt, bestScore := -1, 0.0
	for i, r := range runes {
		if r != ':' {
			continue
		}
		command := strings.TrimSpace(strings.Trim(strings.TrimSpace(string(runes[:i])), "`*"))
		if command == "" || !matches(want, command) {
			continue
		}
		if score := similarity(wantNorm, normalize(command)); bestAt < 0 || score > bestScore {
			bestAt, bestScore = i, score
		}
	}
	if bestAt < 0 || similarity(wantNorm, normalize(string(runes))) > bestScore {
		return "", false
	}
	return strings.TrimSpace(string(runes[bestAt+1:])), true
}

func validationSection(text string) (bool, []validationLine) {
	heading := false
	var lines []validationLine
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		head := strings.Trim(strings.TrimLeft(line, "#*-"), " ")
		lower := strings.ToLower(head)
		if lower == "validation" || strings.HasPrefix(lower, "validation:") {
			heading = true
			if !strings.Contains(head, ":") {
				continue
			}
			line = strings.TrimSpace(strings.SplitN(head, ":", 2)[1])
			if line == "" {
				continue
			}
		}
		if !strings.Contains(line, ":") {
			continue
		}
		lines = append(lines, validationLine{line})
	}
	return heading, lines
}

// SameCheck reports whether a reported check names an expected one, with the
// checker's matching tolerance. The return classifies expected checks with it.
func SameCheck(expected, reported string) bool { return matches(expected, reported) }

func normalize(command string) string {
	tokens := strings.Fields(strings.TrimSpace(strings.Trim(strings.TrimSpace(command), "`")))
	for i, token := range tokens {
		tokens[i] = strings.TrimPrefix(token, "./")
	}
	return strings.Join(tokens, " ")
}

func matches(expected, observed string) bool {
	expected, observed = normalize(expected), normalize(observed)
	return observed == expected || strings.HasPrefix(observed, expected) || similarity(expected, observed) >= 0.9
}

func distinct(values []string) int {
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	return len(seen)
}

// similarity is difflib.SequenceMatcher.ratio: twice the matched runes over the
// total runes. It keeps the checker's 0.9 tolerance on trivial variations.
func similarity(a, b string) float64 {
	ar, br := []rune(a), []rune(b)
	total := len(ar) + len(br)
	if total == 0 {
		return 1
	}
	return 2 * float64(matched(ar, 0, len(ar), br, 0, len(br))) / float64(total)
}

func matched(a []rune, alo, ahi int, b []rune, blo, bhi int) int {
	bestI, bestJ, bestSize := alo, blo, 0
	for i := alo; i < ahi; i++ {
		for j := blo; j < bhi; j++ {
			size := 0
			for i+size < ahi && j+size < bhi && a[i+size] == b[j+size] {
				size++
			}
			if size > bestSize {
				bestI, bestJ, bestSize = i, j, size
			}
		}
	}
	if bestSize == 0 {
		return 0
	}
	return bestSize +
		matched(a, alo, bestI, b, blo, bestJ) +
		matched(a, bestI+bestSize, ahi, b, bestJ+bestSize, bhi)
}
