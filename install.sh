#!/usr/bin/env bash
# Instala la delegación de subagentes nan en la configuración global de OpenCode.
# Uso: bash install.sh
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
config_root="${XDG_CONFIG_HOME:-$HOME/.config}/opencode"

if ! command -v opencode >/dev/null 2>&1; then
  echo "opencode no está en el PATH. Instálalo primero: https://opencode.ai"
  exit 1
fi

# 1. Los agentes —verificación, revisión, documentación, interfaz, tablero y orquestador—
# y la herramienta del flujo, en el mismo directorio.
agents_dir="$config_root/agents"
mkdir -p "$agents_dir"
stamp="$(date +%Y%m%d%H%M%S)"
for src in "$here"/agents/*; do
  [ -f "$src" ] || continue   # salta directorios sueltos (p. ej. __pycache__ de py_compile)
  target="$agents_dir/$(basename "$src")"
  if [ -f "$target" ] && ! cmp -s "$src" "$target"; then
    cp "$target" "$target.bak.$stamp"
    echo "Copia previa del agente: $target.bak.$stamp"
  fi
  cp "$src" "$target"
  if [ -x "$src" ]; then chmod 755 "$target"; fi
done
echo "Agentes copiados en $agents_dir:"
ls -1 "$agents_dir"/*.md "$agents_dir"/orchestrator-runner.py

# 2. Los comandos nativos, en commands/ del mismo directorio de configuración.
commands_dir="$config_root/commands"
mkdir -p "$commands_dir"
for src in "$here"/commands/*; do
  [ -f "$src" ] || continue
  target="$commands_dir/$(basename "$src")"
  if [ -f "$target" ] && ! cmp -s "$src" "$target"; then
    cp "$target" "$target.bak.$stamp"
    echo "Copia previa del comando: $target.bak.$stamp"
  fi
  cp "$src" "$target"
done
echo "Comandos copiados en $commands_dir:"
ls -1 "$commands_dir"/*.md

# 3. El plugin de servidor que inyecta la fase del orquestador en el system prompt.
# Vive en plugins/ y no se registra en cli.json: lo carga el servidor, no la TUI.
server_plugin_src="$here/plugins/orchestrator-phase.ts"
server_plugin_dir="$config_root/plugins"
if [ ! -f "$server_plugin_src" ]; then
  echo "error: no existe $server_plugin_src: el paquete está incompleto" >&2
  exit 1
fi
mkdir -p "$server_plugin_dir"
server_plugin_target="$server_plugin_dir/orchestrator-phase.ts"
if [ -f "$server_plugin_target" ] && ! cmp -s "$server_plugin_src" "$server_plugin_target"; then
  cp "$server_plugin_target" "$server_plugin_target.bak.$stamp"
  echo "Copia previa del plugin de servidor: $server_plugin_target.bak.$stamp"
fi
cp "$server_plugin_src" "$server_plugin_target"
echo "Plugin de servidor copiado en $server_plugin_target"

# 4. El plugin TUI del estado de orquestación, con sus fuentes. Sin node_modules:
# OpenCode resuelve sus dependencias en runtime. Va en cli-plugins y se registra en
# cli.json: un paquete solo-TUI no puede vivir en plugins/ (el servidor lo cargaría
# y colisiona con su OpenTUI interno) y solo la TUI debe importarlo.
plugin_src="$here/plugins/orchestrator-status"
plugin_dir="$config_root/cli-plugins/orchestrator-status"
if [ ! -d "$plugin_src" ]; then
  echo "error: no existe $plugin_src: el paquete está incompleto" >&2
  exit 1
fi
while IFS= read -r src; do
  rel="${src#"$plugin_src"/}"
  target="$plugin_dir/$rel"
  mkdir -p "$(dirname "$target")"
  if [ -f "$target" ] && ! cmp -s "$src" "$target"; then
    cp "$target" "$target.bak.$stamp"
    echo "Copia previa del plugin: $target.bak.$stamp"
  fi
  cp "$src" "$target"
done < <(find "$plugin_src" -type f -not -path '*/node_modules/*')
echo "Plugin copiado en $plugin_dir:"
ls -1 "$plugin_dir"/tui.tsx "$plugin_dir"/package.json

cli_json="$config_root/cli.json"
cli_bak="$cli_json.bak.$stamp"
cp "$cli_json" "$cli_bak" 2>/dev/null || true
if python3 - "$cli_json" "$plugin_dir" 2>/dev/null <<'PY'
import json, sys
path, entry = sys.argv[1], sys.argv[2]
try:
    data = json.load(open(path, encoding="utf-8"))
except FileNotFoundError:
    data = {}
plugins = data.get("plugins")
if plugins is None:
    plugins = []
    data["plugins"] = plugins
elif not isinstance(plugins, list):
    raise ValueError("la clave 'plugins' no es una lista")
if entry not in plugins:
    plugins.append(entry)
with open(path, "w", encoding="utf-8") as f:
    json.dump(data, f, ensure_ascii=False, indent=2)
    f.write("\n")
PY
then
  echo "Plugin registrado en cli.json (copia previa en $cli_bak, si existía)"
else
  cp "$cli_bak" "$cli_json" 2>/dev/null || true
  echo "No pude actualizar $cli_json automáticamente: falta python3, el fichero no es JSON estricto o la clave 'plugins' no es una lista."
  echo "Añade a mano la ruta del plugin a la clave plugins de cli.json:"
  echo "  \"plugins\": [ \"$plugin_dir\", ... ]"
fi

# 5. Modelos de explore y general, y la regla global que deniega tasks_*.
dest=""
for cand in "$config_root/opencode.jsonc" "$config_root/opencode.json"; do
  if [ -f "$cand" ]; then dest="$cand"; break; fi
done

if [ -z "$dest" ]; then
  cp "$here/config/opencode.jsonc" "$config_root/opencode.jsonc"
  echo "Configuración creada: $config_root/opencode.jsonc"
else
  backup="$dest.bak.$(date +%Y%m%d%H%M%S)"
  cp "$dest" "$backup"
  if python3 - "$dest" 2>/dev/null <<'PY'
import json, sys
p = sys.argv[1]
data = json.load(open(p, encoding="utf-8"))
agents = data.get("agents")
if agents is None:
    agents = {}
    data["agents"] = agents
elif not isinstance(agents, dict):
    raise ValueError("la clave 'agents' no es un objeto")

old = data.get("agent")
if isinstance(old, dict):
    for name in ("explore", "general"):
        if name in old:
            agents[name] = old.pop(name)
    if old:
        print("Aviso: la configuración conserva la clave obsoleta 'agent' con otras entradas; los overrides de explore y general se han movido a 'agents'.")
    else:
        data.pop("agent", None)
elif old is not None:
    print("Aviso: la clave obsoleta 'agent' no es un objeto; no se migra.")

agents["explore"] = {"model": {"providerID": "nan", "model": "glm5.3-flash", "variant": "low"}, "permissions": [{"action": "skill", "resource": "*", "effect": "deny"}]}
agents["general"] = {"model": {"providerID": "nan", "model": "deepseek-v4-flash", "variant": "high"}, "permissions": [{"action": "skill", "resource": "*", "effect": "deny"}]}

rule = {"action": "tasks_*", "resource": "*", "effect": "deny"}
perms = data.get("permissions")
if perms is None:
    data["permissions"] = [rule]
elif isinstance(perms, list):
    if not any(isinstance(r, dict) and r.get("action") == "tasks_*" and r.get("effect") == "deny" for r in perms):
        perms.append(rule)
else:
    print("Aviso: la clave 'permissions' no es una lista; no se añade la regla de tasks_*.")

with open(p, "w", encoding="utf-8") as f:
    json.dump(data, f, ensure_ascii=False, indent=2)
    f.write("\n")
PY
  then
    echo "Configuración actualizada: $dest (copia previa en $backup)"
  else
    echo "No pude actualizar $dest automáticamente: el fichero tiene comentarios JSONC, falta python3 o la clave 'agents' no es un objeto."
    echo "Copia previa en $backup. Añade a mano este bloque dentro del objeto raíz:"
    cat <<'EOF'

  "agents": {
    "explore": { "model": { "providerID": "nan", "model": "glm5.3-flash", "variant": "low" }, "permissions": [ { "action": "skill", "resource": "*", "effect": "deny" } ] },
    "general": { "model": { "providerID": "nan", "model": "deepseek-v4-flash", "variant": "high" }, "permissions": [ { "action": "skill", "resource": "*", "effect": "deny" } ] }
  },
  "permissions": [
    { "action": "tasks_*", "resource": "*", "effect": "deny" }
  ],
EOF
  fi
fi

echo ""
echo "Siguiente paso, si el proveedor nan no está autenticado en esta máquina:"
echo "  opencode auth login        # elige el proveedor NaN y pega su API key"
echo ""
echo "Comprobación:"
echo "  opencode models | grep 'nan/'"
echo "  opencode api get /api/agent | grep -E 'orchestrator|verify|review|docs|ui|tasks-agent'"
