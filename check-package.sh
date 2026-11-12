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

# c. Instalación limpia: agentes y configuración en un HOME vacío.
step "instalación limpia en un HOME temporal"
home_c="$tmp/home-clean"
mkdir -p "$home_c"
if env -u XDG_CONFIG_HOME HOME="$home_c" bash install.sh >"$tmp/clean.log" 2>&1; then
  cfg_c="$home_c/.config/opencode/opencode.jsonc"
  [ -f "$cfg_c" ] || cfg_c="$home_c/.config/opencode/opencode.json"
  if [ -d "$home_c/.config/opencode/agents" ] && [ -f "$cfg_c" ]; then
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

echo ""
if [ "$failures" -eq 0 ]; then
  printf '%sTodo en verde.%s\n' "$green" "$reset"
  exit 0
fi
printf '%s%d comprobación(es) en rojo.%s\n' "$red" "$failures" "$reset"
exit 1
