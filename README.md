# Delegación de subagentes nan para OpenCode

Un paquete instalable con la orquestación de modelos `nan` en OpenCode V2: un agente orquestador, cuatro subagentes con permisos acotados y los modelos de dos subagentes integrados del propio OpenCode (`explore` y `general`). El tablero del proyecto no lleva intermediario: el orquestador lo gestiona directamente con AIF (Tasks-AI v0.2.0). La carpeta se copia tal cual a otra máquina y el instalador deja cada fichero en su sitio. No toca tu configuración más allá de lo que necesita: antes de escribir guarda una copia con fecha de la configuración y conserva las claves y agentes ajenos, aunque el fichero quede reescrito y normalizado. Los ficheros de `agents/` con el mismo nombre que uno tuyo se reemplazan tras guardar una copia `.bak` con fecha.

Los ficheros `agents/*.md` son copias literales de la configuración que ya funciona en la máquina de origen; el runner —módulo Go en `runner-go/` y binario `orchestrator-runner`— nace en el paquete. El paquete no es donde nacen los cambios de la delegación, sino por donde viajan.

El orquestador trabaja con ODD (Organic Driven Development): clasifica el tamaño del trabajo, registra el trabajo —épica o tarea, salvo trivialidades de uno o dos archivos— por AIF (`tasks-ai aif`, el subcomando autodescriptivo del tablero) y lo comprueba por la CLI antes de la primera edición, y cierra cada cambio de código con una línea de riesgo; el riesgo alto pasa por verificación independiente. Todo encargo exige a su subagente un retorno estructurado —`status` (`completed`, `partial` o `blocked`, o la pregunta al humano si el bloqueo es de decisión), `validation` con una línea por comprobación, riesgos y lo no verificado, en un informe que se entiende sin haber visto el trabajo—; un obstáculo corriente —error, timeout, herramienta que falla— se diagnostica y se supera, y `blocked` queda para el bloqueo deliberado —fichero marcado para no tocar, acceso retirado a propósito—, que se respeta y se comunica. Ante un `partial` o un retorno malformado, el orquestador clasifica la causa antes de re-encargar —selección de fuente, interpretación, estrategia de búsqueda o ejecución; el eje ajeno al subagente queda para el bloqueo de decisión o del entorno— y el re-encargo cita el eje y lo que lo corrige; el criterio de aceptación viaja literal de la tarea al encargo y cada línea de `validation` cita la salida observada, porque la transcripción entre llamadas introduce errores. El retorno entra por el runner, que valida su forma al registrarlo —`status` único y válido, una línea por comprobación esperada y ni una referencia indefinida—, conserva la evidencia y devuelve lo pendiente. No se lanza el mismo encargo dos veces mientras el primero siga en curso: el runner es el único escritor del registro de delegaciones, calcula la huella del encargo, bloquea un duplicado abierto y lleva el ciclo con las operaciones `deck_*` de AIF. Los mapas de `explore` y la evidencia de `verify` vuelven acotados a unos 2k tokens con `path:line`.

## Documentación

| Qué se busca | Dónde vive |
|---|---|
| La guía completa de la orquestación | [`docs/web/index.html`](docs/web/index.html) — ábrela con `./serve-web.sh` |
| Producto, requisitos, diseño y contrato del runner | [`docs/runner/`](docs/runner/PRODUCT.md) — PRODUCT, PRD, DESIGN y la propuesta de `orchestrator.md` |
| Código y pruebas del runner Go | [`runner-go/`](runner-go/) — `go test ./...` y el arnés de equivalencia en [`runner-go/equivalence/`](runner-go/equivalence/README.md) |
| Requisitos, alcance y criterios de aceptación | [`docs/PRD.md`](docs/PRD.md) |
| Ramas, commits, verificación e integración | [`CONTRIBUTING.md`](CONTRIBUTING.md) |
| Qué cambió en cada versión | [`CHANGELOG.md`](CHANGELOG.md) |
| Mapa del repositorio para agentes | [`AGENTS.md`](AGENTS.md) |
| Catálogo enlazable para agentes | [`llms.txt`](llms.txt) |

## Qué instala

