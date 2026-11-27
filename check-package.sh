#!/usr/bin/env bash
# Puerta de verificación del paquete: ejecuta de una vez la batería que CONTRIBUTING.md
# describe en prosa. No pide interacción y solo escribe en directorios temporales.
set -u

here="$(cd "$(dirname "$0")" && pwd)"
cd "$here" || exit 1

green=''
red=''
reset=''
if [ -t 1 ]; then
  green=$'\033[32m'
  red=$'\033[31m'
  reset=$'\033[0m'
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

failures=0

step() { printf '%s... ' "$1"; }
ok() { printf '%sverde%s\n' "$green" "$reset"; }
ko() {
  printf '%srojo%s\n' "$red" "$reset"
  failures=$((failures + 1))
}

# Binario del runner para toda la batería. La instalación limpia lo recibe por
# RUNNER_BIN para no depender de la red; sin go, los grupos Go se avisan omitidos.
runner_bin="$tmp/orchestrator-runner"
runner_built=''
if command -v go >/dev/null 2>&1; then
  step "compilación del binario runner-go"
  if (cd runner-go && go build -buildvcs=false -o "$runner_bin" ./cmd/orchestrator-runner); then
    ok
    runner_built=1
  else
    ko
  fi
else
  printf 'aviso: sin go en PATH; los grupos Go y la compilación se omiten\n'
fi
if [ -n "$runner_built" ]; then export RUNNER_BIN="$runner_bin"; fi

# a2. Sintaxis y vet del módulo Go, si hay toolchain.
if command -v go >/dev/null 2>&1; then
  step "sintaxis y vet del runner Go (gofmt -l, go vet ./...)"
  if [ -z "$(cd runner-go && gofmt -l .)" ] && (cd runner-go && go vet ./...); then ok; else ko; fi
fi

# a. Sintaxis del instalador de Linux.
step "sintaxis de install.sh (bash -n)"
if bash -n install.sh; then ok; else ko; fi

# b. La plantilla carga como JSON estricto.
step "config/opencode.jsonc es JSON estricto"
if python3 -c "import json; json.load(open('config/opencode.jsonc'))" >/dev/null 2>&1; then
  ok
else
  ko
  echo "    el fichero no carga como JSON estricto"
fi

# c. Instalación limpia: agentes, herramienta del flujo y configuración en un HOME vacío.
step "instalación limpia en un HOME temporal (agentes, herramienta y configuración)"
home_c="$tmp/home-clean"
mkdir -p "$home_c"
if env -u XDG_CONFIG_HOME HOME="$home_c" bash install.sh >"$tmp/clean.log" 2>&1; then
  cfg_c="$home_c/.config/opencode/opencode.jsonc"
  [ -f "$cfg_c" ] || cfg_c="$home_c/.config/opencode/opencode.json"
  if [ -d "$home_c/.config/opencode/agents" ] && [ -f "$cfg_c" ] &&
    [ -x "$home_c/.config/opencode/agents/orchestrator-runner" ] &&
    [ ! -e "$home_c/.config/opencode/agents/orchestrator-runner.py" ] &&
    python3 - "$cfg_c" <<'PY'
import json, sys
data = json.load(open(sys.argv[1], encoding="utf-8"))
agents = data.get("agents") or {}
# Gana la última regla que casa: no basta con que el deny esté, tiene que ser el último.
for name in ("explore", "general"):
    perms = (agents.get(name) or {}).get("permissions") or []
    skills = [p for p in perms if isinstance(p, dict) and p.get("action") == "skill"]
    assert skills, name
    assert skills[-1].get("effect") == "deny", name
PY
  then
    ok
  else
    ko
    sed 's/^/    /' "$tmp/clean.log"
  fi
else
  ko
  sed 's/^/    /' "$tmp/clean.log"
fi

# c2. El plugin de servidor de la fase viaja en la instalación limpia de c: vive en
# plugins/ y NO se registra en cli.json —lo carga el servidor, no la TUI—.
step "instalación limpia: plugin de servidor orchestrator-phase en plugins/"
server_plugin_c="$home_c/.config/opencode/plugins/orchestrator-phase.ts"
if [ -f "$server_plugin_c" ] &&
  cmp -s plugins/orchestrator-phase.ts "$server_plugin_c" &&
  grep -Fq "experimental.chat.system.transform" "$server_plugin_c" &&
  grep -Fq "orchestrator-runner" "$server_plugin_c" &&
  ! grep -Fq "python3" "$server_plugin_c" &&
  python3 -c "import json, sys; assert not any('orchestrator-phase' in p for p in json.load(open(sys.argv[1], encoding='utf-8')).get('plugins') or [])" "$home_c/.config/opencode/cli.json"
