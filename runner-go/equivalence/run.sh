#!/usr/bin/env bash
# Arnés de equivalencia runner-Go vs runner-Python.
#
# Compara por valor (no por prosa) el contrato --json y el efecto de la misma
# secuencia de fases: next, skip, reset, boot/start y el terminal. Después
# migra un fixture legado, recorre un ciclo deck_* completo, exporta de vuelta
# y comprueba con el validador del ledger del Python que el rollback es válido.
#
# Uso: ./run.sh              (deja el trabajo en un directorio temporal)
#      WORK=/tmp/x ./run.sh  (usa un directorio concreto)
#      GO_BIN=/ruta/bin ./run.sh
#
# NO modificar una vez verde: es el aceite de sustitución reversible de WP-05.
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo=$(cd "$here/../.." && pwd)
work=${WORK:-$(mktemp -d "${TMPDIR:-/tmp}/wp04-equivalence.XXXXXX")}
mkdir -p "$work"
bin=${GO_BIN:-$work/orchestrator-runner}
if [ -z "${GO_BIN:-}" ]; then
  (cd "$repo/runner-go" && go build -o "$bin" ./cmd/orchestrator-runner)
fi
# La copia pinned es la baseline histórica del arnés: se usa cuando los
# artefactos ya no viven en agents/. Orden: PY explícita → pinned → agents/ → instalación.
resolve_py() { # nombre del artefacto
  local name=$1 cand
  for cand in "$here/fixtures/pinned/$name" "$repo/agents/$name" "$HOME/.config/opencode/agents/$name"; do
    if [ -f "$cand" ]; then printf '%s\n' "$cand"; return 0; fi
  done
  echo "error: no se encuentra $name en pinned, agents/ ni la instalación" >&2
  return 1
}
python_runner=${PY:-$(resolve_py orchestrator-runner.py)}
fixtures="$here/fixtures"
session=ses_equiv

py_dir="$work/py"
go_dir="$work/go"
mig_dir="$work/mig"
mkdir -p "$py_dir" "$go_dir" "$mig_dir"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }
step() { echo; echo "== $*"; }

run_in() { # dir, output-name, session, stdin, command...
  local dir=$1 name=$2 sess=$3 input=$4
  shift 4
  set +e
  ( cd "$dir" && env ${sess:+OPENCODE_SESSION_ID="$sess"} "$@" ) \
    >"$work/$name.out" 2>"$work/$name.err" <<<"$input"
  echo $? >"$work/$name.rc"
  set -e
}

run_py() { local name=$1 dir=$2; shift 2; run_in "$dir" "py-$name" "" "" python3 "$python_runner" "$@"; }
run_go() { local name=$1 dir=$2; shift 2; run_in "$dir" "go-$name" "$session" "" "$bin" "$@"; }
go_aif_in() { local dir=$1 name=$2 json=$3; run_in "$dir" "go-$name" "$session" "$json" "$bin" aif; }
go_aif() { go_aif_in "$go_dir" "$1" "$2"; }

rc() { cat "$work/$1.rc"; }
assert_ok() { [ "$(rc "$1")" = 0 ] || fail "$1: código $(rc "$1") (stderr: $(head -c 300 "$work/$1.err"))"; }
assert_err() { [ "$(rc "$1")" != 0 ] || fail "$1: se esperaba error y salió 0"; }

snap() { # label
  run_py "json-$1" "$py_dir" --json
  run_go "json-$1" "$go_dir" --json
  assert_ok "py-json-$1"
  assert_ok "go-json-$1"
  python3 "$here/compare_state.py" "$work/py-json-$1.out" "$work/go-json-$1.out" "$1" \
    || fail "estado divergente en $1"
}

json_field() { # file, python expression over d
  python3 - "$1" "$2" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
print(eval(sys.argv[2], {"d": d}))
PY
}

json_assert() { # file, expression, message
  python3 - "$1" "$2" "$3" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
if not eval(sys.argv[2], {"d": d}):
    raise SystemExit("aserción fallida: %s (%s)" % (sys.argv[3], sys.argv[2]))
PY
}

codes=""
record_codes() { codes="${codes}$1: py=$(rc "py-$1") go=$(rc "go-$1")"$'\n'; }

step "1. estado inicial: --json sin base"
snap inicio

step "2. next con evidencia"
run_py next1 "$py_dir" next "evidencia de la fase 1"
go_aif next1 '{"aif":"0.2","operation":"phase_next","input":{"evidence":"evidencia de la fase 1"}}'
record_codes next1
assert_ok py-next1
assert_ok go-next1
snap tras-next

step "3. skip hacia delante con motivo"
run_py skip4 "$py_dir" skip 4 "motivo del salto"
go_aif skip4 '{"aif":"0.2","operation":"phase_skip","input":{"target":4,"reason":"motivo del salto"}}'
record_codes skip4
assert_ok py-skip4
assert_ok go-skip4
snap tras-skip

