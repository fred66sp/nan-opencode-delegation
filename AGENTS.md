# AGENTS.md

Este paquete se distribuye ya instalado o revisable por agentes. Este fichero dice dónde
vive cada fuente canónica y qué leer antes de cada encargo; no guarda contenido, solo
enlaza.

## Qué se busca → dónde vive

| Qué se busca | Dónde vive |
|---|---|
| Qué es el paquete y cómo se instala | [`README.md`](README.md) y [`index.html`](index.html) |
| Qué hace cada agente, con qué modelo y qué permisos | `agents/<nombre>.md`, frontmatter y prompt |
| Qué instala exactamente el instalador y qué hace con una config existente | [`install.sh`](install.sh), [`install.ps1`](install.ps1) |
| Qué fragmento de configuración se añade y cómo se escribe la variante | [`config/opencode.jsonc`](config/opencode.jsonc), [`README.md`](README.md) |
| Requisitos, alcance y criterios de aceptación | [`docs/PRD.md`](docs/PRD.md) |
| Qué cambió en cada versión | [`CHANGELOG.md`](CHANGELOG.md) |
| Ramas, commits, verificación, integración y operaciones con autorización | [`CONTRIBUTING.md`](CONTRIBUTING.md) |
| Catálogo enlazable para agentes | [`llms.txt`](llms.txt) |
| Trabajo del repositorio: crear, mover, bloquear y cerrar tareas | [`.tasks-ai/AGENTS.md`](.tasks-ai/AGENTS.md) |
| Servidor del tablero en segundo plano | [`serve.sh`](serve.sh) |
| Visor de escritorio del tablero | [`tasks-viewer/`](tasks-viewer/config.json) |

## Encargo → qué leer antes → con qué se cierra

| Encargo | Qué leer antes | Con qué se cierra |
|---|---|---|
| Actualizar un artefacto de agente | [`docs/PRD.md`](docs/PRD.md) sección 3, regla de negocio de las copias | `diff -r agents <config global>/agents` sin diferencias y la verificación de [`CONTRIBUTING.md`](CONTRIBUTING.md) |
| Cambiar los instaladores | [`docs/PRD.md`](docs/PRD.md) requisitos R1 a R6 | La batería de [`CONTRIBUTING.md`](CONTRIBUTING.md): sintaxis, instalación limpia, fusión con claves ajenas y camino JSONC |
| Cambiar la guía o la prosa | [`index.html`](index.html) y [`README.md`](README.md) completos | Los criterios de aceptación del PRD, sección 5, aplicados a la documentación: nada que la prosa diga puede contradecir a los artefactos |
| Cambiar requisitos o alcance | [`docs/PRD.md`](docs/PRD.md) completo | El PRD actualizado antes del commit y el CHANGELOG si hay valor de usuario |

## Autoridad

Este mapa no es fuente de nada: enlaza. Si una ruta o una afirmación de este archivo
contradice a la fuente que enlaza, manda la fuente y se corrige aquí.