then ok; else ko; echo "    falta $server_plugin_c, difiere del paquete, no cita el hook y el binario, menciona python3 o cli.json lo registra"; fi

# c3. El comando nativo /odd-fix viaja en la instalación limpia de c y su contrato se
# limita a lo pedido: selecciona al orquestador, usa $ARGUMENTS y no autoriza commits.
step "instalación limpia: comando odd-fix en commands/ con su contrato"
cmd_c="$home_c/.config/opencode/commands/odd-fix.md"
if [ -f "$cmd_c" ] && cmp -s commands/odd-fix.md "$cmd_c" &&
  python3 - "$cmd_c" <<'PY'
import re, sys
text = open(sys.argv[1], encoding="utf-8").read()
m = re.match(r"^---\n(.*?)\n---\n", text, re.S)
assert m, "sin frontmatter"
fm = m.group(1)
assert re.search(r"(?m)^agent:\s*orchestrator\s*$", fm), "agent distinto de orchestrator"
assert re.search(r"(?m)^subagent:\s*false\s*$", fm), "subagent distinto de false"
assert re.search(r"(?m)^description:\s*\S", fm), "sin descripción"
assert "$ARGUMENTS" in text, "no usa $ARGUMENTS"
assert re.search(r"(?i)confirmaci[oó]n expl[ií]cita", text), "no exige confirmación explícita"
PY
then ok; else ko; echo "    falta $cmd_c, difiere del paquete o incumple frontmatter/argumentos/confirmación"; fi

# c4. El comando `runner` queda disponible en el PATH del HOME de la instalación
# limpia: `command -v` lo resuelve y `runner --version` identifica el binario.
step "instalación limpia: comando runner en ~/.local/bin y resoluble por PATH"
runner_cmd_c="$home_c/.local/bin/runner"
if [ -x "$runner_cmd_c" ] &&
  env PATH="$home_c/.local/bin:$PATH" sh -c 'command -v runner' | grep -Fq "$runner_cmd_c" &&
  env PATH="$home_c/.local/bin:$PATH" sh -c 'runner --version' 2>&1 | grep -Eq '^orchestrator-runner [^ ]+ '; then
  ok
else
  ko
  echo "    falta $runner_cmd_c o no se resuelve por PATH/--version"
fi

# c5. El comando runner no sobrescribe un comando ajeno y la segunda pasada es
# idempotente: el fichero ajeno sigue igual y no nace copia .bak.
step "runner: comando ajeno no sobrescrito y segunda pasada idempotente"
home_r="$tmp/home-runner"
mkdir -p "$home_r/.local/bin"
printf '#!/bin/sh\necho foreign\n' >"$tmp/foreign-runner"
cp "$tmp/foreign-runner" "$home_r/.local/bin/runner"
if env -u XDG_CONFIG_HOME HOME="$home_r" bash install.sh >"$tmp/runner-foreign.log" 2>&1 &&
  cmp -s "$tmp/foreign-runner" "$home_r/.local/bin/runner" &&
  grep -Fq "no se sobrescribe" "$tmp/runner-foreign.log" &&
  env -u XDG_CONFIG_HOME HOME="$home_r" bash install.sh >>"$tmp/runner-foreign.log" 2>&1 &&
  cmp -s "$tmp/foreign-runner" "$home_r/.local/bin/runner" &&
  ! ls "$home_r"/.local/bin/runner.bak.* >/dev/null 2>&1; then
  ok
else
  ko
  sed 's/^/    /' "$tmp/runner-foreign.log"
fi

# d. Fusión sobre una configuración con claves ajenas.
step "fusión con claves ajenas (share, mcp) y copia .bak"
home_d="$tmp/home-merge"
mkdir -p "$home_d/.config/opencode"
printf '%s\n' '{"share":"disabled","mcp":{"algo":{}}}' >"$home_d/.config/opencode/opencode.json"
if env -u XDG_CONFIG_HOME HOME="$home_d" bash install.sh >"$tmp/merge.log" 2>&1 &&
  python3 - "$home_d/.config/opencode/opencode.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1], encoding="utf-8"))
assert data.get("share") == "disabled"
assert "algo" in data.get("mcp", {})
agents = data.get("agents") or {}
assert "explore" in agents and "general" in agents
for name in ("explore", "general"):
    perms = agents[name].get("permissions") or []
    skills = [p for p in perms if isinstance(p, dict) and p.get("action") == "skill"]
    assert skills, name
    assert skills[-1].get("effect") == "deny", name
