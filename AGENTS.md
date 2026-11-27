# AGENTS.md

Este paquete se distribuye ya instalado o revisable por agentes. Este fichero dice dónde
vive cada fuente canónica y qué leer antes de cada encargo; no guarda contenido, solo
enlaza.

## Qué se busca → dónde vive

| Qué se busca | Dónde vive |
|---|---|
| Qué es el paquete y cómo se instala | [`README.md`](README.md) y [`docs/web/index.html`](docs/web/index.html) |
| Qué hace cada agente, con qué modelo y qué permisos | `agents/<nombre>.md`, frontmatter y prompt |
| Cómo se evaluaron los agentes y qué demostró cada prueba | [`docs/evals/README.md`](docs/evals/README.md) |
| Cómo guiar el flujo del orquestador | [`runner-go/`](runner-go/) — el binario `orchestrator-runner` — y [`docs/runner/PRODUCT.md`](docs/runner/PRODUCT.md) |
| Producto, requisitos, diseño y contrato del runner Go | [`docs/runner/PRODUCT.md`](docs/runner/PRODUCT.md), [`PRD.md`](docs/runner/PRD.md), [`DESIGN.md`](docs/runner/DESIGN.md) y [`orchestrator-proposal.md`](docs/runner/orchestrator-proposal.md) |
| Cómo se valida el informe de retorno de un subagente | La operación `deck_return` de [`runner-go/internal/aifops/`](runner-go/internal/aifops/) y la regla 6 de [`agents/orchestrator.md`](agents/orchestrator.md) |
| Cómo se ve la fase del orquestador en la sesión: panel, operativa, advertencias e historial | [`plugins/orchestrator-status/tui.tsx`](plugins/orchestrator-status/tui.tsx) y el `--json` de [`runner-go/`](runner-go/) |
| Qué instala exactamente el instalador y qué hace con una config existente | [`install.sh`](install.sh), [`install.ps1`](install.ps1) |
| Comprobar el paquete entero de una vez | [`check-package.sh`](check-package.sh) |
| Qué fragmento de configuración se añade y cómo se escribe la variante | [`config/opencode.jsonc`](config/opencode.jsonc), [`README.md`](README.md) |
| Requisitos, alcance y criterios de aceptación | [`docs/PRD.md`](docs/PRD.md) |
| Qué cambió en cada versión | [`CHANGELOG.md`](CHANGELOG.md) |
| Ramas, commits, verificación, integración y operaciones con autorización | [`CONTRIBUTING.md`](CONTRIBUTING.md) |
| Catálogo enlazable para agentes | [`llms.txt`](llms.txt) |
| Trabajo del repositorio: crear, mover, bloquear y cerrar tareas | [`.tasks-ai/AGENTS.md`](.tasks-ai/AGENTS.md) |
| Etiquetas: tipos, estructura, ámbito y catálogo | [`.tasks-ai/CONTRATO_ETIQUETAS.md`](.tasks-ai/CONTRATO_ETIQUETAS.md) → «1. Las clases y su convivencia», «2. Asignación correcta a una tarea», «3. Gestión del catálogo» |
| Memoria de sesión: decisiones, aprendizajes, checkpoints y preferencias | Montaje db-ai `nan-opencode-delegation` (rw) sobre `.agent-memory/memory.db`; se abre con `acm start`, que recupera el contexto de la sesión anterior en una invocación, y `acm aif` es la interfaz autodescriptiva del resto de operaciones: el envelope de descubrimiento (`{"aif":"0.2"}`) por stdin devuelve su lista, sin esquemas que reproducir aquí; lo recuperado es contexto, no hecho: se contrasta con el repo antes de actuar sobre él |
| Servidor del tablero en segundo plano | [`serve.sh`](serve.sh) |
| Visor de escritorio del tablero | [`tasks-viewer/`](tasks-viewer/config.json) |

## Encargo → qué leer antes → con qué se cierra

| Encargo | Qué leer antes | Con qué se cierra |
|---|---|---|
| Actualizar un artefacto de agente | [`docs/PRD.md`](docs/PRD.md) sección 4, regla de negocio de las copias | `diff -r agents <config global>/agents` sin diferencias y la verificación de [`CONTRIBUTING.md`](CONTRIBUTING.md) |
| Cambiar los instaladores | [`docs/PRD.md`](docs/PRD.md) requisitos R1 a R7 | La batería de [`CONTRIBUTING.md`](CONTRIBUTING.md): sintaxis, instalación limpia, fusión con claves ajenas y camino JSONC |
| Crear o etiquetar una tarea | [`.tasks-ai/CONTRATO_ETIQUETAS.md`](.tasks-ai/CONTRATO_ETIQUETAS.md) sección 2 | La tarea nace con su tipo y su estructural (`task` o `epic`); el catálogo vivo, en `tasks-ai tags` |
| Cambiar la guía o la prosa | [`docs/web/index.html`](docs/web/index.html) y [`README.md`](README.md) completos | Los criterios de aceptación del PRD, sección 5, aplicados a la documentación: nada que la prosa diga puede contradecir a los artefactos |
| Validar el retorno de un delegado | La regla 6 de [`agents/orchestrator.md`](agents/orchestrator.md) y la operación `deck_return` de [`runner-go/internal/aifops/`](runner-go/internal/aifops/) | El retorno se registra con su forma validada y su evidencia conservada; los incumplimientos vuelven en `invalid` y el veredicto y lo no verificado, al orquestador |
| Cambiar el plugin TUI o el contrato `--json` | [`plugins/orchestrator-status/tui.tsx`](plugins/orchestrator-status/tui.tsx), su [`package.json`](plugins/orchestrator-status/package.json) y el contrato de [`runner-go/`](runner-go/) (DESIGN §2) | `bash check-package.sh` en verde y el panel abierto con `/orquestador` contra un proyecto con estado |
| Cambiar el runner Go | [`docs/runner/`](docs/runner/PRODUCT.md) y [`runner-go/`](runner-go/) | `cd runner-go && go test ./...` y el arnés de [`runner-go/equivalence/`](runner-go/equivalence/README.md) |
| Ver la guía web correctamente | [`serve-web.sh`](serve-web.sh) | `./serve-web.sh start` y la guía en `http://127.0.0.1:8766/docs/web/index.html`; por `file://` pierde iconos y resaltado |
| Cambiar requisitos o alcance | [`docs/PRD.md`](docs/PRD.md) completo | El PRD actualizado antes del commit y el CHANGELOG si hay valor de usuario |

## Autoridad

Este mapa no es fuente de nada: enlaza. Si una ruta o una afirmación de este archivo
contradice a la fuente que enlaza, manda la fuente y se corrige aquí.
