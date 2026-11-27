#!/usr/bin/env bash
# Métricas de ruido por operación clave: bytes y líneas de la respuesta (stdout)
# del binario nuevo frente al runner Python, medidos sobre sandboxes limpios.
# Metodología WP-01: una invocación por operación, misma entrada lógica, mismas
# cifras de bytes y líneas; la prosa no se compara.
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo=$(cd "$here/../.." && pwd)
work=${WORK:-$(mktemp -d "${TMPDIR:-/tmp}/wp04-metrics.XXXXXX")}
mkdir -p "$work"
bin=${GO_BIN:-$work/orchestrator-runner}
if [ -z "${GO_BIN:-}" ]; then
  (cd "$repo/runner-go" && go build -o "$bin" ./cmd/orchestrator-runner)
fi
# Copia pinned como baseline cuando los artefactos ya no viven en agents/.
# Orden: variable explícita → pinned → agents/ → instalación.
resolve_py() { # nombre del artefacto
  local name=$1 cand
  for cand in "$here/fixtures/pinned/$name" "$repo/agents/$name" "$HOME/.config/opencode/agents/$name"; do
    if [ -f "$cand" ]; then printf '%s\n' "$cand"; return 0; fi
  done
  echo "error: no se encuentra $name en pinned, agents/ ni la instalación" >&2
  return 1
}
python_runner=${PY:-$(resolve_py orchestrator-runner.py)}
checker=${PY_CHECKER:-$(resolve_py delegation-report-checker.py)}
session=ses_metrics
rows="$work/metrics.tsv"
: >"$rows"

mkdir -p "$work"/{go-disc,py-json,go-next,py-next,go-start,py-boot,go-deck}
go() { local dir=$1; shift; ( cd "$work/$dir" && OPENCODE_SESSION_ID="$session" "$bin" "$@" ); }
aif() { local dir=$1 input=$2; ( cd "$work/$dir" && OPENCODE_SESSION_ID="$session" "$bin" aif ) <<<"$input"; }
py() { local dir=$1; shift; ( cd "$work/$dir" && python3 "$python_runner" "$@" ); }

row() { # op, go-bytes, go-lines, py-op, py-bytes, py-lines, note
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$@" >>"$rows"
}
size() { wc -c <"$1"; }
lines() { wc -l <"$1"; }

# Catálogo / descubrimiento
aif go-disc '{"aif":"0.2"}' >"$work/disc.out"
py py-json --json >"$work/pyjson.out"
row "discovery" "$(size "$work/disc.out")" "$(lines "$work/disc.out")" \
  "python --json" "$(size "$work/pyjson.out")" "$(lines "$work/pyjson.out")" \
  "Python no tiene descubrimiento AIF; --json es su catálogo"

# next / phase_next (fase 1 -> 2 con la misma evidencia)
go go-next aif <<<'{"aif":"0.2","operation":"phase_next","input":{"evidence":"métrica: evidencia de la fase 1"}}' >"$work/next-go.out"
py py-next next "métrica: evidencia de la fase 1" >"$work/next-py.out"
row "phase_next" "$(size "$work/next-go.out")" "$(lines "$work/next-go.out")" \
  "python next" "$(size "$work/next-py.out")" "$(lines "$work/next-py.out")" "misma transición"

# Arranque integrado
go go-start start >"$work/start.out"
py py-boot boot >"$work/boot.out"
row "start" "$(size "$work/start.out")" "$(lines "$work/start.out")" \
  "python boot" "$(size "$work/boot.out")" "$(lines "$work/boot.out")" "mismas secciones, cinco en Go (encargos)"

# Ciclo de delegación: las cuatro invocaciones del binario
prep='{"aif":"0.2","operation":"deck_prepare","input":{"agent":"general","task":"tk-metrics","goal":"medir el ciclo","scope":["runner-go/equivalence"],"checks":["go test ./...","grep -c TODO runner-go/internal/store/export.go"],"criteria":"medido"}}'
aif go-deck "$prep" >"$work/prepare.out"
id=$(python3 - "$work/prepare.out" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))["outcome"]["id"])
PY
)
aif go-deck "{\"aif\":\"0.2\",\"operation\":\"deck_launch\",\"input\":{\"id\":\"$id\",\"subagent_session\":\"ses_sub_metrics\"}}" >"$work/launch.out"
aif go-deck "{\"aif\":\"0.2\",\"operation\":\"deck_return\",\"input\":{\"id\":\"$id\",\"status\":\"partial\",\"checks\":[{\"check\":\"go test ./...\",\"observed\":\"ok\"},{\"check\":\"grep -c TODO runner-go/internal/store/export.go\",\"failed\":\"no ejecutado\"}],\"risks\":\"presupuesto\"}}" >"$work/return.out"
aif go-deck "{\"aif\":\"0.2\",\"operation\":\"deck_close\",\"input\":{\"id\":\"$id\",\"outcome\":\"done\"}}" >"$work/close.out"

# Baseline Python del retorno: el checker también procesa la forma del informe.
printf 'go test ./...\ngrep -c TODO runner-go/internal/store/export.go\n' >"$work/checks.txt"
cat >"$work/report.md" <<'MD'
status: partial
validation
go test ./...: ok
grep -c TODO runner-go/internal/store/export.go: no ejecutado — presupuesto
MD
python3 "$checker" --json --checks "$work/checks.txt" "$work/report.md" >"$work/checker.out"

manual_bytes=$(head -n 1 "$here/fixtures/legacy-delegations.jsonl" | wc -c)
row "deck_prepare" "$(size "$work/prepare.out")" "$(lines "$work/prepare.out")" \
  "sin operación" "-" "-" "Python escribe a mano una línea de ledger (~$manual_bytes B)"
row "deck_launch" "$(size "$work/launch.out")" "$(lines "$work/launch.out")" \
  "sin operación" "-" "-" "Python no tiene operación de lanzamiento"
row "deck_return" "$(size "$work/return.out")" "$(lines "$work/return.out")" \
  "checker --json" "$(size "$work/checker.out")" "$(lines "$work/checker.out")" "el checker no clasifica checks ni devuelve pendientes"
row "deck_close" "$(size "$work/close.out")" "$(lines "$work/close.out")" \
  "sin operación" "-" "-" "Python actualiza el status de la línea a mano"

go_cycle=$(( $(size "$work/prepare.out") + $(size "$work/launch.out") + $(size "$work/return.out") + $(size "$work/close.out") ))
go_cycle_lines=$(( $(lines "$work/prepare.out") + $(lines "$work/launch.out") + $(lines "$work/return.out") + $(lines "$work/close.out") ))
py_cycle=$(( manual_bytes + $(size "$work/checker.out") ))
py_cycle_lines=$(( 1 + $(lines "$work/checker.out") ))
row "ciclo deck_*" "$go_cycle" "$go_cycle_lines" \
  "ledger+checker" "$py_cycle" "$py_cycle_lines" "Go: 4 invocaciones; Python: 2 ediciones + 1 checker"

python3 - "$rows" <<'PY'
import sys
rows = [line.rstrip("\n").split("\t") for line in open(sys.argv[1], encoding="utf-8") if line.strip()]
print()
print("| operación | Go bytes | Go líneas | baseline Python | Py bytes | Py líneas | nota |")
print("|---|---:|---:|---|---:|---:|---|")
for op, gb, gl, pop, pb, pl, note in rows:
    print("| %s | %s | %s | %s | %s | %s | %s |" % (op, gb, gl, pop, pb, pl, note))
PY
echo
echo "datos crudos: $rows"
