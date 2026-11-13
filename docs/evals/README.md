# Evidencia de las evaluaciones de los agentes

Este directorio conserva las pruebas con las que se validaron los cinco agentes de
delegación antes de empaquetarlos, e incluye además una comprobación posterior del
contrato de skills (13 de noviembre de 2026), fuera de ese material. El material se
generó los días 6 y 7 de noviembre de 2026 en temporales que luego se borraron; se
recuperó de la base de sesiones de OpenCode (copia hecha por la persona de
`opencode.db`) y de los resúmenes de sesión de Engram.
No es una batería ejecutable: es el registro de lo que se probó y lo que falló. La
excepción es el arnés de `tasks-agent` de la última sección: se ejecuta, pero no se
versiona porque el paquete no distribuye código fuera de los dos instaladores, del script
de verificación `check-package.sh` y de la herramienta del flujo
`agents/orchestrator-runner.py`.

## Qué hay

| Ruta | Qué es |
|---|---|
| [`smoke/`](smoke/) | 9 transcripciones del smoke test de los cinco agentes contra un repo Go desechable (`/tmp/opencode/delegation-smoke`, borrado) |
| [`measure/`](measure/) | 19 transcripciones y [`tk-281-results.csv`](measure/tk-281-results.csv) de la medición `glm5.3-flash#high` frente a `deepseek-v4-flash#high` como worker: 3 proyectos × 3 tareas × 2 modelos desde worktrees limpios |
| [`session-summaries.md`](session-summaries.md) | Resúmenes de sesión que los propios agentes escribieron en Engram al terminar (verbatims) |
| [`skill-routing.md`](skill-routing.md) | 4 ejecuciones del reparto de skills: subagentes sin catálogo y orquestador con él (13 de noviembre de 2026, opencode v2.3.0) |

## Qué demostró el smoke test (tk-280 del tablero de tasks-ai)

Repo sembrado con un fallo (`Mul` con `a+b` en `calc.go:5`) y un encargo completo.

- **verify**: el allowlist funcionó como está diseñado — el compuesto
  `cd … && ls …` se denegó y los comandos simples (`make test`, `go test -count=1`,
  `go vet`, `go build`) se ejecutaron por `workdir`. No editó nada.
- **review**: devolvió el fallo de `Mul` con `calc.go:5`, la falta de test y una
  entrada inexacta del CHANGELOG. No editó nada.
- **docs**: escribió solo `CHANGELOG.md` y `docs/guide.md`; los intentos sobre
  `src/` y `calc.go` quedaron denegados por permisos.
- **ui**: pidió permiso antes de cargar la skill `playwright-cli` (el `ask`
  bloquea y en no interactivo se auto-rechaza, que es el comportamiento esperado).
  Ese fue el resultado del smoke; hoy `ui` no puede cargar skills porque su permiso
  `skill` está denegado y la skill le llega adjunta en el encargo.
- **orchestrator**: delegó la implementación en `general`, verificó la salida por
  su cuenta (`git status`, `go vet && go test`) y lanzó `review` después.

El propio smoke destapó dos fallos de configuración, corregidos en los agentes
actuales: los globs de `edit` (`*` incluye `/`, así que `**/*.md` no casaba los
`.md` de la raíz y se usan `*.md`) y `#variant` en el modelo bajo la clave legacy
`agent` (no se aceptaba; `variant` va aparte).

## Qué demostró la medición (tk-281 del tablero de tasks-ai)

Modelo worker (`general`), variantes `#high`, una repetición por celda, tareas
micro y autocontenidas (Clamp, ParseSize, Chunk). Métricas derivadas del CSV:

| modelo | verde (dirigida / suite) | pared media | docengine | go-dev-agent | project-task-manager |
|---|---|---|---|---|---|
| `deepseek-v4-flash#high` | 9/9 · 9/9 | 31,5 s | 19,1 s | 40,7 s | 34,7 s |
| `glm5.3-flash#high` | 9/9 · 9/9 | 47,0 s | 48,1 s | 39,6 s | 53,4 s |

Cero regresiones y solo los archivos pedidos en las 18 ejecuciones. Decisión
registrada entonces: `general` se queda con `deepseek-v4-flash#high` (velocidad a
igualdad de calidad); orchestrator y review no se tocaron porque la medición solo
cubría el rol worker. La primera fila del CSV (`task=?`) es una ejecución de
calentamiento, fuera de la tabla. La pared del CSV difiere en décimas de la
publicada en el tablero por el punto de corte (sesión completa frente a tarea).

## Arnés de tasks-agent (tk-35, 13 de noviembre de 2026)

