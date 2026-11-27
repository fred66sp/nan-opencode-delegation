// Package boot composes the integrated start context: memory, board, Git, the
// phase of this session and the open assignments. Each source degrades on its
// own to `no disponible`; the command never fails because a source is missing.
package boot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/phase"
	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/store"
)

// Start renders the integrated context. Alone among the commands it may open
// the ACM session; everything else it reads.
func Start(root, session string) string {
	sections := []string{
		memory(),
		board(root),
		git(root),
		phaseSection(root, session),
		assignments(root, session),
	}
	return strings.Join(sections, "\n\n")
}

func memory() string {
	home, _ := os.UserHomeDir()
	acm := locate("acm", filepath.Join(home, ".local", "bin", "acm"))
	lines := []string{"## Memoria"}
	if acm == "" {
		return strings.Join(append(lines, "no disponible: acm no está instalado ni en el PATH"), "\n")
	}
	output, err := capture(".", "", acm, "start")
	if err != nil {
		return strings.Join(append(lines, "no disponible: "+err.Error()), "\n")
	}
	return strings.Join(append(lines, head(output, 30)...), "\n")
}

type task struct {
	Ref    string `json:"ref"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

func board(root string) string {
	lines := []string{"## Tablero"}
	binary := locate(filepath.Join(root, ".tasks-ai", "bin", "tasks-ai"), "tasks-ai")
	if binary == "" {
		return strings.Join(append(lines, "no disponible: no hay CLI tasks-ai, ni .tasks-ai/bin ni en el PATH"), "\n")
	}

	// The board operation carries exactly the two lists the start needs.
	var response struct {
		Result  string `json:"result"`
		Outcome struct {
			InProgress []task `json:"in_progress"`
			Ready      []task `json:"ready"`
		} `json:"outcome"`
	}
	output, err := capture(root, `{"aif":"0.2","operation":"tasks_board"}`, binary, "aif")
	if err == nil && json.Unmarshal([]byte(output), &response) == nil && response.Result == "executed" {
		lines = append(lines, taskLines(response.Outcome.InProgress, false)...)
		lines = append(lines, taskLines(response.Outcome.Ready, true)...)
		return strings.Join(lines, "\n")
	}

	output, err = capture(root, "", binary, "list")
	if err != nil {
		return strings.Join(append(lines, "no disponible: tasks-ai list "+err.Error()), "\n")
	}
	var tasks []task
	if err := json.Unmarshal([]byte(output), &tasks); err != nil {
		return strings.Join(append(lines, "no disponible: la salida de tasks-ai list no es JSON"), "\n")
	}
	var course, ready []task
	for _, t := range tasks {
		switch t.Status {
		case "in_progress":
			course = append(course, t)
		case "open":
			ready = append(ready, t)
		}
	}
	lines = append(lines, taskLines(course, false)...)
	lines = append(lines, taskLines(ready, true)...)
	return strings.Join(lines, "\n")
}

func taskLines(tasks []task, ready bool) []string {
	title := "En curso:"
	if ready {
		title = "Listas para empezar:"
	}
	if len(tasks) == 0 {
		return []string{title, "  - ninguna"}
	}
	lines := []string{title}
	for _, t := range tasks {
		lines = append(lines, fmt.Sprintf("  - %s · %s", t.Ref, t.Title))
	}
	return lines
}

func git(root string) string {
	lines := []string{"## Git"}
	if locate("git") == "" {
		return strings.Join(append(lines, "no disponible: git no está instalado"), "\n")
	}
	branch, err := capture(root, "", "git", "branch", "--show-current")
	if err != nil {
		return strings.Join(append(lines, "no disponible: git branch "+err.Error()), "\n")
	}
	status, err := capture(root, "", "git", "status", "--porcelain")
	if err != nil {
		return strings.Join(append(lines, "no disponible: git status "+err.Error()), "\n")
	}
	changed := 0
	for _, line := range strings.Split(status, "\n") {
		if strings.TrimSpace(line) != "" {
			changed++
		}
	}
	lines = append(lines, "Rama: "+strings.TrimSpace(branch))
	if changed == 0 {
		lines = append(lines, "Estado: limpio")
	} else {
		lines = append(lines, fmt.Sprintf("Estado: %d fichero(s) cambiado(s)", changed))
	}
	lines = append(lines, "Últimos commits:")
	log, err := capture(root, "", "git", "log", "-3", "--pretty=format:%h %s")
	if err != nil {
		return strings.Join(append(lines, "no disponible: git log "+err.Error()), "\n")
	}
	found := false
	for _, line := range strings.Split(log, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, "  - "+line)
			found = true
		}
	}
	if !found {
		lines = append(lines, "  - (sin commits)")
	}
	return strings.Join(lines, "\n")
}

func phaseSection(root, session string) string {
	lines := []string{"## Fase"}
	s, err := store.Open(root, false)
	if err != nil {
		return strings.Join(append(lines, "no disponible: "+err.Error()), "\n")
	}
	defer s.Close()
	state, err := s.SessionPhase(session)
	if err != nil {
		return strings.Join(append(lines, "no disponible: "+err.Error()), "\n")
	}
	if state.Corrupt {
		return strings.Join(append(lines, "estado corrupto en "+s.Path+": ejecuta `runner recover` (se muestra la fase 1)"), "\n")
	}
	if !state.Exists {
		lines = append(lines, "sin estado para esta sesión: la primera invocación empieza en la fase 1")
	}
	p := phase.Get(state.Phase)
	lines = append(lines, fmt.Sprintf("Fase %d/%d · %s", p.Number, phase.Count, p.Name))
	lines = append(lines, "Cuándo: "+p.When)
	lines = append(lines, "Evidencia esperada: "+p.Evidence)
	lines = append(lines, phase.SkippedNotes(state.Evidence)...)
	if report, err := s.MigrationReport(); err == nil && report != nil && len(report.Legacy) > 0 {
		lines = append(lines, "", "Migración del estado legado: fase "+fmt.Sprint(report.Phase)+
			fmt.Sprintf(", %d evidencia(s), %d delegación(es), %d nota(s), %d omitida(s).", report.Evidence, report.Delegations, report.Notes, report.Skipped))
		for _, limitation := range report.Limitations {
			lines = append(lines, "  - "+limitation)
		}
	}
	return strings.Join(lines, "\n")
}

func assignments(root, session string) string {
	lines := []string{"## Encargos abiertos"}
	s, err := store.Open(root, false)
	if err != nil {
		return strings.Join(append(lines, "no disponible: "+err.Error()), "\n")
	}
	defer s.Close()
	open, err := s.OpenDelegations(10)
	if err != nil {
		return strings.Join(append(lines, "no disponible: "+err.Error()), "\n")
	}
	if len(open) == 0 {
		return strings.Join(append(lines, "  - ninguno"), "\n")
	}
	for _, d := range open {
		what := d.Task
		if what == "" {
			what = d.Goal
		}
		lines = append(lines, fmt.Sprintf("  - %s · %s · %s", d.State, d.ID, what))
	}
	return strings.Join(lines, "\n")
}

func locate(name string, fallbacks ...string) string {
	candidates := append([]string{name}, fallbacks...)
	for _, candidate := range candidates {
		if filepath.IsAbs(candidate) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func capture(dir, stdin, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	if stdin != "" {
		command.Stdin = strings.NewReader(stdin)
	}
	var out, errOut bytes.Buffer
	command.Stdout, command.Stderr = &out, &errOut
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return "", errors.New("sin respuesta en 15 s")
		}
		detail := strings.TrimSpace(errOut.String())
		if detail == "" {
			detail = strings.TrimSpace(out.String())
		}
		if detail != "" {
			return "", errors.New(strings.Split(detail, "\n")[0])
		}
		return "", fmt.Errorf("salió con código %d", command.ProcessState.ExitCode())
	}
	return out.String(), nil
}

func head(text string, limit int) []string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{"(sin contenido)"}
	}
	if len(lines) <= limit {
		return lines
	}
	return append(lines[:limit], fmt.Sprintf("… %d líneas más", len(lines)-limit))
}
