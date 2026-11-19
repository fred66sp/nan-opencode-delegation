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
    [ -x "$home_c/.config/opencode/agents/orchestrator-runner.py" ] &&
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
cfgroot="${XDG_CONFIG_HOME:-$HOME/.config}/opencode"
if [ -d "$cfgroot/agents" ]; then
  step "diff de agents/ contra $cfgroot/agents"
  if diff -r -x '*.bak.*' agents "$cfgroot/agents"; then ok; else ko; fi
else
  printf 'aviso: diff omitido, no hay agentes instalados en esta máquina\n'
fi

# g. Los cuatro subagentes de verificación y documentación deniegan el permiso de
# acción skill en su frontmatter; tasks-agent lo conserva. `tools` es campo V1 y se
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

# j. El flag --json del runner: JSON válido con las 11 fases y el estado, sin mutar nada,
# con la ayuda y el estado corrupto cubiertos. El resto de usos del runner no cambian.
runner="$here/agents/orchestrator-runner.py"
dir_j="$tmp/runner-json"
mkdir -p "$dir_j"

step "runner --json: JSON válido, version 1 y las 11 fases"
if (cd "$dir_j" && python3 "$runner" --json) >"$tmp/runner.json" 2>"$tmp/runner.err" &&
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

step "runner --json no crea ni modifica el estado"
dir_k="$tmp/runner-json-estado"
mkdir -p "$dir_k/.run"
printf '%s\n' '{"phase": 4, "evidence": [{"phase": 3, "text": "explorado"}]}' >"$dir_k/.run/orchestrator-runner.json"
cp "$dir_k/.run/orchestrator-runner.json" "$tmp/runner-estado-antes"
if (cd "$dir_k" && python3 "$runner" --json) >"$tmp/runner-estado.json" 2>&1 &&
  cmp -s "$tmp/runner-estado-antes" "$dir_k/.run/orchestrator-runner.json" &&
  python3 -c "import json, sys; d = json.load(open(sys.argv[1])); assert d['state']['exists'] is True and d['state']['phase'] == 4, d['state']" "$tmp/runner-estado.json"
then ok; else ko; echo "    el estado cambió o la fase no se conservó"; fi

step "runner --json con otro argumento es error de uso (código 1)"
(cd "$dir_j" && python3 "$runner" --json next) >/dev/null 2>"$tmp/runner-uso.err"
rc_j=$?
if [ "$rc_j" -eq 1 ] && grep -Fq "uso:" "$tmp/runner-uso.err"; then ok; else ko; echo "    código $rc_j"; fi

step "runner next --json no avanza ni escribe estado (código 1)"
(cd "$dir_j" && python3 "$runner" next --json) >/dev/null 2>"$tmp/runner-next-json.err"
rc_nj=$?
if [ "$rc_nj" -eq 1 ] && [ ! -e "$dir_j/.run" ]; then ok; else ko; echo "    código $rc_nj; ¿.run creado? $([ -e "$dir_j/.run" ] && echo sí || echo no)"; fi

step "runner --help documenta --json"
if python3 "$runner" --help | grep -Fq -- "--json"; then ok; else ko; fi

step "runner --json marca el estado corrupto con valid=false"
printf '%s\n' '{"phase": 99}' >"$dir_k/.run/orchestrator-runner.json"
if (cd "$dir_k" && python3 "$runner" --json) >"$tmp/runner-corrupto.json" 2>&1 &&
  python3 - "$tmp/runner-corrupto.json" <<'PY'
import json, sys
state = json.load(open(sys.argv[1], encoding="utf-8"))["state"]
assert state["exists"] is True and state["valid"] is False and state["phase"] == 1, state
PY
then ok; else ko; fi

step "runner sin argumentos, next y reset siguen funcionando"
dir_l="$tmp/runner-usos"
mkdir -p "$dir_l"
if (cd "$dir_l" && python3 "$runner") >/dev/null 2>&1 &&
  (cd "$dir_l" && python3 "$runner" next "texto de prueba") >/dev/null 2>&1 &&
  python3 -c "import json, sys; assert json.load(open(sys.argv[1]))['phase'] == 2" "$dir_l/.run/orchestrator-runner.json" &&
  (cd "$dir_l" && python3 "$runner" reset) >/dev/null 2>&1 &&
  [ ! -e "$dir_l/.run/orchestrator-runner.json" ]
