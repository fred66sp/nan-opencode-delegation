# Flujo de trabajo Git

Política de este repositorio para ramas, commits, verificación, integración, CHANGELOG y
versiones. Prevalece sobre cualquier modelo genérico. La ceremonia de cada operación
—clasificarla, preparar el stage, presentar el informe y esperar confirmación— la gobierna
la skill `git-flow-management`.

## Modelo

- `main` es la única rama de larga vida; contiene lo que está listo para usarse.
- No hay `develop`, `release/*` ni `hotfix/*`: introducirlos exige cambiar esta política.
- El commit se hace antes de cerrar la tarea en el tablero.

## Ramas

Se crean desde `main` y viven lo mínimo: nombres en minúsculas, palabras separadas con
guiones.

| Patrón | Uso |
|---|---|
| `feature/<slug>` | Funcionalidad nueva |
| `fix/<slug>` | Corrección de un defecto |
| `docs/<slug>` | Documentación |
| `refactor/<slug>` | Reorganización sin cambio de comportamiento |
| `chore/<slug>` | Mantenimiento, herramientas, configuración |

Una tarea pequeña puede ir directamente a `main`. Es pequeña cuando el diff se entiende
leyéndolo, se deshace revirtiendo el commit y no toca datos almacenados, contratos
públicos, seguridad, concurrencia ni código ejecutable en varios módulos, ni mezcla
cambios ajenos. Ante la duda, rama.

Crear una rama local no integra ni publica nada.

## Commits

- Conventional Commits, en inglés: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`,
  `style`, `perf`, `ci`, `build`, `revert`.
- Descripción breve, en imperativo y sin punto final.
- Un commit representa una unidad coherente. Varios solo cuando separan cambios
  independientes o hacen la revisión más clara.
- El cuerpo nombra la tarea que sostiene el commit con `Refs: tk-<n>`, una por tarea. Es
  la única atadura entre tarea y commit; el tablero no guarda hashes:

```bash
git log --format='%h %s' --grep='Refs: tk-1'
```

- Todo cambio de código lleva en el cuerpo una línea de riesgo, `Risk: item N (motivo)` o
  `Risk: none`. Los ítems: 1 datos o efectos irreversibles; 2 seguridad; 3 contratos que
  otros consumen; 4 concurrencia; 5 entrega o entorno; 6 nada que detecte una regresión en
  lo que cambia. Un riesgo alto añade una verificación independiente: otro agente lee el
  criterio de la tarea y lo prueba contra el resultado real.

- Ningún commit con cabos sueltos: lo que se descubre al prepararlo se arregla o se
  pregunta antes, no en un commit de corrección inmediata.
- Sin firmas de IA ni líneas `Co-Authored-By` salvo petición expresa.
- Ningún commit se crea sin confirmación explícita: cubre lo presentado, y un cambio de
  alcance pide otra.

## Verificación

La verificación es proporcional al cambio. La batería completa de este paquete está
automatizada en `bash check-package.sh`, que ejecuta de una vez las comprobaciones que
siguen. Esta lista sigue siendo la definición de la batería y la vía manual cuando no se
quiere o no se puede usar el script:

```bash
bash -n install.sh
python3 -c "import json; json.load(open('config/opencode.jsonc'))"
diff -r -x '*.bak.*' agents "${XDG_CONFIG_HOME:-$HOME/.config}/opencode/agents"   # sin copias .bak (R7)
env -u XDG_CONFIG_HOME HOME=$(mktemp -d) bash install.sh   # instalación limpia
```

La batería incluye el comprobador del retorno: su sintaxis (`python3 -m py_compile
agents/delegation-report-checker.py`), el contrato de su `--help` y cinco informes de prueba
—uno conforme que sale con 0, con una anáfora que su ruta resuelve, y cuatro no conformes
que salen con 1: sin `status`, sin una comprobación esperada (con `--json`), con dos
`status` contradictorios y con una anáfora sin ruta. Valida además mecánicamente el
`.run/delegations.jsonl` del proyecto si existe —los seis campos y sus conjuntos por
línea— y pasa si el proyecto no lo tiene, y comprueba que la copia embebida del runner
en `docs/web/index.html` —el interior del `code` del modal— sigue
coincidiendo byte a byte con `agents/orchestrator-runner.py`.
Un cambio en el checker o en el contrato del informe se cierra con `bash check-package.sh`.

Y, cuando la fusión o el camino JSONC cambien: instalación sobre una configuración con
claves ajenas (deben conservarse y nacer copia `.bak`) y sobre una con comentarios (el
fichero debe quedar intacto y el bloque a mano impreso). `install.ps1` admite el mismo
camino en una máquina Windows; desde WSL, con el `powershell.exe` del propio Windows y
`XDG_CONFIG_HOME` temporal, cubre instalación limpia, fusión y migración. Toda verificación no
ejecutada se declara.

## Integración

- Se integra en `main` con merge no fast-forward: `git merge --no-ff <rama>`.
- Sin `squash` ni `rebase` sobre ramas ya publicadas.
- Antes: la tarea terminada, las comprobaciones correctas, ningún cambio ajeno que se
  pierda y la rama correcta.
- Después: se comprueba estado, rama y que el commit está en `main`.
- Una rama con trabajo no integrado no se borra sin autorización.

## CHANGELOG

- Se anota lo que tiene valor para quien usa el paquete, al publicar una versión, bajo
  `## [Sin publicar]`, con las secciones `Añadido`, `Modificado` y `Corregido`.
- No se actualiza por cada commit ni por cambios internos de prosa.

## Versiones y etiquetas

- Versionado semántico cuando haya versión publicada o se pida una: `PATCH` para una
  corrección compatible, `MINOR` para funcionalidad compatible y `MAJOR` para un cambio
  incompatible.
- La etiqueta es anotada, con prefijo `v`, y marca el commit integrado.
- Crear una etiqueta, preparar una release o publicar exige autorización explícita.

## Operaciones que exigen autorización

- Fusionar en `main`.
- `push`, `fetch` para integrar y cualquier otra operación remota.
- Crear una etiqueta o publicar una versión.
- `reset`, `clean`, `rebase`, `filter-repo` u otra operación que elimine o reescriba
  trabajo.
- Sobrescribir, descartar u ocultar cambios de la persona.
- Borrar una rama con trabajo no integrado.

Una autorización cubre lo que nombra y no se hereda a otra operación.

## Principio

Git aporta seguridad y trazabilidad sin convertirse en una carga. El orden de prioridad:
no perder trabajo, entregar algo verificable, mantener un historial comprensible, pedir
autorización para lo persistente y no añadir ramas, commits ni comprobaciones que no
aporten.