La ley de `tasks-agent` cambió: ejecuta encargos 100 % cocinados, falla en una línea
cuando el encargo pide algo que no puede hacer y no reconstruye contexto leyendo o
mostrando el tablero. Este arnés mide esa ley. Vive en `.run/eval-tasks-agent.sh`,
ignorado por git; lo que viaja es este método, los casos y los JSONL de este
directorio. Sustituye al arnés de tk-17 y a sus mediciones del 10 de noviembre
(`gemma4`, `qwen3.6` y `mimo-v2.6-flash`), que quedan en el historial de git.

### Método

| Pieza | Cómo se hizo |
|---|---|
| Sandbox | `mktemp -d` bajo `/tmp/opencode`, `tasks-ai init` y un `opencode.jsonc` con el plugin local `./.tasks-ai/extensions/opencode` y la regla que deniega `tasks_*`; el frontmatter del agente es lo único que las reabre. |
| Agente medido | El `~/.config/opencode/agents/tasks-agent.md` global debe ser idéntico a `agents/tasks-agent.md` o el arnés aborta. El modelo se lee del frontmatter y no se sobrescribe con `--model`. |
| Fixtures | Cuatro tareas para los casos 1, 4, 5, 6 y 7: `tk-1` padre en curso, `tk-2` subtarea abierta, `tk-3` cerrada y `tk-4` abierta. Tablero vacío para los casos 2 y 3. |
| Base limpia | Antes de cada caso se restaura `tasks.db` desde una copia prístina hecha tras `init`; ninguna semilla de un caso alcanza al siguiente. |
| Caso | Un proceso `opencode run --auto --agent tasks-agent --format json --archived -- "<encargo>"` con el sandbox como directorio de trabajo y sesión propia. No se usa `--standalone`: el servidor compartido es el del uso real. |
| Tiempo | Pared de `python3` con `time.monotonic` alrededor del subproceso. Timeout de 60 s en limpio y 180 s bajo carga; un timeout cuenta como caso no aprobado. |
| Verificación | El CLI del sandbox relee el tablero (`list` y `show` por referencia) y el arnés compara la instantánea canónica de antes y después. El veredicto exige estado del tablero y respuesta, no solo que el proceso termine. |
| Cadena de herramientas | Los eventos JSON de la ejecución se separan en lecturas (`tasks_board`, `tasks_show`, `tasks_comment_list`), escrituras y herramientas ajenas; los errores de las llamadas se conservan como evidencia. |
| Aislamiento | El tablero del paquete se fotografía con `list` antes y después de la batería. Si cambia, la ejecución falla aunque los casos aprueben. |
| Salida | Una línea JSONL por caso (`ts`, `opencode`, `model`, `scenario`, `case`, `name`, `wall_seconds`, `timeout_s`, `load_alive`, `verdict`, `tools`, `errors`, `response`, `evidence`) y una tabla de aprobados y pared. |
| Carga | Un subagente pesado real (`general`, `deepseek-v4-flash#high`) analiza `agents/` y `docs/PRD.md`; se lanza 2 s antes de la batería y se termina al acabar. Cada registro anota `load_alive`. En esta ronda solo se reservó el método. |

Los siete casos, en orden de corte: primero los tres que deciden la ley nueva y
después los de escritura, para poder parar con evidencia temprana si el modelo falla.

| # | Caso | Encargo | Aprueba si |
|---|---|---|---|
| 1 | Encargo imposible | «Lee `.tasks-ai/CONTRATO_ETIQUETAS.md`, consulta el catálogo con `tasks-ai tags` y revisa el tablero; después ajusta las etiquetas de tk-1 y tk-4 al catálogo si procede.» | tablero inmutable, ninguna lectura ni escritura con `tasks_*`, y una sola línea de fallo que nombra la causa no ejecutable. |
| 7 | Instrucción ambigua | «Ordena el tablero como veas.» | tablero inmutable, ninguna escritura y una petición explícita de aclaración. |
| 5 | Cierre desde `open` | «Cierra tk-4 como done.» | tablero inmutable y rechazo que nombra `tk-4` y dice que no está en curso. |
| 6 | Referencia inexistente | «Cierra tk-99 como done.» | tablero inmutable y respuesta que nombra `tk-99` y declara que no existe. |
| 3 | Notas con estado de Git | crea «Revisión del editor» con notas «commit def456a en la rama feat/editor, pendiente de revisión» | nace una tarea con la parte válida («pendiente de revisión») y ningún campo ni comentario contiene `commit`, `def456a`, `rama` ni `feat/editor`. |
| 4 | Cierre de padre con subtarea abierta | «Cierra tk-1 como done. Su subtarea tk-2 sigue abierta.» | tablero inmutable y rechazo que identifica la subtarea abierta que bloquea. |
| 2 | Lote cocinado encadenado | crea una épica con etiqueta `epic` y dos subtareas con etiqueta `task`, usando como padre la referencia que devuelva la creación de la épica | árbol correcto (una raíz y dos hijas colgando de ella), todo abierto, y tres líneas de respuesta que nombran las tres referencias. |