then ok; else ko; echo "    un uso existente cambió de comportamiento"; fi

# k. El plugin TUI viaja en la instalación limpia de c: sus fuentes, sus tipos y su
# registro en cli.json (un paquete solo-TUI no va en plugins/, el servidor colisiona).
step "instalación limpia: plugin orchestrator-status con sus fuentes"
plugin_c="$home_c/.config/opencode/cli-plugins/orchestrator-status"
missing_k=''
for f in tui.tsx package.json tsconfig.json types/opencode-plugin.d.ts; do
  [ -f "$plugin_c/$f" ] || missing_k="$missing_k $f"
done
if [ -z "$missing_k" ] &&
  python3 -c "import json, sys; assert sys.argv[1] in json.load(open(sys.argv[2], encoding='utf-8'))['plugins']" "$plugin_c" "$home_c/.config/opencode/cli.json"
then ok; else ko; echo "    faltan en $plugin_c:$missing_k o cli.json no lo registra"; fi

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

# n. El comprobador del retorno estructurado: sintaxis, contrato en la ayuda, un informe
# conforme (código 0) y dos no conformes (código 1), con las fixtures en el temporal. El
# bytecode va a $tmp: un __pycache__ en agents/ haría saltar el diff de f.
checker="$here/agents/delegation-report-checker.py"
dir_n="$tmp/checker"
mkdir -p "$dir_n"
printf '%s\n' 'python3 -m py_compile agents/orchestrator-runner.py' 'bash check-package.sh' >"$dir_n/checks.txt"
cat >"$dir_n/conforme.txt" <<'REPORT'
status: completed

Se revisó el fichero agents/orchestrator-runner.py y compila.

validation:
  python3 -m py_compile agents/orchestrator-runner.py: sin salida, código 0
  bash check-package.sh: Todo en verde., código 0
REPORT
cat >"$dir_n/sin-status.txt" <<'REPORT'
validation:
  python3 -m py_compile agents/orchestrator-runner.py: sin salida, código 0
  bash check-package.sh: Todo en verde., código 0
REPORT
cat >"$dir_n/sin-comprobacion.txt" <<'REPORT'
status: partial

validation:
  bash check-package.sh: rojo, 2 comprobaciones fallidas
REPORT
cat >"$dir_n/contradicho.txt" <<'REPORT'
status: completed

validation:
  python3 -m py_compile agents/orchestrator-runner.py: sin salida, código 0
  bash check-package.sh: Todo en verde., código 0

status: partial
REPORT
cat >"$dir_n/anafora.txt" <<'REPORT'
status: completed

validation:
  python3 -m py_compile agents/orchestrator-runner.py: sin salida, código 0
  bash check-package.sh: Todo en verde., código 0

Se revisó el fichero y quedó listo.
REPORT

step "sintaxis de delegation-report-checker.py (py_compile)"
if PYTHONPYCACHEPREFIX="$dir_n/pyc" python3 -m py_compile "$checker"; then ok; else ko; fi

step "checker --help documenta --checks y los códigos de salida"
if python3 "$checker" --help | grep -Fq -- "--checks" &&
  python3 "$checker" --help | grep -Fq "Códigos de salida"; then ok; else ko; fi

step "checker: informe conforme devuelve 0"
if python3 "$checker" --checks "$dir_n/checks.txt" "$dir_n/conforme.txt" >"$dir_n/conforme.out" 2>&1 &&
  grep -Fq "conforme: status completed" "$dir_n/conforme.out"; then ok; else ko; sed 's/^/    /' "$dir_n/conforme.out"; fi

step "checker: informe sin status devuelve 1 y lo nombra"
python3 "$checker" --checks "$dir_n/checks.txt" "$dir_n/sin-status.txt" >"$dir_n/sin-status.out" 2>&1
rc_n=$?
if [ "$rc_n" -eq 1 ] && grep -Fq 'falta la línea' "$dir_n/sin-status.out"; then ok; else ko; echo "    código $rc_n"; sed 's/^/    /' "$dir_n/sin-status.out"; fi

