package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	aif "github.com/fred66sp/agent-interaction-forms"

	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/aifops"
	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/boot"
	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/phase"
	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/store"
)

// Overridden at build time: -ldflags "-X main.version=... -X main.commit=...".
var (
	version = "dev"
	commit  = "dev"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	command := ""
	if len(args) > 0 {
		command = args[0]
	}
	root := "."
	session := os.Getenv("OPENCODE_SESSION_ID")

	switch command {
	case "", "--help", "-h":
		fmt.Fprint(stdout, aifops.Help(aifops.New(nil, "").Operations))
		return 0
	case "--version", "-v":
		fmt.Fprintf(stdout, "orchestrator-runner %s (%s)\n", version, commit)
		return 0
	case "aif":
		return runAIF(stdin, stdout, stderr, root, session)
	case "--json":
		return runJSON(stdout, stderr, root, session)
	case "start":
		fmt.Fprint(stdout, boot.Start(root, session))
		return 0
	case "recover":
		return runRecover(stdout, stderr, root, session)
	case "export-legacy":
		return runExport(stdout, stderr, root, session)
	default:
		fmt.Fprintf(stderr, "error: subcomando no reconocido: %s\n", command)
		fmt.Fprintln(stderr, "consulta `runner --help`")
		return 1
	}
}

func runAIF(stdin io.Reader, stdout, stderr io.Writer, root, session string) int {
	raw, err := io.ReadAll(io.LimitReader(stdin, aif.MaxInputBytes+1))
	if err != nil {
		fmt.Fprintf(stderr, "error: no se pudo leer stdin: %v\n", err)
		return 1
	}
	s, err := store.Open(root, true)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	defer s.Close()
	response := aifops.New(s, session).Handle(raw)
	out, err := json.Marshal(response)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, string(out))
	return exitCode(response.Result)
}

func exitCode(result string) int {
	switch result {
	case "", aif.ResultExecuted:
		return 0
	case aif.ResultInputNeeded, aif.ResultInvalid:
		return 2
	case aif.ResultUnknownOperation, aif.ResultUnsupportedVersion, aif.ResultMalformedRequest:
		return 3
	case aif.ResultDenied:
		return 4
	default:
		return 1
	}
}

type stateJSON struct {
	Exists        bool             `json:"exists"`
	Phase         int              `json:"phase"`
	Valid         bool             `json:"valid"`
	EvidenceValid bool             `json:"evidence_valid"`
	Evidence      []phase.Evidence `json:"evidence"`
}

func runJSON(stdout, stderr io.Writer, root, session string) int {
	state := stateJSON{Phase: 1, Valid: true, EvidenceValid: true, Evidence: []phase.Evidence{}}
	// Without an identifiable session the contract is the empty state: there is
	// no invoking session whose phase could be answered.
	if session != "" {
		s, err := store.Open(root, false)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		defer s.Close()
		current, err := s.SessionPhase(session)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		evidence := current.Evidence
		if evidence == nil {
			evidence = []phase.Evidence{}
		}
		state = stateJSON{
			Exists:        current.Exists,
			Phase:         current.Phase,
			Valid:         current.Valid,
			EvidenceValid: current.EvidenceValid,
			Evidence:      evidence,
		}
	}
	payload := struct {
		Version int           `json:"version"`
		Phases  []phase.Phase `json:"phases"`
		State   stateJSON     `json:"state"`
	}{
		Version: 1,
		Phases:  phase.Phases,
		State:   state,
	}
	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, string(out))
	return 0
}

func runRecover(stdout, stderr io.Writer, root, session string) int {
	s, err := store.Open(root, true)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	defer s.Close()
	report, err := s.Recover(session)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	writeJSON(stdout, stderr, report)
	return 0
}

func runExport(stdout, stderr io.Writer, root, session string) int {
	s, err := store.Open(root, false)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	defer s.Close()
	report, err := s.ExportLegacy(session)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	writeJSON(stdout, stderr, report)
	return 0
}

func writeJSON(stdout, stderr io.Writer, value any) {
	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return
	}
	fmt.Fprintln(stdout, string(out))
}
