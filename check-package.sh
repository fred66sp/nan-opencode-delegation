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
required_skills="playwright-cli-usage"
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

echo ""
if [ "$failures" -eq 0 ]; then
  printf '%sTodo en verde.%s\n' "$green" "$reset"
  exit 0
fi
printf '%s%d comprobación(es) en rojo.%s\n' "$red" "$failures" "$reset"
exit 1