step "4. reset"
run_py reset "$py_dir" reset
go_aif reset '{"aif":"0.2","operation":"phase_reset","input":{"reason":"reinicio de la prueba"}}'
record_codes reset
assert_ok py-reset
assert_ok go-reset
snap tras-reset

step "5. transiciones inválidas: el estado no cambia"
run_py badskip "$py_dir" skip 2 "salto que no omite"
go_aif badskip '{"aif":"0.2","operation":"phase_skip","input":{"target":2,"reason":"salto que no omite"}}'
record_codes badskip
assert_err py-badskip
assert_err go-badskip
run_py badnext "$py_dir" next ""
go_aif badnext '{"aif":"0.2","operation":"phase_next","input":{"evidence":""}}'
record_codes badnext
assert_err py-badnext
assert_err go-badnext
snap tras-invalidas

step "6. boot (Python) / start (Go): degradan sin fallar y no mutan"
run_py boot "$py_dir" boot
run_go boot "$go_dir" start
record_codes boot
assert_ok py-boot
assert_ok go-boot
grep -q "## Fase" "$work/py-boot.out" || fail "boot sin sección de fase"
grep -q "## Fase" "$work/go-boot.out" || fail "start sin sección de fase"
grep -qi "fase 1" "$work/py-boot.out" || fail "boot no muestra fase 1"
grep -qi "fase 1" "$work/go-boot.out" || fail "start no muestra fase 1"
snap tras-boot

step "7. fase terminal: no avanza y conserva el estado"
run_py skip11 "$py_dir" skip 11 "salto al final de la prueba"
go_aif skip11 '{"aif":"0.2","operation":"phase_skip","input":{"target":11,"reason":"salto al final de la prueba"}}'
record_codes skip11
assert_ok py-skip11
assert_ok go-skip11
snap tras-skip11
run_py terminal "$py_dir" next "prueba de la fase terminal"
go_aif terminal '{"aif":"0.2","operation":"phase_next","input":{"evidence":"prueba de la fase terminal"}}'
record_codes terminal
assert_ok py-terminal
assert_ok go-terminal
snap tras-terminal
grep -q "Última fase" "$work/py-terminal.out" || fail "Python no avisa del terminal"
json_assert "$work/go-terminal.out" 'd["outcome"]["terminal"] is True' "Go no marca terminal"

step "8. diff de estado post-secuencia"
# La estructura completa (version/phases/state) a través de compare_state.py,
# que contrasta la estructura de fases y declara la divergencia de textos de
# pasos (la fase 1 ya no cita al runner Python: DESIGN §8, arnés).
python3 "$here/compare_state.py" "$work/py-json-tras-terminal.out" "$work/go-json-tras-terminal.out" "post-secuencia" || fail "el estado final difiere"
ok "estructura equivalente post-secuencia (divergencias de prosa declaradas)"

step "9. migración del fixture legado y ciclo deck_* completo"
mkdir -p "$mig_dir/.run"
cp "$fixtures/legacy-state.json" "$mig_dir/.run/orchestrator-runner.json"
cp "$fixtures/legacy-delegations.jsonl" "$mig_dir/.run/delegations.jsonl"

run_go before-mig "$mig_dir" --json
assert_ok go-before-mig
json_assert "$work/go-before-mig.out" 'd["state"]["exists"] is False and d["state"]["phase"] == 1' \
  "--json antes de mutar debía devolver el estado vacío sin crear base"
[ ! -e "$mig_dir/.run/orchestrator.db" ] || fail "--json creó la base"
echo "divergencia declarada: --json antes de la primera mutación no lee el legado (Python ya vería la fase 3)"

go_aif_in "$mig_dir" after-mig '{"aif":"0.2","operation":"phase_next","input":{"evidence":"primer mutador tras migrar"}}'
assert_ok go-after-mig
[ -f "$mig_dir/.run/orchestrator-runner.json.migrated" ] || fail "no se renombró el estado legado"
[ -f "$mig_dir/.run/delegations.jsonl.migrated" ] || fail "no se renombró el ledger legado"
run_go json "$mig_dir" --json
json_assert "$work/go-json.out" 'd["state"]["phase"] == 4 and len(d["state"]["evidence"]) == 3' \
  "la migración no conservó fase y evidencia"
go_aif_in "$mig_dir" list '{"aif":"0.2","operation":"deck_list","input":{"state":"all"}}'
json_assert "$work/go-list.out" 'len(d["outcome"]["delegations"]) == 5' "no se importaron las 5 delegaciones"

go_aif_in "$mig_dir" prepare '{"aif":"0.2","operation":"deck_prepare","input":{"agent":"general","task":"tk-wp04","goal":"Ciclo completo en sandbox","scope":["runner-go/equivalence"],"checks":["go test ./...","grep -c TODO runner-go/internal/store/export.go"],"criteria":"El ciclo queda registrado y exportable.","request_id":"req-prepare-1"}}'
assert_ok go-prepare
json_assert "$work/go-prepare.out" 'd["result"] == "executed" and d["outcome"]["state"] == "prepared"' \
  "deck_prepare no quedó prepared"