step "checker: comprobación esperada ausente devuelve 1 y --json la nombra"
python3 "$checker" --json --checks "$dir_n/checks.txt" <"$dir_n/sin-comprobacion.txt" >"$dir_n/sin-comprobacion.json" 2>&1
rc_njson=$?
if [ "$rc_njson" -eq 1 ] && python3 - "$dir_n/sin-comprobacion.json" <<'PY'
import json, sys
doc = json.load(open(sys.argv[1], encoding="utf-8"))
assert doc["ok"] is False and doc["status"] == "partial", doc
assert any("orchestrator-runner.py" in v for v in doc["violations"]), doc["violations"]
PY
then ok; else ko; echo "    código $rc_njson"; sed 's/^/    /' "$dir_n/sin-comprobacion.json"; fi

step "checker: status contradictorios devuelve 1 y lo nombra"
python3 "$checker" --checks "$dir_n/checks.txt" "$dir_n/contradicho.txt" >"$dir_n/contradicho.out" 2>&1
rc_nc=$?
if [ "$rc_nc" -eq 1 ] && grep -Fq "contradictorios" "$dir_n/contradicho.out"; then ok; else ko; echo "    código $rc_nc"; sed 's/^/    /' "$dir_n/contradicho.out"; fi

step "checker: anáfora sin ruta devuelve 1 y la nombra"
python3 "$checker" --checks "$dir_n/checks.txt" "$dir_n/anafora.txt" >"$dir_n/anafora.out" 2>&1
rc_na=$?
if [ "$rc_na" -eq 1 ] && grep -Fq "referencia indefinida" "$dir_n/anafora.out"; then ok; else ko; echo "    código $rc_na"; sed 's/^/    /' "$dir_n/anafora.out"; fi

# n+1. Validación mecánica opcional del libro de delegaciones: si el proyecto tiene
# .run/delegations.jsonl, cada línea es un objeto JSON con los seis campos de R11 y
# valores en conjunto; si no lo tiene, el paso pasa.
step "ledger .run/delegations.jsonl: validación mecánica opcional"
if [ ! -f "$here/.run/delegations.jsonl" ]; then
  ok
elif python3 - "$here/.run/delegations.jsonl" <<'PY'
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
then ok; else ko; echo "    ledger inválido: $here/.run/delegations.jsonl"; fi

# o. La guía no pide el runner por red: su copia viaja embebida en el HTML y la
# deriva respecto al fichero del paquete se caza aquí, no al abrir.
step "la copia embebida del runner en la guía coincide con agents/orchestrator-runner.py"
if python3 - docs/web/index.html agents/orchestrator-runner.py >"$tmp/embed.out" 2>&1 <<'PY'
import difflib, html, re, sys

guide, runner = sys.argv[1], sys.argv[2]
text = open(guide, encoding="utf-8").read()
bodies = re.findall(r'<code id="runner-code">(.*?)</code>', text, re.S)
assert len(bodies) == 1, "el code del runner aparece %d veces, no una" % len(bodies)
assert "<!--" not in bodies[0], \
    "hay un comentario dentro del code del runner: microlighter exige un solo nodo de texto"
embedded = html.unescape(bodies[0]).encode("utf-8")
expected = open(runner, "rb").read()
if embedded != expected:
    print("la copia embebida no coincide con %s" % runner)
    sys.stdout.writelines(difflib.unified_diff(
        expected.decode("utf-8").splitlines(True),
        embedded.decode("utf-8").splitlines(True),
        runner, "%s (copia embebida)" % guide, n=1))
    raise SystemExit(1)
PY
then ok; else ko; sed 's/^/    /' "$tmp/embed.out"; fi

echo ""
if [ "$failures" -eq 0 ]; then
  printf '%sTodo en verde.%s\n' "$green" "$reset"
  exit 0
fi
printf '%s%d comprobación(es) en rojo.%s\n' "$red" "$failures" "$reset"
exit 1