| Origen | Destino | Qué es |
|---|---|---|
| `agents/orchestrator.md` | `~/.config/opencode/agents/orchestrator.md` | Agente primario: planifica, delega y verifica; único que cierra |
| `agents/verify.md` | `~/.config/opencode/agents/verify.md` | Contrasta el criterio, analiza críticamente el cambio y devuelve evidencia de las comprobaciones |
| `agents/review.md` | `~/.config/opencode/agents/review.md` | Revisa un cambio contra su criterio sin modificarlo |
| `agents/docs.md` | `~/.config/opencode/agents/docs.md` | Documentación y CHANGELOG dentro de su perímetro |
| `agents/ui.md` | `~/.config/opencode/agents/ui.md` | Verificación de interfaz con navegador; pide permiso para Playwright |
| `runner-go/` | `~/.config/opencode/agents/orchestrator-runner` (`.exe` en Windows) | Runner del proceso en Go, con AIF nativo y SQLite: `start` presenta el contexto operativo, `aif` ejecuta el catálogo, `--json` alimenta a los plugins, `recover` recupera un estado dañado y `export-legacy` permite volver al formato anterior |
| `runner-go/` | `~/.local/bin/runner` (Linux, macOS y WSL); `$HOME\.local\bin\runner.exe` con PATH de usuario (Windows) | El comando `runner` del contrato, en el PATH del usuario como hace ACM con `acm`: enlace simbólico al binario instalado en Linux y copia del `.exe` en Windows. Si el nombre lo ocupa otra herramienta, el instalador avisa y no lo sobrescribe; retirar el enlace o la copia revierte |
| `commands/odd-fix.md` | `~/.config/opencode/commands/odd-fix.md` | Comando nativo `/odd-fix`: selecciona al agente `orchestrator` en la sesión actual y le pasa la descripción del defecto en `$ARGUMENTS`; si falta, el agente la pide y no toca archivos |
| `plugins/orchestrator-phase.ts` | `~/.config/opencode/plugins/orchestrator-phase.ts` | Plugin de servidor: inyecta en el system prompt de cada petición la fase actual del runner, su nombre y la evidencia que pide la transición para avanzar, solo cuando cambia la fase o hay avisos; sin registro en `cli.json` |
| `plugins/orchestrator-status/` | `~/.config/opencode/cli-plugins/orchestrator-status/` registrado en `cli.json` | Plugin TUI: diálogo modal con scroll y la fase del orquestador, su operativa, sus advertencias y el historial de evidencias; se abre con `/orquestador` y se cierra con `esc` |
| `config/opencode.jsonc` | `~/.config/opencode/opencode.jsonc` | Plantilla de configuración con los modelos de `explore` y `general` y su permiso `skill` denegado |

`orchestrator-runner aif` es la interfaz principal: descubre y ejecuta el catálogo —`phase_next`, `phase_skip`, `phase_reset`, `recover`, `deck_prepare`, `deck_launch`, `deck_return`, `deck_close`, `deck_note` y `deck_list`— con un envelope JSON por stdin y una respuesta por stdout. `start` presenta en una invocación memoria, tablero, Git, fase y pendientes, con degradación por sección; `--version` identifica el binario y `--json` responde en solo lectura a los plugins. El estado del proyecto vive en `.run/orchestrator.db` (SQLite): la primera mutación importa el estado y el registro de delegaciones anteriores y los archiva como `.migrated`; `recover` conserva lo rescatable y declara lo perdido, y `export-legacy` devuelve el estado al formato anterior para revertir sin perder trabajo.

El comando `/odd-fix` viaja en esa misma copia: el instalador deja `commands/odd-fix.md` en `~/.config/opencode/commands/`, con copia `.bak` con fecha si reemplaza una versión distinta. Se invoca con la descripción del defecto y la pasa al `orchestrator`; si falta o no describe un defecto observable, el agente pide la descripción antes de actuar. No añade herramientas ni un flujo propio: el trabajo sigue el contrato y el tablero del orquestador. No hay `/odd-status`: se descartó porque `start` puede escribir —`git status` y `acm start`— y su `--json` se solapa con el panel `/orquestador`.