id=$(json_field "$work/go-prepare.out" 'd["outcome"]["id"]')
[ -n "$id" ] || fail "deck_prepare no devolvió id"

go_aif_in "$mig_dir" launch "{\"aif\":\"0.2\",\"operation\":\"deck_launch\",\"input\":{\"id\":\"$id\",\"subagent_session\":\"ses_sub_wp04\"}}"
assert_ok go-launch
json_assert "$work/go-launch.out" 'd["outcome"]["state"] == "launched"' "deck_launch no quedó launched"

go_aif_in "$mig_dir" return-partial "{\"aif\":\"0.2\",\"operation\":\"deck_return\",\"input\":{\"id\":\"$id\",\"status\":\"partial\",\"checks\":[{\"check\":\"go test ./...\",\"observed\":\"ok 7 paquetes\"},{\"check\":\"grep -c TODO runner-go/internal/store/export.go\",\"failed\":\"no ejecutado: presupuesto agotado\"}],\"risks\":\"la segunda comprobación queda pendiente\"}}"
assert_ok go-return-partial
json_assert "$work/go-return-partial.out" 'd["outcome"]["state"] == "returned" and len(d["outcome"]["pendientes"]) == 1' \
  "el retorno parcial no devolvió su pendiente"

go_aif_in "$mig_dir" return-fix "{\"aif\":\"0.2\",\"operation\":\"deck_return\",\"input\":{\"id\":\"$id\",\"status\":\"completed\",\"checks\":[{\"check\":\"grep -c TODO runner-go/internal/store/export.go\",\"observed\":\"0\"}],\"request_id\":\"req-return-2\"}}"
assert_ok go-return-fix
json_assert "$work/go-return-fix.out" 'd["outcome"]["returns"] == 2 and d["outcome"]["pendientes"] == []' \
  "la corrección no conservó la versión anterior y cerró los pendientes"

go_aif_in "$mig_dir" close "{\"aif\":\"0.2\",\"operation\":\"deck_close\",\"input\":{\"id\":\"$id\",\"outcome\":\"done\"}}"
assert_ok go-close
json_assert "$work/go-close.out" 'd["outcome"]["state"] == "closed" and d["outcome"]["outcome"] == "done"' \
  "deck_close no cerró"
go_aif_in "$mig_dir" list-final '{"aif":"0.2","operation":"deck_list","input":{"state":"all"}}'
json_assert "$work/go-list-final.out" 'len(d["outcome"]["delegations"]) == 6' "el ciclo no dejó 6 delegaciones"

step "10. export-legacy y rollback"
run_go export "$mig_dir" export-legacy
assert_ok go-export
json_assert "$work/go-export.out" 'd["delegations"] == 6 and len(d["limitations"]) > 0' \
  "export-legacy no declaró las 6 delegaciones y sus limitaciones"
python3 "$here/validate_legacy.py" "$mig_dir/.run/delegations.jsonl" || fail "el ledger exportado no es válido para el Python"
python3 - "$mig_dir/.run/delegations.jsonl" <<'PY'
import json, sys
rows = [json.loads(line) for line in open(sys.argv[1], encoding="utf-8") if line.strip()]
assert len(rows) == 6, len(rows)
assert sum(1 for row in rows if row["task"] is None) == 1, "la tarea sin referencia debía exportarse como null"
assert {row["status"] for row in rows} <= {"launched", "completed", "blocked"}
assert any(row.get("note") == "sin acceso al fixture del Python" for row in rows), "se perdió la nota legada"
PY

run_py json "$mig_dir" --json
assert_ok py-json
run_go json "$mig_dir" --json
python3 "$here/compare_state.py" "$work/py-json.out" "$work/go-json.out" "post-export" \
  || fail "el estado exportado no coincide con el de Go"
run_py next "$mig_dir" next "continuación con el Python tras el rollback"
assert_ok py-next
run_py after "$mig_dir" --json
json_assert "$work/py-after.out" 'd["state"]["phase"] == 5 and len(d["state"]["evidence"]) == 4' \
  "el Python no continuó desde el estado exportado"
python3 "$here/validate_legacy.py" "$mig_dir/.run/delegations.jsonl" >/dev/null \
  || fail "el ledger dejó de ser válido tras continuar con el Python"
echo "divergencia declarada: tras exportar y escribir el Python, la base Go no reimporta (marker migrated_legacy); la vuelta al Go exige una migración nueva. Punto de conmutación único."

step "RESUMEN"
echo "work: $work"
echo "códigos de salida por paso (Python/Go):"
printf '%s' "$codes" | sed 's/^/  /'
echo
echo "PASS: secuencia equivalente, migración y rollback válidos"