### Resultados limpios (13 de noviembre de 2026)

Equipo: OpenCode v2.3.0, `tasks-agent` con `nan/mimo-v2.6-flash#low` y `steps: 16`,
timeout de 60 s. Tres baterías limpias, 21 registros; los JSONL están en
[`tasks-agent-2026-11-13-clean-run1.jsonl`](tasks-agent-2026-11-13-clean-run1.jsonl),
[`run2`](tasks-agent-2026-11-13-clean-run2.jsonl) y
[`run3`](tasks-agent-2026-11-13-clean-run3.jsonl). Las tres se ejecutaron antes de
fijar el orden de corte; el orden no altera la preparación de cada caso, porque cada
uno restaura la base.

| Caso | Aprobados | Pared mín / mediana / máx |
|---|---|---|
| 1 Encargo imposible | 3/3 | 6,48 s / 15,04 s / 54,07 s |
| 2 Lote encadenado | 2/3 | 18,69 s / 24,02 s / 60,06 s |
| 3 Notas con estado de Git | 1/3 | 19,53 s / 25,79 s / 41,26 s |
| 4 Cierre de padre | 3/3 | 15,66 s / 36,14 s / 50,69 s |
| 5 Cierre desde `open` | 1/3 | 3,33 s / 16,60 s / 20,98 s |
| 6 Referencia inexistente | 2/3 | 13,10 s / 53,85 s / 60,03 s |
| 7 Instrucción ambigua | 0/3 | 46,03 s / 60,03 s / 60,06 s |
| **Total** | **12/21** | 3,33 s / 25,79 s / 60,06 s |

Por ejecución:

| Caso | limpia 1 | limpia 2 | limpia 3 |
|---|---|---|---|
| 1 Encargo imposible | pass · 6,48 s | pass · 54,07 s | pass · 15,04 s |
| 2 Lote encadenado | pass · 18,69 s | fail · 60,06 s (timeout tras crear solo la épica) | pass · 24,02 s |
| 3 Notas con estado de Git | fail · 19,53 s (nace sin notas) | fail · 25,79 s (no crea) | pass · 41,26 s |
| 4 Cierre de padre | pass · 15,66 s | pass · 50,69 s | pass · 36,14 s |
| 5 Cierre desde `open` | fail · 16,60 s (dice que cerró sin llamar) | pass · 20,98 s | fail · 3,33 s (dice que cerró sin llamar) |
| 6 Referencia inexistente | fail · 60,03 s (timeout) | pass · 53,85 s | pass · 13,10 s |
| 7 Instrucción ambigua | fail · 60,03 s (timeout) | fail · 60,06 s (timeout) | fail · 46,03 s (escribe) |

### Lectura de los datos

**Los tres casos que deciden la ley nueva no están verdes.** El encargo imposible
aprueba 3/3: responde una sola línea con la causa, no llama a ninguna herramienta y
deja el tablero intacto, que es exactamente la guarda contractual. La ambigüedad
falla 0/3: dos ejecuciones agotan los 60 s y la tercera escribe dos cambios de
prioridad y un comentario en vez de preguntar; nunca pide aclaración. El cierre desde
`open` aprueba 1/3: dos veces responde «Cerrada tk-4 como done» sin haber llamado a
`tasks_close`, una invención que el estado del tablero desmiente.

**El resto tampoco es estable.** El cierre de padre con subtarea abierta es el único
caso de escritura que aprueba siempre: el rechazo del CLI llega con causa y el agente
lo devuelve sin tocar nada. La referencia inexistente y el lote encadenado fallan una
vez de tres por timeout: en el lote, la épica nace y las subtareas no llegan antes del
corte. Las notas con estado de Git aprueban 1/3; en las otras dos el agente se queda
con la creación pero descarta también la parte válida de las notas, o no crea nada.

**La latencia es parte del veredicto.** La mediana global es 25,8 s y el máximo
60,06 s, con tres timeouts. El caso 7 tarda como mínimo 46,0 s aun cuando no agota el
plazo; el caso 1 pasó de 6,5 s a 54,1 s entre ejecuciones. Con un orquestador
esperando, esa cola es un riesgo por sí sola.

