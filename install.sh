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

# 1. Los cinco agentes: verificación, revisión, documentación, interfaz y orquestador.
mkdir -p "$config_root/agents"
cp "$here"/agents/*.md "$config_root/agents/"
echo "Agentes copiados en $config_root/agents:"
ls -1 "$config_root/agents"

# 2. Modelos de los agentes integrados explore y general.
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
agent = data.setdefault("agent", {})
agent["explore"] = {"model": "nan/glm5.3-flash", "variant": "low"}
agent["general"] = {"model": "nan/deepseek-v4-flash", "variant": "high"}
with open(p, "w", encoding="utf-8") as f:
    json.dump(data, f, ensure_ascii=False, indent=2)
    f.write("\n")
PY
  then
    echo "Configuración actualizada: $dest (copia previa en $backup)"
  else
    echo "No pude actualizar $dest automáticamente: el fichero tiene comentarios JSONC o falta python3."
    echo "Copia previa en $backup. Añade a mano este bloque dentro del objeto raíz:"
    cat <<'EOF'

  "agent": {
    "explore": { "model": "nan/glm5.3-flash", "variant": "low" },
    "general": { "model": "nan/deepseek-v4-flash", "variant": "high" }
  },
EOF
  fi
fi

echo ""
echo "Siguiente paso, si el proveedor nan no está autenticado en esta máquina:"
echo "  opencode auth login        # elige el proveedor NaN y pega su API key"
echo ""
echo "Comprobación:"
echo "  opencode models | grep 'nan/'"
echo "  opencode api get /api/agent | grep -E 'orchestrator|verify|review|docs|ui'"
