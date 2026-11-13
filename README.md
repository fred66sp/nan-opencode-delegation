# Delegación de subagentes nan para OpenCode

Un paquete instalable con la orquestación de modelos `nan` en OpenCode V2: un agente orquestador, cinco subagentes con permisos acotados —uno de ellos, `tasks-agent`, escribe en el tablero del proyecto— y los modelos de dos subagentes integrados del propio OpenCode (`explore` y `general`). La carpeta se copia tal cual a otra máquina y el instalador deja cada fichero en su sitio. No toca tu configuración más allá de lo que necesita: antes de escribir guarda una copia con fecha de la configuración y conserva las claves y agentes ajenos, aunque el fichero quede reescrito y normalizado. Los ficheros de `agents/` con el mismo nombre que uno tuyo se reemplazan tras guardar una copia `.bak` con fecha.

Los ficheros `agents/*.md` son copias literales de la configuración que ya funciona en la máquina de origen; la herramienta del flujo, `agents/orchestrator-runner.py`, nace en el paquete. El paquete no es donde nacen los cambios de la delegación, sino por donde viajan.

El orquestador trabaja con ODD (Organic Driven Development): clasifica el tamaño del trabajo, lanza el registro del trabajo grande en el tablero a `tasks-agent` en segundo plano y lo comprueba por la CLI antes de la primera edición, y cierra cada cambio de código con una línea de riesgo; el riesgo alto pasa por verificación independiente.

## Documentación

| Qué se busca | Dónde vive |
|---|---|
| La guía completa de la orquestación | [`docs/web/index.html`](docs/web/index.html) — ábrela con `./serve-web.sh` |
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
| `agents/tasks-agent.md` | `~/.config/opencode/agents/tasks-agent.md` | Escritor del tablero tasks-ai; única puerta para las herramientas `tasks_*` |
| `agents/orchestrator-runner.py` | `~/.config/opencode/agents/orchestrator-runner.py` | Guía determinista del flujo: fase actual, lista operativa y `next` con la evidencia de la fase para avanzar |
| `config/opencode.jsonc` | `~/.config/opencode/opencode.jsonc` | Plantilla de configuración con los modelos de `explore` y `general` y la regla que deniega `tasks_*` |

## Requisitos