PY
then
  if ls "$home_d"/.config/opencode/opencode.json.bak.20* >/dev/null 2>&1; then
    ok
  else
    ko
    echo "    no nació la copia .bak con fecha"
  fi
else
  ko
  sed 's/^/    /' "$tmp/merge.log"
fi

# e. Camino JSONC: fichero intacto y bloque manual impreso.
step "camino JSONC (fichero intacto y bloque manual)"
home_e="$tmp/home-jsonc"
mkdir -p "$home_e/.config/opencode"
cat >"$home_e/.config/opencode/opencode.json" <<'JSONC'
{
  // la fusión automática no puede leer comentarios
  "share": "disabled"
}
JSONC
cp "$home_e/.config/opencode/opencode.json" "$tmp/jsonc-original"
out_e="$(env -u XDG_CONFIG_HOME HOME="$home_e" bash install.sh 2>&1)"
rc_e=$?
if [ "$rc_e" -eq 0 ] &&
  cmp -s "$tmp/jsonc-original" "$home_e/.config/opencode/opencode.json" &&
  printf '%s' "$out_e" | grep -Fq "Añade a mano"; then
  ok
else
  ko
  echo "    código $rc_e; ¿fichero intacto? $(cmp -s "$tmp/jsonc-original" "$home_e/.config/opencode/opencode.json" && echo sí || echo no)"
fi

# f. Los agentes del paquete coinciden con los instalados en esta máquina.
# -x '*.bak.*': el instalador deja copias .bak.<fecha> en esa carpeta (R7) y el
# diff literal las marcaría siempre; contrasta agentes, no copias de respaldo.
# -x de los dos artefactos retirados: una instalación anterior puede conservarlos
# hasta que se reinstale; el paquete ya no los distribuye y la batería no la toca.
cfgroot="${XDG_CONFIG_HOME:-$HOME/.config}/opencode"
if [ -d "$cfgroot/agents" ]; then
  step "diff de agents/ contra $cfgroot/agents"
  if diff -r -x '*.bak.*' -x 'orchestrator-runner.py' -x 'delegation-report-checker.py' -x 'orchestrator-runner' agents "$cfgroot/agents"; then ok; else ko; fi
else
  printf 'aviso: diff omitido, no hay agentes instalados en esta máquina\n'
fi

# g. Los cuatro subagentes de verificación y documentación deniegan el permiso de
# acción skill en su frontmatter. `tools` es campo V1 y se
# ignora en V2; el reparto se expresa con permisos.
step "los cuatro subagentes deniegan el permiso de acción skill"
missing_g=''
for name in verify review docs ui; do
  if ! python3 - "agents/$name.md" <<'PY'
import re, sys
text = open(sys.argv[1], encoding="utf-8").read()
m = re.match(r"^---\n(.*?)\n---\n", text, re.S)
fm = m.group(1) if m else ""
rule = r"(?m)^\s*-\s*action:\s*skill\s*\n\s*resource:\s*[\"']?\*[\"']?\s*\n\s*effect:\s*deny\s*$"
raise SystemExit(0 if re.search(rule, fm) else 1)
PY
  then
    missing_g="$missing_g $name"
  fi
done
if [ -z "$missing_g" ]; then ok; else ko; echo "    sin la regla que deniega skill:$missing_g"; fi

# h. Ni los agentes ni la configuración abren el permiso de acción skill en allow o ask:
# el deny es lo correcto y oculta la skill al modelo. Gana la última regla que casa, así
# que un ask posterior en la plantilla dejaría el deny inerte sin que la batería lo viera.
step "ningún agente abre el permiso de acción skill (allow/ask)"
if offenders_h="$(python3 - agents/*.md config/opencode.jsonc <<'PY'
import json, re, sys
bad = []
for path in sys.argv[1:]:
    if path.endswith(".jsonc"):
        data = json.load(open(path, encoding="utf-8"))
        for name, entry in (data.get("agents") or {}).items():
            skills = [r for r in (entry.get("permissions") or [])
                      if isinstance(r, dict) and r.get("action") == "skill"]
            if skills and skills[-1].get("effect") != "deny":
                bad.append(f"{path}: agents.{name} deja skill en {skills[-1].get('effect')}")
        for rule in data.get("permissions") or []:
            if isinstance(rule, dict) and rule.get("action") == "skill" and rule.get("effect") != "deny":
                bad.append(f"{path}: permiso global de skill en {rule.get('effect')}")
        continue
    text = open(path, encoding="utf-8").read()
    m = re.match(r"^---\n(.*?)\n---\n", text, re.S)
    fm = m.group(1) if m else ""
    for block in re.finditer(r"(?ms)^\s*-\s*action:\s*skill\b(.*?)(?=^\s*-\s*action:|\Z)", fm):
        effect = re.search(r"(?m)^\s*effect:\s*[\"']?([A-Za-z]+)", block.group(1))
        if effect and effect.group(1) in ("allow", "ask"):
            bad.append(f"{path}: efecto {effect.group(1)}")