Los plugins no se compilan ni se instalan con npm: el instalador copia sus fuentes y OpenCode resuelve sus dependencias en runtime. El de servidor vive en `plugins/` y lo carga el servidor; el TUI va en `cli-plugins/` y se registra en `cli.json`, porque un paquete solo-TUI en `plugins/` colisiona con el OpenTUI interno. El panel no guarda estado propio: invoca `orchestrator-runner --json` desde el directorio del proyecto y pinta lo que responde, así que abrirlo o pulsar `r` para refrescar no cambia la fase. Sin estado muestra la fase 1 y lo dice. El panel, de la fase 2 a la 8, avisa cuando `git status --porcelain` encuentra ficheros sin commit —trabajo en el árbol que aún no ha pasado por Riesgo y Commit—, y ese aviso tiene preferencia sobre el de la fase 1 sin evidencia. El plugin de servidor es tolerante: sin estado, sin runner o con una consulta que excede los 3 s no inyecta nada y la petición sigue.

## Requisitos

- OpenCode V2 instalado y en el PATH ([https://opencode.ai](https://opencode.ai)).
- Acceso a los modelos del proveedor `nan`: `opencode auth login` y elige el proveedor NaN para pegar su API key. Sin esto los agentes cargan pero no responden.
- Para el agente `ui`, la skill `playwright-cli-usage`, que el orquestador le pasa como ruta absoluta en el encargo. Es opcional: los subagentes no llevan catálogo de skills y ninguno más la necesita.
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

A mano: copia los ficheros de `agents/` a `~/.config/opencode/agents/`, coloca allí el binario `orchestrator-runner` —el instalador lo descarga de las GitHub Releases; compílalo con `cd runner-go && go build ./cmd/orchestrator-runner` si trabajas desde el repositorio—, enlaza el comando `runner` al binario en `~/.local/bin` (`ln -s ~/.config/opencode/agents/orchestrator-runner ~/.local/bin/runner`, o su equivalente en Windows), copia `commands/odd-fix.md` a `~/.config/opencode/commands/`, copia `plugins/orchestrator-phase.ts` a `~/.config/opencode/plugins/` —lo carga el servidor, así que no va en `cli.json`—, copia `plugins/orchestrator-status/` a `~/.config/opencode/cli-plugins/orchestrator-status/` y añade esa ruta a la clave `plugins` de tu `cli.json`. Después añade a tu `opencode.jsonc`, dentro del objeto raíz, estos bloques:

```jsonc
"agents": {
  "explore": { "model": { "providerID": "nan", "model": "glm5.3-flash", "variant": "low" }, "permissions": [ { "action": "skill", "resource": "*", "effect": "deny" } ] },
  "general": { "model": { "providerID": "nan", "model": "deepseek-v4-flash", "variant": "high" }, "permissions": [ { "action": "skill", "resource": "*", "effect": "deny" } ] }
}
```

Un detalle que importa: en el frontmatter de los agentes en Markdown, el modelo lleva la variante con `#` (`nan/glm5.3-flash#high`). En `agents`, la variante va en el objeto `model` (`providerID`, `model`, `variant`) o en el string `nan/glm5.3-flash#low`; no hay campo `variant` suelto. `agent` en singular está obsoleto; el instalador escribe en `agents` y migra ahí los overrides `explore` y `general`, conservando el resto de claves.

El tablero no lleva intermediario: el orquestador ejecuta sus operaciones por AIF, el subcomando autodescriptivo del tablero (`tasks-ai aif`), y lo que AIF no cubre —los ADR (`adr create/edit/import/export`) y el catálogo de etiquetas (`tag create/edit/delete`)— va por la CLI `tasks-ai` —o `.tasks-ai/bin/tasks-ai` si no está en el PATH—, igual que las lecturas.

El reparto de skills también va por `permissions`: los subagentes, y `explore` y `general`, llevan `action: skill` con `resource: "*"` y `effect: deny`, que oculta la skill al modelo y rechaza la carga; el catálogo lo conserva el orquestador. `tools` es un campo de la V1 que OpenCode V2 acepta en el esquema pero ignora; el mecanismo vigente es el permiso.

## Verificación

```sh
opencode models | grep 'nan/'
opencode api get /api/agent | grep -E 'orchestrator|verify|review|docs|ui'
```

La primera orden confirma que la máquina ve los modelos de `nan`; la segunda, que los agentes están cargados. OpenCode recarga la configuración al guardarla: no hace falta reiniciar.

## El reparto de modelos

El frontmatter de `agents/` es la fuente canónica de modelos, pasos y permisos; esta tabla es un resumen.

| Agente | Modo | Modelo | Variante | Pasos | Permisos |
|---|---|---|---|---|---|
| `orchestrator` | primary | `nan/glm5.3-flash` | high | 60 | Lanza solo los subagentes que declara su frontmatter: `explore`, `general`, `verify`, `review`, `docs` y `ui` |
| `verify` | subagent | `nan/qwen3.8-flash` | medium | 12 | Sin editar; shell con lista de prefijos |
| `review` | subagent | `nan/glm5.3-flash` | xhigh | 12 | Sin editar; shell abierto |
| `docs` | subagent | `nan/deepseek-v4-flash` | medium | 20 | Edición solo Markdown, `docs/` y `CHANGELOG`; shell abierto (el prompt le prohíbe código) |
| `ui` | subagent | `nan/mimo-v2.6-flash` | medium | 20 | Edición permitida; Playwright con permiso `ask`; `npm run` y `pnpm` abiertos |
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
- El reparto de skills depende del orquestador: ningún subagente lleva catálogo —tiene el permiso `skill` denegado (`action: skill`, `resource: "*"`, `effect: deny`)— y no puede cargar ninguna. Si el orquestador no adjunta la skill que el encargo necesita, el subagente trabaja sin ella y nada lo detecta automáticamente.
- El runner queda junto a los agentes: `~/.config/opencode/agents/orchestrator-runner`. El panel y el plugin de servidor lo buscan en esa ruta o, si no, en el `PATH`; si instalaste con otro `XDG_CONFIG_HOME`, usa `OPENCODE_RUNNER_BIN`. El comando `runner` del contrato vive en `~/.local/bin` (Linux, macOS y WSL) o en `$HOME\.local\bin` con PATH de usuario (Windows): si el instalador encuentra ahí un `runner` ajeno, avisa y deja el comando de la otra herramienta intacto. El plugin de servidor tolera además la ausencia de estado, el runner ausente o una consulta de más de 3 s: la petición sigue sin la fase.
- La validación del retorno juzga la forma, no si el trabajo está bien: un informe conforme puede describir un trabajo roto, y una prosa legítima puede quedar marcada por una anáfora sin ruta en la misma línea. El veredicto no sustituye a la evidencia.
- El runner no observa OpenCode: el estado registrado y el real se contrastan por el orquestador. Un encargo abierto sin cerrar produce una advertencia en `start` y en `deck_list`, nunca un abandono automático.
- El registro vive en `.run/orchestrator.db` (SQLite) y solo lo escribe el runner; el registro de delegaciones anterior se importa una vez y queda archivado como `.migrated`. No se edita a mano.
- Las instrucciones `AGENTS.md` global y de proyecto siguen llegando a todos los agentes: el reparto de skills no las sustituye.
- Desde OpenCode v2.3.1, cuando un directorio no tiene `AGENTS.md` se lee su `CLAUDE.md`, y si falta el `AGENTS.md` global se cae a `~/.claude/CLAUDE.md`. Si llevas reglas ajenas en alguno de esos ficheros, llegan igualmente a los agentes de este paquete; para saltarse todos los `CLAUDE.md` exporta `OPENCODE_DISABLE_CLAUDE_CODE` (o `OPENCODE_DISABLE_CLAUDE_CODE_PROMPT` si solo quieres excluirlos de las instrucciones inyectadas).
- El instalador reescribe `agents.explore` y `agents.general` enteros: cualquier clave propia dentro de esos dos agentes se pierde al instalar.
- Las herramientas de extensiones (MCP) aplican el permiso que ellas mismas declaran: la prohibición de escribir en sistemas externos vive en el prompt del orquestador, no en la configuración de permisos.

## Qué no es

No es orquestación automática: no hay planificador que reparta trabajo solo, ni límite de concurrencia configurable. El padre decide cuándo delegar con la herramienta `subagent`. Tampoco trae una skill de navegador: la verificación de interfaz depende de que la máquina tenga `playwright-cli-usage` y de que el orquestador la adjunte al encargo de `ui`.
