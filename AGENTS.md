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
| Cómo guiar el flujo del orquestador | [`agents/orchestrator-runner.py`](agents/orchestrator-runner.py) |
| Cómo se ve la fase del orquestador en la sesión: panel, operativa, advertencias e historial | [`plugins/orchestrator-status/tui.tsx`](plugins/orchestrator-status/tui.tsx) y el `--json` de [`agents/orchestrator-runner.py`](agents/orchestrator-runner.py) |
| Qué instala exactamente el instalador y qué hace con una config existente | [`install.sh`](install.sh), [`install.ps1`](install.ps1) |
| Comprobar el paquete entero de una vez | [`check-package.sh`](check-package.sh) |
| Qué fragmento de configuración se añade y cómo se escribe la variante | [`config/opencode.jsonc`](config/opencode.jsonc), [`README.md`](README.md) |
| Requisitos, alcance y criterios de aceptación | [`docs/PRD.md`](docs/PRD.md) |
| Qué cambió en cada versión | [`CHANGELOG.md`](CHANGELOG.md) |
| Ramas, commits, verificación, integración y operaciones con autorización | [`CONTRIBUTING.md`](CONTRIBUTING.md) |
| Catálogo enlazable para agentes | [`llms.txt`](llms.txt) |
| Trabajo del repositorio: crear, mover, bloquear y cerrar tareas | [`.tasks-ai/AGENTS.md`](.tasks-ai/AGENTS.md) |
| Etiquetas: tipos, estructura, ámbito y catálogo | [`.tasks-ai/CONTRATO_ETIQUETAS.md`](.tasks-ai/CONTRATO_ETIQUETAS.md) → «1. Las clases y su convivencia», «2. Asignación correcta a una tarea», «3. Gestión del catálogo» |
| Servidor del tablero en segundo plano | [`serve.sh`](serve.sh) |
| Visor de escritorio del tablero | [`tasks-viewer/`](tasks-viewer/config.json) |

## Encargo → qué leer antes → con qué se cierra

| Encargo | Qué leer antes | Con qué se cierra |
|---|---|---|
| Actualizar un artefacto de agente | [`docs/PRD.md`](docs/PRD.md) sección 4, regla de negocio de las copias | `diff -r agents <config global>/agents` sin diferencias y la verificación de [`CONTRIBUTING.md`](CONTRIBUTING.md) |
| Cambiar los instaladores | [`docs/PRD.md`](docs/PRD.md) requisitos R1 a R7 | La batería de [`CONTRIBUTING.md`](CONTRIBUTING.md): sintaxis, instalación limpia, fusión con claves ajenas y camino JSONC |
| Crear o etiquetar una tarea | [`.tasks-ai/CONTRATO_ETIQUETAS.md`](.tasks-ai/CONTRATO_ETIQUETAS.md) sección 2 | La tarea nace con su tipo y su estructural (`task` o `epic`); el catálogo vivo, en `tasks-ai tags` |
| Cambiar la guía o la prosa | [`docs/web/index.html`](docs/web/index.html) y [`README.md`](README.md) completos | Los criterios de aceptación del PRD, sección 5, aplicados a la documentación: nada que la prosa diga puede contradecir a los artefactos |
| Cambiar el plugin TUI o el contrato `--json` | [`plugins/orchestrator-status/tui.tsx`](plugins/orchestrator-status/tui.tsx), su [`package.json`](plugins/orchestrator-status/package.json) y el `--help` de [`agents/orchestrator-runner.py`](agents/orchestrator-runner.py) | `bash check-package.sh` en verde y el panel abierto con `/orquestador` contra un proyecto con estado |
| Ver la guía web correctamente | [`serve-web.sh`](serve-web.sh) | `./serve-web.sh start` y la guía en `http://127.0.0.1:8766/docs/web/index.html`; por `file://` pierde iconos y resaltado |
| Cambiar requisitos o alcance | [`docs/PRD.md`](docs/PRD.md) completo | El PRD actualizado antes del commit y el CHANGELOG si hay valor de usuario |

## Autoridad

Este mapa no es fuente de nada: enlaza. Si una ruta o una afirmación de este archivo
contradice a la fuente que enlaza, manda la fuente y se corrige aquí.