print("\n".join(bad))
raise SystemExit(1 if bad else 0)
PY
)"; then
  ok
else
  ko
  printf '%s\n' "$offenders_h" | sed 's/^/    /'
fi

# i. Toda skill requerida por el paquete existe en una raíz de descubrimiento.
# Si la máquina no tiene ninguna raíz, se avisa: el paquete no posee la skill.
step "las skills requeridas existen en una raíz de descubrimiento"
required_skills="playwright-cli-usage software-testing"
roots_i=""
for root in "${XDG_CONFIG_HOME:-$HOME/.config}/opencode/skills" "$HOME/.agents/skills" "$HOME/.claude/skills"; do
  [ -d "$root" ] && roots_i="$roots_i $root"
done
if [ -z "$roots_i" ]; then
  printf 'aviso: esta máquina no tiene raíz de descubrimiento de skills; no se comprueban:%s\n' "$required_skills"
else
  missing_i=''
  for skill in $required_skills; do
    found=''
    for root in $roots_i; do
      if [ -f "$root/$skill/SKILL.md" ] || [ -f "$root/$skill.md" ]; then found=1; break; fi
    done
    [ -z "$found" ] && missing_i="$missing_i $skill"
  done
  if [ -z "$missing_i" ]; then ok; else ko; echo "    no encontradas en ninguna raíz:$missing_i"; fi
fi

# j. Contrato del binario Go: --json v1 (11 fases, solo lectura), catálogo AIF,
# transiciones y errores de entrada. Se ejecuta el binario compilado arriba; sin
# go, los grupos se avisan omitidos. Los casos de forma viven en go test (§n).
if [ -z "$runner_built" ]; then
  printf 'aviso: sin binario runner-go compilado; se omiten las comprobaciones del runner\n'
else
  session_j=ses_bateria

  step "runner --json: JSON válido, version 1 y las 11 fases"
  dir_j="$tmp/runner-json"
  mkdir -p "$dir_j"
  if (cd "$dir_j" && "$runner_bin" --json) >"$tmp/runner.json" 2>"$tmp/runner.err" &&
    [ ! -e "$dir_j/.run" ] &&
    python3 - "$tmp/runner.json" <<'PY'
import json, sys
doc = json.load(open(sys.argv[1], encoding="utf-8"))
assert doc["version"] == 1, doc["version"]
phases = doc["phases"]
assert [p["number"] for p in phases] == list(range(1, 12)), "fases fuera de orden"
for p in phases:
    assert set(p) == {"number", "name", "when", "evidence", "steps", "warnings"}, p["number"]
    assert p["name"] and p["when"] and p["evidence"], p["number"]
    assert isinstance(p["steps"], list) and p["steps"], p["number"]
    assert isinstance(p["warnings"], list), p["number"]
assert doc["state"] == {"exists": False, "phase": 1, "valid": True, "evidence_valid": True, "evidence": []}, doc["state"]
PY
  then ok; else ko; sed 's/^/    /' "$tmp/runner.err"; fi

  step "runner --json con sesión y sin base no crea ni modifica el estado"
  dir_k="$tmp/runner-json-sin-base"
  mkdir -p "$dir_k"
  if (cd "$dir_k" && env OPENCODE_SESSION_ID="$session_j" "$runner_bin" --json) >"$tmp/runner-vacio.json" 2>&1 &&
    [ ! -e "$dir_k/.run" ] &&
    python3 -c "import json, sys; s = json.load(open(sys.argv[1]))['state']; assert s['exists'] is False and s['phase'] == 1 and s['evidence'] == [], s" "$tmp/runner-vacio.json"
  then ok; else ko; echo "    el estado vacío no es el contrato o se creó .run/"; fi

  step "runner aif: phase_next registra evidencia y --json la devuelve"
  dir_l="$tmp/runner-next"
  mkdir -p "$dir_l"
  if printf '%s\n' '{"aif":"0.2","operation":"phase_next","input":{"evidence":"leído"}}' |
    (cd "$dir_l" && env OPENCODE_SESSION_ID="$session_j" "$runner_bin" aif) >"$tmp/runner-next.out" 2>&1 &&
    (cd "$dir_l" && env OPENCODE_SESSION_ID="$session_j" "$runner_bin" --json) >"$tmp/runner-next.json" 2>&1 &&
    python3 - "$tmp/runner-next.json" <<'PY'