- OpenCode V2 instalado y en el PATH ([https://opencode.ai](https://opencode.ai)).
- Acceso a los modelos del proveedor `nan`: `opencode auth login` y elige el proveedor NaN para pegar su API key. Sin esto los agentes cargan pero no responden.
- Para el agente `ui`, la skill `playwright-cli`. Es opcional: los otros cinco agentes no la necesitan.
- En Linux y macOS, el instalador fusiona una configuración existente con Python cuando el fichero es JSON estricto; con comentarios JSONC o sin Python deja el fichero intacto e imprime el bloque exacto para pegar a mano.

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

Copia los ficheros de `agents/` a `~/.config/opencode/agents/` y añade a tu `opencode.jsonc`, dentro del objeto raíz, estos bloques:

```jsonc
"agents": {
  "explore": { "model": { "providerID": "nan", "model": "glm5.3-flash", "variant": "low" } },
  "general": { "model": { "providerID": "nan", "model": "deepseek-v4-flash", "variant": "high" } }
},
"permissions": [
  { "action": "tasks_*", "resource": "*", "effect": "deny" }
]
```

Un detalle que importa: en el frontmatter de los agentes en Markdown, el modelo lleva la variante con `#` (`nan/glm5.3-flash#high`). En `agents`, la variante va en el objeto `model` (`providerID`, `model`, `variant`) o en el string `nan/glm5.3-flash#low`; no hay campo `variant` suelto. `agent` en singular está obsoleto; el instalador escribe en `agents` y migra ahí los overrides `explore` y `general`, conservando el resto de claves.

La regla `permissions` deniega `tasks_*` a todos los agentes; solo `tasks-agent` las reabre en su frontmatter, y por eso es la única puerta de las herramientas `tasks_*`. Las lecturas del tablero no pasan por esas herramientas: van directas por la CLI `tasks-ai` —o `.tasks-ai/bin/tasks-ai` si no está en el PATH—. `tasks-agent` acepta secuencias ordenadas de escrituras en un solo encargo, con una línea de resultado por operación. Corre en una sesión hija con contexto limpio: cada invocación recibe la intención y los identificadores, no el historial de la conversación.

## Verificación

```sh
opencode models | grep 'nan/'
opencode api get /api/agent | grep -E 'orchestrator|verify|review|docs|ui|tasks-agent'
```

La primera orden confirma que la máquina ve los modelos de `nan`; la segunda, que los agentes están cargados. OpenCode recarga la configuración al guardarla: no hace falta reiniciar.

## El reparto de modelos

El frontmatter de `agents/` es la fuente canónica de modelos, pasos y permisos; esta tabla es un resumen.

| Agente | Modo | Modelo | Variante | Pasos | Permisos |
|---|---|---|---|---|---|
| `orchestrator` | primary | `nan/glm5.3-flash` | high | 60 | Lanza solo los subagentes que declara su frontmatter: `explore`, `general`, `verify`, `review`, `docs`, `ui` y `tasks-agent` |
| `verify` | subagent | `nan/qwen3.8-flash` | medium | 12 | Sin editar; shell con lista de prefijos |
| `review` | subagent | `nan/glm5.3-flash` | xhigh | 12 | Sin editar; shell abierto |
| `docs` | subagent | `nan/deepseek-v4-flash` | medium | 20 | Edición solo Markdown, `docs/` y `CHANGELOG`; shell abierto (el prompt le prohíbe código) |
| `ui` | subagent | `nan/mimo-v2.6-flash` | medium | 20 | Edición permitida; Playwright con permiso `ask`; `npm run` y `pnpm` abiertos |
| `tasks-agent` | subagent | `nan/mimo-v2.6-flash` | low | 16 | Escribe en el tablero: solo `tasks_*`, el resto de herramientas denegado |
| `explore` | integrado | `nan/glm5.3-flash` | low | — | Solo configuración |
| `general` | integrado | `nan/deepseek-v4-flash` | high | — | Solo configuración |

La comparación de los dos modelos como agente de trabajo —`glm5.3-flash` frente a `deepseek-v4-flash`— se hizo con tres proyectos y tres tareas por proyecto, y dio 9 de 9 comprobaciones en verde con los dos y cero regresiones, con medias de 47,6 s para `glm5.3-flash` frente a 32,0 s para `deepseek-v4-flash`; ese es el dato que sostiene el modelo de `general`. El banco de pruebas no se conserva en el repositorio, así que la medición no es reproducible desde aquí.

## Servidor y visor

```sh
./serve-web.sh start # deja la guía web en http://127.0.0.1:8766
./serve-web.sh status
./serve-web.sh stop

./serve.sh start    # deja el tablero en http://127.0.0.1:8891
./serve.sh status
./serve.sh stop
```

`serve-web.sh` sirve el paquete por HTTP para revisar la guía con sus iconos y su resaltado: los navegadores bloquean por `file://` las máscaras SVG de los iconos y el módulo de resaltado, y servida la página se ve completa. Lo mismo de siempre: pid y registro en `.run/`, idempotente, y el 8766 no pisa los 8890 y 8891 de los dos tableros. Sírvete la raíz del paquete, no `docs/web`, porque el pie de la guía enlaza `../../README.md` y `../../CHANGELOG.md`.

`serve.sh` arranca el servidor del tablero en segundo plano, con su registro y su pid en `.run/`. Escucha en el `127.0.0.1:8891` de `.tasks-ai/settings.json`: el 8890 queda para el tablero de tasks-ai, y así los dos servidores conviven en la misma máquina. De `.tasks-ai/` solo se versionan las instrucciones, el contrato de etiquetas y los ajustes; el binario de `bin/`, la base y las extensiones no se versionan, y `tasks-ai init` completa lo que falte.

El visor de escritorio vive en [`tasks-viewer/`](tasks-viewer/): la misma interfaz en una ventana nativa, con el binario oficial de tasks-ai para Windows (amd64) y un `config.json` que ya apunta al servidor del paquete. Copia la carpeta donde quieras tenerla y lanza el exe con el servidor en marcha; sin él, la ventana no tendrá nada que mostrar.

## Límites conocidos

- La delegación se sostiene por prompt: si el orquestador no delega, nada lo obliga.
- Un subagente no puede lanzar otro; el anidamiento es de un nivel.
- `ui` con permiso `ask` bloquea su sesión hasta que alguien aprueba: no lo lances en segundo plano si no estás delante.
- `verify` solo permite los prefijos de comprobación declarados en su frontmatter; un comando compuesto (`cd x && make test`) se rechaza.
- Las herramientas de extensiones (MCP) aplican el permiso que ellas mismas declaran: la prohibición de escribir en sistemas externos vive en el prompt del orquestador, no en la configuración de permisos.

## Qué no es

No es orquestación automática: no hay planificador que reparta trabajo solo, ni límite de concurrencia configurable. El padre decide cuándo delegar con la herramienta `subagent`. Tampoco trae una skill de navegador: la verificación de interfaz depende de que la máquina tenga `playwright-cli`.