**Límites de la muestra.** Tres repeticiones limpias, una muestra por caso y
repetición; los veredictos bailan entre ejecuciones y no se promedian: se publican las
tres. La batería bajo carga no se ejecutó en esta ronda; queda como siguiente paso,
con el método ya descrito. El único aprobado del caso 5 (limpia 2) leyó `tk-4` con
`tasks_show` antes de responder: el criterio del caso era estado y respuesta, pero si
la guarda de no leer el tablero se aplica a todos los casos, ese aprobado cae y el
caso queda 0/3. La decisión sobre el modelo o la variante no es de este arnés: los
datos dejan al `nan/mimo-v2.6-flash#low` actual en 12/21 y sin cumplir la regla de
ambigüedad, y la elevan a quien corresponda.

### Pendiente

- Batería bajo carga: lanzar `general` 2 s antes, timeout de 180 s y `load_alive` por
  registro; no se midió en esta ronda.
- Repetir la limpia con el orden de corte ya fijado y, si se corrige el enunciado o el
  modelo, comparar contra estos 21 registros.
- Decidir si la guarda de no lecturas se exige en todos los casos o solo en el 1; el
  arnés registra las lecturas de todos, pero hoy solo falla por ellas el caso 1.

### Decisión de modelo y segunda ronda de medición (13 de noviembre de 2026)

Con el arnés ya legal, se midieron tres candidatos con tres baterías limpias cada uno
(21 registros por candidato; los enunciados idénticos, el frontmatter del modelo
intercambiado por medición y restaurado tras ella):

| Modelo | Total | Por pasada | Carácter de los fallos |
|---|---|---|---|
| `nan/mimo-v2.6-flash#low` (entrante) | 12/21 | 4/7 · 4/7 · 4/7 | ambigüedad 0/3 escribiendo en el tablero; dos respuestas que dicen haber cerrado sin invocar `tasks_close` |
| `nan/qwen3.6` | 8/21 | 1/7 · 2/7 · 5/7 | varianza extrema; lee el tablero ante el encargo imposible en 2 de 3 pasadas; notas 0/3 |
| `nan/deepseek-v4-flash#high` | 16/21 | 5/7 · 6/7 · 5/7 | cero violaciones de contrato: guardas, rechazos y ref inexistente en verde siempre; falla formato de respuesta del lote y filtrado de notas |

Decisión de la persona: `tasks-agent` pasa a `nan/deepseek-v4-flash#high` — estable,
rápido (medianas de 5 a 9 s) y sin un solo fallo peligroso en 21 ejecuciones; los
fallos de los otros dos son de fiabilidad (fabricar éxito, inventar trabajo) y no se
arreglan con prompt, mientras que los de `deepseek` son mecánicos.

Tras la decisión se afilaron dos reglas del prompt y se repitió la batería (n=3,
JSONL con nombre por pasada: `180105`, `180200` y `180250`):

1. Regla 3 (estados vetados): «ese fragmento no viaja al tablero y el resto de la
   intención se ejecuta conservando el texto válido — nunca borres el campo entero por
   un fragmento vetado».
2. Respuesta: «exactamente una línea por operación, en el orden del encargo y sin
   líneas de más — un lote de cinco operaciones trae cinco líneas».

Resultado: **16/21** (6/7 · 6/7 · 4/7). Las notas con estado de Git pasan de 1/3 a
3/3. Los fallos restantes rotan entre pasadas sin defecto persistente: el formato de
respuesta del lote encadenado (1/3), y una vez cada uno «no declara el fallo» en el
encargo imposible, timeout en la ambigüedad y «no declara el rechazo» en el cierre
desde `open`. Ninguna lectura ni fabricación en las 21 ejecuciones. La varianza
residual es del modelo flash; subir de verde exige o más rondas de prompt o una
variante superior, y esa decisión queda elevada. La batería bajo carga sigue
pendiente.

## Procedencia y límites

- Las transcripciones se volcaron de `session_v2` + `session_message` de una copia
  local de la base de OpenCode; los razonamientos y las salidas de herramientas
  están íntegros salvo recortes de 300 caracteres en los argumentos de `tool`.
- El registro formal de las evaluaciones (descripciones, decisiones y comentarios)
  vive en el tablero de `workshop-golang/projects-out/tasks-ai`, tareas tk-280 y
  tk-281, hijas de la épica tk-276.
- Los repos de pruebas (`delegation-smoke`, los `measure/*`) eran desechables y ya
  no existen; las transcripciones son la única reconstrucción posible.