import json, sys
s = json.load(open(sys.argv[1], encoding="utf-8"))["state"]
assert s["exists"] is True and s["phase"] == 2, s
assert s["evidence"] == [{"phase": 1, "text": "leído"}], s["evidence"]
PY
  then ok; else ko; sed 's/^/    /' "$tmp/runner-next.out"; fi

  step "runner aif: phase_next sin evidencia es invalid (código 2) y no avanza"
  dir_m="$tmp/runner-next-invalido"
  mkdir -p "$dir_m"
  printf '%s\n' '{"aif":"0.2","operation":"phase_next","input":{"evidence":""}}' |
    (cd "$dir_m" && env OPENCODE_SESSION_ID="$session_j" "$runner_bin" aif) >"$tmp/runner-next-invalido.out" 2>&1
  rc_j=$?
  if [ "$rc_j" -eq 2 ] && [ ! -e "$dir_m/.run" ] &&
    python3 -c "import json, sys; d = json.load(open(sys.argv[1])); assert d['result'] == 'invalid', d" "$tmp/runner-next-invalido.out"
  then ok; else ko; echo "    código $rc_j"; sed 's/^/    /' "$tmp/runner-next-invalido.out"; fi

  step "runner aif: phase_skip avanza con motivo, omite fases y conserva la evidencia"
  dir_n="$tmp/runner-skip"
  mkdir -p "$dir_n"
  if printf '%s\n' '{"aif":"0.2","operation":"phase_next","input":{"evidence":"leído"}}' |
    (cd "$dir_n" && env OPENCODE_SESSION_ID="$session_j" "$runner_bin" aif) >/dev/null 2>&1 &&
    printf '%s\n' '{"aif":"0.2","operation":"phase_skip","input":{"target":6,"reason":"sin decisiones ni delegación"}}' |
    (cd "$dir_n" && env OPENCODE_SESSION_ID="$session_j" "$runner_bin" aif) >"$tmp/runner-skip.out" 2>&1 &&
    python3 -c "import json, sys; d = json.load(open(sys.argv[1])); assert d['result'] == 'executed' and d['outcome']['phase'] == 6 and d['outcome']['skipped'] == [3, 4, 5], d" "$tmp/runner-skip.out" &&
    (cd "$dir_n" && env OPENCODE_SESSION_ID="$session_j" "$runner_bin" --json) >"$tmp/runner-skip.json" 2>&1 &&
    python3 - <<'PY' "$tmp/runner-skip.json"
