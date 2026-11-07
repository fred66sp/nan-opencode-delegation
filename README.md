# Delegación de subagentes nan para OpenCode

Un paquete instalable con la orquestación de modelos `nan` en OpenCode V2: un agente orquestador, cuatro subagentes con permisos acotados y los modelos de dos subagentes integrados del propio OpenCode (`explore` y `general`). La carpeta se copia tal cual a otra máquina y el instalador deja cada fichero en su sitio. No toca tu configuración más allá de lo que necesita: antes de escribir guarda una copia con fecha y conserva las claves y agentes ajenos, aunque el fichero quede reescrito y normalizado.

Los cinco ficheros de `agents/` son copias literales de la configuración que ya funciona en la máquina de origen. El paquete no es donde nacen los cambios de la delegación, sino por donde viajan.

## Documentación

| Qué se busca | Dónde vive |
|---|---|
| La guía completa de la orquestación | [`index.html`](index.html) |
| Requisitos, alcance y criterios de aceptación | [`docs/PRD.md`](docs/PRD.md) |
| Ramas, commits, verificación e integración | [`CONTRIBUTING.md`](CONTRIBUTING.md) |
| Qué cambió en cada versión | [`CHANGELOG.md`](CHANGELOG.md) |
| Mapa del repositorio para agentes | [`AGENTS.md`](AGENTS.md) |
| Catálogo enlazable para agentes | [`llms.txt`](llms.txt) |

## Qué instala

| Origen | Destino | Qué es |
|---|---|---|
| `agents/orchestrator.md` | `~/.config/opencode/agents/orchestrator.md` | Agente primario: planifica, delega y verifica; único que cierra |
| `agents/verify.md` | `~/.config/opencode/agents/verify.md` | Ejecuta las comprobaciones del proyecto y devuelve evidencia |
| `agents/review.md` | `~/.config/opencode/agents/review.md` | Revisa un cambio contra su criterio sin modificarlo |
| `agents/docs.md` | `~/.config/opencode/agents/docs.md` | Documentación y CHANGELOG dentro de su perímetro |
| `agents/ui.md` | `~/.config/opencode/agents/ui.md` | Verificación de interfaz con navegador; pide permiso para Playwright |
| `config/opencode.jsonc` | `~/.config/opencode/opencode.jsonc` | Plantilla de configuración con los modelos de `explore` y `general` |

## Requisitos

- OpenCode V2 instalado y en el PATH ([https://opencode.ai](https://opencode.ai)).
- Acceso a los modelos del proveedor `nan`: `opencode auth login` y elige el proveedor NaN para pegar su API key. Sin esto los agentes cargan pero no responden.
- Para el agente `ui`, la skill `playwright-cli`. Es opcional: los otros cuatro agentes no la necesitan.
- En Linux y macOS, el instalador fundiona una configuración existente con Python cuando el fichero es JSON estricto; con comentarios JSONC o sin Python deja el fichero intacto e imprime el bloque exacto para pegar a mano.

## Instalación

En Linux, macOS o WSL:

```sh
bash install.sh
```

En Windows (PowerShell):

```powershell
powershell -ExecutionPolicy Bypass -File install.ps1
```

El instalador resuelve todo desde su propia carpeta y el destino queda en el usuario actual: no hay nada que apunte a la máquina de origen.

### A mano

Copia los cinco ficheros de `agents/` a `~/.config/opencode/agents/` y añade a tu `opencode.jsonc`, dentro del objeto raíz, este bloque:

```jsonc
"agent": {
  "explore": { "model": "nan/glm5.3-flash", "variant": "low" },
  "general": { "model": "nan/deepseek-v4-flash", "variant": "high" }
}
```

Un detalle que importa: en el frontmatter de los agentes en Markdown, el modelo lleva la variante con `#` (`nan/glm5.3-flash#high`). En la clave `agent` de la configuración, la variante va en su propio campo (`variant`); con `#` el campo se queda vacío. Si tu configuración ya usa la clave `agents` en plural para los overrides, conserva la tuya y fusiona los dos pares.

## Verificación

```sh
opencode models | grep 'nan/'
opencode api get /api/agent | grep -E 'orchestrator|verify|review|docs|ui'
```

La primera orden confirma que la máquina ve los modelos de `nan`; la segunda, que los agentes están cargados. OpenCode recarga la configuración al guardarla: no hace falta reiniciar.

## El reparto de modelos

| Agente | Modo | Modelo | Variante | Pasos | Permisos |
|---|---|---|---|---|---|
| `orchestrator` | primary | `nan/glm5.3-flash` | high | 60 | Lanza solo los subagentes que declara su frontmatter: `explore`, `general`, `verify`, `review`, `docs` y `ui` |
| `verify` | subagent | `nan/qwen3.8-flash` | medium | 12 | Sin editar; shell con lista de prefijos |
| `review` | subagent | `nan/glm5.3-flash` | xhigh | 12 | Sin editar |
| `docs` | subagent | `nan/deepseek-v4-flash` | medium | 20 | Solo Markdown, `docs/` y CHANGELOG |
| `ui` | subagent | `nan/mimo-v2.6-flash` | medium | 20 | Playwright con permiso `ask` |
| `explore` | integrado | `nan/glm5.3-flash` | low | — | Solo configuración |
| `general` | integrado | `nan/deepseek-v4-flash` | high | — | Solo configuración |

El modelo `nan/glm5.3` sin flash y las variantes del plan original quedaron fuera: la API key no tenía acceso a él cuando se montó esto. La comparación de los dos modelos como agente de trabajo —`glm5.3-flash` frente a `deepseek-v4-flash`— dio 9 de 9 comprobaciones en verde con los dos y cero regresiones, con medias de 47,6 s para `glm5.3-flash` frente a 32,0 s para `deepseek-v4-flash`; ese es el dato que sostiene el modelo de `general`.

## Límites conocidos

- La delegación se sostiene por prompt: si el orquestador no delega, nada lo obliga.
- Un subagente no puede lanzar otro; el anidamiento es de un nivel.
- `ui` con permiso `ask` bloquea su sesión hasta que alguien aprueba: no lo lances en segundo plano si no estás delante.
- `verify` solo permite los prefijos de comprobación declarados en su frontmatter; un comando compuesto (`cd x && make test`) se rechaza.
- Las herramientas de extensiones (MCP) aplican el permiso que ellas mismas declaran: la prohibición de escribir en sistemas externos vive en el prompt del orquestador, no en la configuración de permisos.

## Qué no es

No es orquestación automática: no hay planificador que reparta trabajo solo, ni límite de concurrencia configurable. El padre decide cuándo delegar con la herramienta `subagent`. Tampoco trae una skill de navegador: la verificación de interfaz depende de que la máquina tenga `playwright-cli`.
