// Command frictions records and lists the friction of the orchestrator in an
// append-only JSONL file of the user, outside the runner store.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fred66sp/nan-opencode-delegation/runner-go/internal/frictions"
)

const usage = `frictions registra las fricciones del orquestador en un JSONL de usuario.

Uso:
  frictions add --class C --surface S --observed TEXTO [--key K] [--project P]
  frictions list [--min N] [--limit N] [--json] [--all]
  frictions fixed HUELLA

Clases: contract, tool, environment, doc
Superficies: runner, tasks-ai, acm, opencode, subagent, git
Códigos: 0 correcto, 2 uso inválido, 1 error de E/S.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "add":
		return runAdd(args[1:], stdout, stderr)
	case "list":
		return runList(args[1:], stdout, stderr)
	case "fixed":
		return runFixed(args[1:], stdout, stderr)
	case "--help", "-h", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "error: subcomando no reconocido: %s\n", args[0])
		fmt.Fprint(stderr, usage)
		return 2
	}
}

func runAdd(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("add", flag.ContinueOnError)
	flags.SetOutput(stderr)
	class := flags.String("class", "", "clase de la fricción")
	surface := flags.String("surface", "", "superficie donde ocurrió")
	observed := flags.String("observed", "", "hecho observable, con comando y mensaje")
	key := flags.String("key", "", "clave estable; por defecto Normalize(observed)")
	project := flags.String("project", "", "proyecto; por defecto el directorio actual")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "error: argumento no esperado: %s\n", flags.Arg(0))
		return 2
	}
	if !frictions.ValidClass(*class) {
		fmt.Fprintf(stderr, "error: clase fuera del conjunto: %q; se espera %s\n", *class, strings.Join(frictions.Classes, ", "))
		return 2
	}
	if !frictions.ValidSurface(*surface) {
		fmt.Fprintf(stderr, "error: superficie fuera del conjunto: %q; se espera %s\n", *surface, strings.Join(frictions.Surfaces, ", "))
		return 2
	}
	if strings.TrimSpace(*observed) == "" {
		fmt.Fprintln(stderr, "error: --observed es obligatorio y no puede estar vacío")
		return 2
	}
	if *project == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "error: no pude resolver el proyecto: %v\n", err)
			return 1
		}
		*project = filepath.Base(cwd)
	}
	path, err := frictions.Path()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	entry, err := frictions.Add(path, frictions.Record{
		Class: *class, Surface: *surface, Key: *key,
		Observed: *observed, Project: *project, Now: time.Now(),
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if entry.Count == 1 {
		fmt.Fprintf(stdout, "new %s · %s/%s · %s\n", entry.FP, entry.Class, entry.Surface, entry.Key)
	} else {
		fmt.Fprintf(stdout, "seen %d× since %s · %s · %s/%s · %s\n",
			entry.Count, entry.First, entry.FP, entry.Class, entry.Surface, entry.Key)
	}
	return 0
}

func runList(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("list", flag.ContinueOnError)
	flags.SetOutput(stderr)
	min := flags.Int("min", 1, "conteo mínimo")
	limit := flags.Int("limit", 20, "máximo de entradas; 0 = todas")
	asJSON := flags.Bool("json", false, "salida JSON")
	all := flags.Bool("all", false, "incluye las fijadas, que por defecto salen del listado")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "error: argumento no esperado: %s\n", flags.Arg(0))
		return 2
	}
	if *min < 1 || *limit < 0 {
		fmt.Fprintln(stderr, "error: --min debe ser >= 1 y --limit >= 0")
		return 2
	}
	path, err := frictions.Path()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	events, skipped, err := frictions.Read(path)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if skipped > 0 {
		fmt.Fprintf(stderr, "aviso: %d línea(s) ilegibles omitidas\n", skipped)
	}
	entries := frictions.Aggregate(events)
	if !*all {
		entries = frictions.Open(entries)
	}
	entries = frictions.Select(entries, *min, *limit)
	if *asJSON {
		out, err := json.Marshal(entries)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(out))
		return 0
	}
	if len(entries) == 0 {
		fmt.Fprintln(stdout, "sin fricciones")
		return 0
	}
	for _, entry := range entries {
		fmt.Fprintf(stdout, "%s  %d×  %s/%s  %s  %s → %s  %s\n",
			entry.FP, entry.Count, entry.Class, entry.Surface, entry.Key,
			entry.First, entry.Last, truncate(entry.Observed, 80))
	}
	return 0
}

func runFixed(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("fixed", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "error: fixed exige una huella")
		fmt.Fprint(stderr, usage)
		return 2
	}
	fp := flags.Arg(0)
	path, err := frictions.Path()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	result, err := frictions.Fix(path, fp, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	switch result {
	case frictions.Fixed:
		fmt.Fprintf(stdout, "fixed %s\n", fp)
		return 0
	case frictions.AlreadyFixed:
		fmt.Fprintln(stdout, "already fixed")
		return 0
	default:
		fmt.Fprintf(stderr, "huella desconocida: %s\n", fp)
		return 2
	}
}

func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}