import json, sys
s = json.load(open(sys.argv[1], encoding="utf-8"))["state"]
assert s["phase"] == 6, s
assert s["evidence"] == [
    {"phase": 1, "text": "leído"},
    {"phase": 2, "text": "sin decisiones ni delegación", "skip": 6},
], s["evidence"]
PY
  then ok; else ko; sed 's/^/    /' "$tmp/runner-skip.out"; fi

  step "runner aif: saltos inválidos son invalid (código 2) sin tocar el estado"
  dir_o="$tmp/runner-skip-invalido"
  mkdir -p "$dir_o"
  printf '%s\n' '{"aif":"0.2","operation":"phase_skip","input":{"target":6,"reason":"punto de partida"}}' |
    (cd "$dir_o" && env OPENCODE_SESSION_ID="$session_j" "$runner_bin" aif) >/dev/null 2>&1
  (cd "$dir_o" && env OPENCODE_SESSION_ID="$session_j" "$runner_bin" --json) >"$tmp/runner-antes.json" 2>&1
  skip_fails() { # envelope JSON
    printf '%s\n' "$1" | (cd "$dir_o" && env OPENCODE_SESSION_ID="$session_j" "$runner_bin" aif) >/dev/null 2>&1
    [ $? -eq 2 ]
  }
  if skip_fails '{"aif":"0.2","operation":"phase_skip","input":{"target":1,"reason":"x"}}' &&
    skip_fails '{"aif":"0.2","operation":"phase_skip","input":{"target":6,"reason":"x"}}' &&
    skip_fails '{"aif":"0.2","operation":"phase_skip","input":{"target":7,"reason":"x"}}' &&
    skip_fails '{"aif":"0.2","operation":"phase_skip","input":{"target":12,"reason":"x"}}' &&
    skip_fails '{"aif":"0.2","operation":"phase_skip","input":{"target":8,"reason":"   "}}' &&
    skip_fails '{"aif":"0.2","operation":"phase_skip","input":{"target":"ocho","reason":"x"}}' &&
    (cd "$dir_o" && env OPENCODE_SESSION_ID="$session_j" "$runner_bin" --json) >"$tmp/runner-despues.json" 2>&1 &&
    cmp -s "$tmp/runner-antes.json" "$tmp/runner-despues.json"
  then ok; else ko; echo "    algún salto inválido no salió con 2 o el estado cambió"; fi

  step "runner aif: la fase 11 es terminal y phase_reset vuelve a la fase 1"
  dir_p="$tmp/runner-terminal"
  mkdir -p "$dir_p"
  if printf '%s\n' '{"aif":"0.2","operation":"phase_skip","input":{"target":11,"reason":"prueba del terminal"}}' |
    (cd "$dir_p" && env OPENCODE_SESSION_ID="$session_j" "$runner_bin" aif) >/dev/null 2>&1 &&
    printf '%s\n' '{"aif":"0.2","operation":"phase_next","input":{"evidence":"texto"}}' |
    (cd "$dir_p" && env OPENCODE_SESSION_ID="$session_j" "$runner_bin" aif) >"$tmp/runner-terminal.out" 2>&1 &&
    python3 -c "import json, sys; d = json.load(open(sys.argv[1])); assert d['result'] == 'executed' and d['outcome']['terminal'] is True, d" "$tmp/runner-terminal.out" &&
    printf '%s\n' '{"aif":"0.2","operation":"phase_reset","input":{"reason":"fin de la prueba"}}' |
    (cd "$dir_p" && env OPENCODE_SESSION_ID="$session_j" "$runner_bin" aif) >/dev/null 2>&1 &&
    (cd "$dir_p" && env OPENCODE_SESSION_ID="$session_j" "$runner_bin" --json) >"$tmp/runner-reset.json" 2>&1 &&
    python3 -c "import json, sys; s = json.load(open(sys.argv[1]))['state']; assert s['exists'] is False and s['phase'] == 1, s" "$tmp/runner-reset.json"
  then ok; else ko; sed 's/^/    /' "$tmp/runner-terminal.out"; fi

  step "runner --json marca la base dañada con valid=false"
  dir_q="$tmp/runner-corrupto"
  mkdir -p "$dir_q/.run"
  printf 'esto no es una base sqlite\n' >"$dir_q/.run/orchestrator.db"
  if (cd "$dir_q" && env OPENCODE_SESSION_ID="$session_j" "$runner_bin" --json) >"$tmp/runner-corrupto.json" 2>&1 &&
    python3 -c "import json, sys; s = json.load(open(sys.argv[1]))['state']; assert s['exists'] is True and s['valid'] is False and s['phase'] == 1, s" "$tmp/runner-corrupto.json"
  then ok; else ko; fi

  step "runner --version identifica el ejecutable"
  if "$runner_bin" --version | grep -Eq '^orchestrator-runner [^ ]+ '; then ok; else ko; fi

  step "runner --help documenta el catálogo AIF, phase_next y los códigos de salida"
  if "$runner_bin" --help | grep -Fq "runner aif" &&
    "$runner_bin" --help | grep -Fq "phase_next" &&
    "$runner_bin" --help | grep -Fq "Códigos de salida"; then ok; else ko; fi

  step "runner con un subcomando desconocido sale con 1"
  "$runner_bin" inventado >/dev/null 2>"$tmp/runner-uso.err"
  rc_u=$?
  if [ "$rc_u" -eq 1 ] && grep -Fq "no reconocido" "$tmp/runner-uso.err"; then ok; else ko; echo "    código $rc_u"; fi
fi

# k. El plugin TUI viaja en la instalación limpia de c: sus fuentes, sus tipos y su
# registro en cli.json (un paquete solo-TUI no va en plugins/, el servidor colisiona).
step "instalación limpia: plugin orchestrator-status con sus fuentes"
plugin_c="$home_c/.config/opencode/cli-plugins/orchestrator-status"
missing_k=''
for f in tui.tsx package.json tsconfig.json types/opencode-plugin.d.ts; do
  [ -f "$plugin_c/$f" ] || missing_k="$missing_k $f"
