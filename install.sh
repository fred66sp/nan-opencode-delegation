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

# 1. Los agentes: verificación, revisión, documentación, interfaz, tablero y orquestador.
agents_dir="$config_root/agents"
mkdir -p "$agents_dir"
stamp="$(date +%Y%m%d%H%M%S)"
for src in "$here"/agents/*.md; do
  target="$agents_dir/$(basename "$src")"
  if [ -f "$target" ] && ! cmp -s "$src" "$target"; then
    cp "$target" "$target.bak.$stamp"
    echo "Copia previa del agente: $target.bak.$stamp"
  fi
  cp "$src" "$target"
done
echo "Agentes copiados en $agents_dir:"
ls -1 "$agents_dir"/*.md

# 2. Modelos de explore y general, y la regla global que deniega tasks_*.
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

agents["explore"] = {"model": {"providerID": "nan", "model": "glm5.3-flash", "variant": "low"}}
agents["general"] = {"model": {"providerID": "nan", "model": "deepseek-v4-flash", "variant": "high"}}

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
    "explore": { "model": { "providerID": "nan", "model": "glm5.3-flash", "variant": "low" } },
    "general": { "model": { "providerID": "nan", "model": "deepseek-v4-flash", "variant": "high" } }
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