done
if [ -z "$missing_k" ] &&
  ! grep -Fq "python3" "$plugin_c/tui.tsx" &&
  python3 -c "import json, sys; assert sys.argv[1] in json.load(open(sys.argv[2], encoding='utf-8'))['plugins']" "$plugin_c" "$home_c/.config/opencode/cli.json"
then ok; else ko; echo "    faltan en $plugin_c:$missing_k, menciona python3 o cli.json no lo registra"; fi

# l. Sin node_modules: las dependencias las resuelve OpenCode en runtime.
step "el plugin instalado no lleva node_modules"
if [ -d "$plugin_c" ] && [ ! -e "$plugin_c/node_modules" ]; then
  ok
else
  ko
  echo "    $plugin_c/node_modules existe o falta el plugin"
fi

# m. Instalación sobre un plugin previo: copia .bak.<fecha>, reemplazo y segunda
# pasada idempotente que no crea otra copia ni duplica el registro de cli.json.
step "plugin sobre instalación previa: copia .bak y segunda pasada idempotente"
home_m="$tmp/home-plugin-merge"
plugin_m="$home_m/.config/opencode/cli-plugins/orchestrator-status"
mkdir -p "$plugin_m"
printf '%s\n' '// versión anterior del plugin' >"$plugin_m/tui.tsx"
if env -u XDG_CONFIG_HOME HOME="$home_m" bash install.sh >"$tmp/plugin-merge.log" 2>&1 &&
  cmp -s plugins/orchestrator-status/tui.tsx "$plugin_m/tui.tsx" &&
  grep -qx '// versión anterior del plugin' "$plugin_m"/tui.tsx.bak.*
then
  copias_m="$(ls "$plugin_m"/tui.tsx.bak.* | wc -l)"
  registros_m="$(python3 -c "import json, sys; print(json.load(open(sys.argv[1], encoding='utf-8'))['plugins'].count(sys.argv[2]))" "$home_m/.config/opencode/cli.json" "$plugin_m")"
  if env -u XDG_CONFIG_HOME HOME="$home_m" bash install.sh >"$tmp/plugin-merge2.log" 2>&1 &&
    [ "$(ls "$plugin_m"/tui.tsx.bak.* | wc -l)" -eq "$copias_m" ] &&
    [ "$(python3 -c "import json, sys; print(json.load(open(sys.argv[1], encoding='utf-8'))['plugins'].count(sys.argv[2]))" "$home_m/.config/opencode/cli.json" "$plugin_m")" -eq "$registros_m" ]; then
    ok
  else
    ko
    echo "    la segunda pasada creó otra copia .bak o duplicó el registro"
    sed 's/^/    /' "$tmp/plugin-merge2.log"
  fi
else
  ko
  sed 's/^/    /' "$tmp/plugin-merge.log"
fi

# m2. Instalación sobre un plugin de servidor previo distinto: copia .bak.<fecha> y
# el contenido nuevo. Sin registro en cli.json, así que no hay nada que deduplicar.
step "plugin de servidor sobre instalación previa: copia .bak y contenido nuevo"
home_p="$tmp/home-phase-merge"
server_plugin_p="$home_p/.config/opencode/plugins/orchestrator-phase.ts"
mkdir -p "$home_p/.config/opencode/plugins"
printf '%s\n' '// versión anterior del plugin de fase' >"$server_plugin_p"
if env -u XDG_CONFIG_HOME HOME="$home_p" bash install.sh >"$tmp/phase-merge.log" 2>&1 &&
  cmp -s plugins/orchestrator-phase.ts "$server_plugin_p" &&
  grep -qx '// versión anterior del plugin de fase' "$server_plugin_p".bak.*
then ok; else ko; sed 's/^/    /' "$tmp/phase-merge.log"; fi

# m3. Instalación sobre un comando previo distinto: copia .bak.<fecha>, reemplazo y
# segunda pasada idempotente que no crea otra copia.
step "comando sobre instalación previa: copia .bak y segunda pasada idempotente"
home_cmd="$tmp/home-command-merge"
cmd_m="$home_cmd/.config/opencode/commands/odd-fix.md"
mkdir -p "$(dirname "$cmd_m")"
printf '%s\n' '--- versión anterior del comando' >"$cmd_m"
if env -u XDG_CONFIG_HOME HOME="$home_cmd" bash install.sh >"$tmp/command-merge.log" 2>&1 &&
  cmp -s commands/odd-fix.md "$cmd_m" &&
  grep -qx -- '--- versión anterior del comando' "$cmd_m".bak.*
then
  copias_cmd="$(ls "$cmd_m".bak.* | wc -l)"
  if env -u XDG_CONFIG_HOME HOME="$home_cmd" bash install.sh >>"$tmp/command-merge.log" 2>&1 &&
    [ "$(ls "$cmd_m".bak.* | wc -l)" -eq "$copias_cmd" ]; then
    ok
  else
    ko
    echo "    la segunda pasada creó otra copia .bak"
    sed 's/^/    /' "$tmp/command-merge.log"
  fi
else
  ko
  sed 's/^/    /' "$tmp/command-merge.log"
fi

# n. La validación del informe de retorno vive en internal/report: el grupo del
# checker Python se sustituye por la suite de equivalencia del módulo, sin duplicar
# casos; el binario y go test son la misma puerta.
if [ -n "$runner_built" ]; then
  step "checker: equivalencia del contrato de informe (go test ./internal/report)"
  if (cd runner-go && go test ./internal/report); then ok; else ko; fi

  step "runner-go: suite completa (go test ./...)"
  if (cd runner-go && go test ./...); then ok; else ko; fi

  # n+1. El ledger deja de ser producto manual: export-legacy regenera el formato
  # del Python desde la base y aquí se valida su forma (los seis campos de R11).
  step "ledger: export-legacy produce líneas válidas para el Python"
  dir_ledger="$tmp/ledger-export"
  mkdir -p "$dir_ledger"
  prep='{"aif":"0.2","operation":"deck_prepare","input":{"agent":"general","task":"tk-bateria","goal":"validar el ledger exportado","scope":["runner-go"],"checks":["go test ./..."],"criteria":"la línea exportada conserva la forma del Python"}}'
  if printf '%s\n' "$prep" |
    (cd "$dir_ledger" && env OPENCODE_SESSION_ID=ses_ledger "$runner_bin" aif) >"$tmp/ledger-prepare.json" 2>&1 &&
    id_ledger="$(python3 -c "import json, sys; print(json.load(open(sys.argv[1]))['outcome']['id'])" "$tmp/ledger-prepare.json")" &&
    printf '%s\n' "{\"aif\":\"0.2\",\"operation\":\"deck_launch\",\"input\":{\"id\":\"$id_ledger\"}}" |
    (cd "$dir_ledger" && env OPENCODE_SESSION_ID=ses_ledger "$runner_bin" aif) >/dev/null 2>&1 &&
    (cd "$dir_ledger" && env OPENCODE_SESSION_ID=ses_ledger "$runner_bin" export-legacy) >/dev/null 2>&1 &&
    python3 - "$dir_ledger/.run/delegations.jsonl" <<'PY'
import json, sys

STATUSES = {"launched", "completed", "blocked"}
FIELDS = ("ts", "phase", "task", "fingerprint", "agent", "status")
lines = [l for l in open(sys.argv[1], encoding="utf-8").read().splitlines() if l.strip()]
assert lines, "el ledger está vacío"
for n, line in enumerate(lines, 1):
    doc = json.loads(line)
    assert isinstance(doc, dict), "la línea %d no es un objeto JSON" % n
    missing = [f for f in FIELDS if f not in doc]
    assert not missing, "a la línea %d le faltan %s" % (n, missing)
    assert isinstance(doc["ts"], str) and doc["ts"].strip(), "ts vacío en la línea %d" % n
    assert isinstance(doc["phase"], int) and not isinstance(doc["phase"], bool), \
        "phase no es entero en la línea %d" % n
    assert doc["task"] is None or (isinstance(doc["task"], str) and doc["task"].strip()), \
        "task no es referencia o null en la línea %d" % n
    assert isinstance(doc["fingerprint"], str) and doc["fingerprint"].strip(), \
        "fingerprint vacío en la línea %d" % n
    assert isinstance(doc["agent"], str) and doc["agent"].strip(), \
        "agent vacío en la línea %d" % n
    assert doc["status"] in STATUSES, "status fuera del conjunto en la línea %d" % n
PY
  then ok; else ko; echo "    export-legacy no produjo un ledger con la forma del Python"; fi
else
  printf 'aviso: sin binario runner-go compilado; se omiten la equivalencia y el ledger\n'
fi


echo ""
if [ "$failures" -eq 0 ]; then
  printf '%sTodo en verde.%s\n' "$green" "$reset"
  exit 0
fi
printf '%s%d comprobación(es) en rojo.%s\n' "$red" "$failures" "$reset"
exit 1
