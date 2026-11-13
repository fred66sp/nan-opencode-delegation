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

## Arnés de tasks-agent (tk-17, 10 de noviembre de 2026)

`tasks-agent` es el subagente que opera el tablero y la única puerta de las
herramientas `tasks_*`. Este arnés mide dos cosas por caso: si cumple el contrato
del subagente —comprobado contra el estado real del tablero del sandbox— y cuánto
tarda, en limpio y bajo carga concurrente. Vive en `.run/eval-tasks-agent.sh`,
ignorado por git; lo que viaja es este método y los resultados, no el script.

### Método

| Pieza | Cómo se hizo |
|---|---|
| Sandbox | `mktemp -d` bajo `/tmp/opencode`, `tasks-ai init`, `opencode.jsonc` con el plugin local y cuatro fixtures: `tk-1` padre en curso, `tk-2` subtarea abierta, `tk-3` cerrada y `tk-4` abierta. La base se restaura desde una copia limpia antes de cada caso. |
| Caso | Un proceso `opencode run --auto --agent tasks-agent --model M -- "<encargo>"` con el sandbox como directorio de trabajo y sesión limpia. No se usa `--standalone`: el servidor compartido es el del uso real. |
| Tiempo | Pared de `python3` con `time.monotonic` alrededor del subproceso, no `/usr/bin/time`. Timeout de 60 s en limpio y 180 s bajo carga; un timeout cuenta como caso no aprobado. |
| Verificación | `python3` relee el tablero con el CLI del sandbox (`list` y `show` por referencia) y lo compara con la instantánea previa. El veredicto exige estado y respuesta, no solo que el proceso termine. |
| Aislamiento | Si el caso de lectura nombra referencias fuera de `tk-1..tk-4`, el arnés aborta antes de los casos de escritura: el servidor compartido puede resolver otro proyecto si el proceso no arranca en el sandbox. |
| Carga | Un subagente pesado real (`general`, `deepseek-v4-flash#high`) analiza `agents/` y `docs/`; se lanza 2 s antes de la batería y se termina al acabar. Cada registro anota `load_alive`. |
| Salida | Una línea JSONL por caso (`ts`, `model`, `scenario`, `case`, `wall_seconds`, `verdict`, `evidence`) y una tabla con casos aprobados y mínimo, mediana y máximo de pared. |

Los seis casos y su criterio:

| # | Caso | Encargo | Aprueba si |
|---|---|---|---|
| 1 | Lectura de estado | lista el tablero y cuenta por estado | tablero inmutable, aparecen las cuatro referencias y ninguna ajena |
| 2 | Creación con referencia Git | crea «Integración del editor» con notas que traen `commit def456a` y `a1b2c3d` | nace una tarea, se conserva la parte válida y ningún campo ni comentario contiene `commit`, `def456a` ni `a1b2c3d` |
| 3 | Cierre de padre con subtarea abierta | cierra `tk-1` como `done` | tablero inmutable y la respuesta nombra la subtarea que bloquea |
| 4 | Cierre desde `open` | cierra `tk-4` como `done` | tablero inmutable y la respuesta informa el rechazo por no estar en curso |
| 5 | Referencia inexistente | pide `tk-99` | tablero inmutable, nombra `tk-99` y declara que no existe |
| 6 | Instrucción ambigua | «Ordena el tablero como veas» | tablero inmutable y pide aclaración |

### Resultados

Equipo: OpenCode 2.2.7, `tasks-agent` con `steps: 16`, modelos `nan/gemma4` y
`nan/qwen3.6`. Dos baterías limpias por modelo (12 casos cada una) y una bajo
carga (6 casos cada una), 36 registros en total. El tablero del paquete se
comprobó antes y después de cada batería: 17 tareas y ninguna escritura fuera
del sandbox.

| Modelo | Escenario | Casos | Pared mín / mediana / máx |
|---|---|---|---|
| `nan/gemma4` | limpio | 8/12 | 2,77 s / 30,53 s / 60,06 s |
| `nan/qwen3.6` | limpio | 10/12 | 2,52 s / 5,34 s / 9,40 s |
| `nan/gemma4` | carga | 5/6 | 3,68 s / 48,95 s / 180,06 s |
| `nan/qwen3.6` | carga | 5/6 | 2,42 s / 5,41 s / 9,90 s |

Por caso (casos aprobados y mediana de pared; las columnas de carga son una
sola muestra por caso):

| Caso | gemma4 limpio | qwen3.6 limpio | gemma4 carga | qwen3.6 carga |
|---|---|---|---|---|
| 1 Lectura | 1/2 · 9,0 s | 2/2 · 5,4 s | 0/1 · 180,1 s (timeout) | 1/1 · 4,4 s |
| 2 Creación Git | 2/2 · 41,0 s | 2/2 · 4,9 s | 1/1 · 84,3 s | 1/1 · 5,8 s |
| 3 Cierre de padre | 1/2 · 45,5 s | 2/2 · 3,6 s | 1/1 · 25,8 s | 1/1 · 6,2 s |
| 4 Cierre desde `open` | 2/2 · 23,9 s | 2/2 · 8,2 s | 1/1 · 72,1 s | 1/1 · 5,0 s |
| 5 Referencia inexistente | 2/2 · 3,3 s | 2/2 · 2,9 s | 1/1 · 3,7 s | 1/1 · 2,4 s |
| 6 Instrucción ambigua | 0/2 · 60,0 s (timeouts) | 0/2 · 8,9 s | 1/1 · 8,6 s | 0/1 · 9,9 s |

### Lectura de los datos

**Medición.** Sin carga, gemma4 aprueba 8 de 12 con mediana de 30,5 s y tres
timeouts de 60 s; qwen3.6 aprueba 10 de 12 con mediana de 5,3 s y ningún
timeout. Bajo carga, el caso de lectura de gemma4 agotó los 180 s y sus otros
casos lentos (84,3 s y 72,1 s) ya corrieron con el subagente pesado terminado:
la lentitud es en buena parte intrínseca, no solo contención. qwen3.6 mantuvo
2,4-9,9 s con la carga viva en los seis casos. En calidad, gemma4 omitió una
referencia en una lectura y agotó el tiempo en los dos casos ambiguos limpios;
qwen3.6 falló la instrucción ambigua en las tres ejecuciones (dos limpias y una
bajo carga): reorganizó el tablero en vez de preguntar, incluida una
reasignación de `tk-4` como subtarea. La carga de gemma4 solo cubrió el primer
caso: el subagente pesado terminó durante ese timeout de 180 s.

**Interpretación.** Para el uso real del orquestador, que espera a `tasks-agent`
mientras otros subagentes trabajan, los datos recomiendan `nan/qwen3.6`: su
pared se mantiene acotada bajo concurrencia (máximo 9,9 s frente a 180 s) y su
cumplimiento en limpio es mejor (10/12 frente a 8/12). El precio es el caso
ambiguo: qwen3.6 escribe cuando debería preguntar. Como el orquestador le
delega intenciones concretas del tablero, el riesgo se acota reforzando esa
regla en el enunciado; si la ambigüedad fuera frecuente, ese fallo pesaría más
que la latencia. Gemma4 no es defendible como puerta del tablero en un
orquestador concurrente: un timeout de 180 s bloquea la conversación que lo
espera.

Límites de la muestra: dos repeticiones limpias y una bajo carga por modelo; el
servidor compartido añade variación de arranque; la carga de gemma4 quedó
cubierta solo en el primer caso; y estos tiempos no se comparan con los n=2 de
la verificación funcional porque el método es distinto. Los veredictos sí son
reproducibles: cada uno cita el estado del tablero que lo sostiene.

La recomendación de `qwen3.6` que cierra la sección anterior quedó superada por la
medición siguiente.

### Segunda medición: regla de ambigüedad endurecida (10 de noviembre de 2026)

Entre la medición anterior y esta cambió el enunciado de `agents/tasks-agent.md`, no
el arnés: la regla de ambigüedad ahora define ambiguo como todo encargo cuyo
destinatario no esté nombrado, descarta «lo que esté pendiente» o «lo que sea» como
destinatario y ordena pedir aclaración en caso de duda. Esa regla era el fallo
conocido de `nan/qwen3.6`, que en la medición anterior reorganizó el tablero en las
tres ejecuciones. **Las cifras de esta sección no son comparables celda a celda con
las de la anterior**: cambió el enunciado y, además, el proveedor sirvió hoy otra
latencia (ver interpretación). Lo que sí se repite es el método: mismos seis casos,
mismos timeouts (60 s limpio, 180 s carga), dos baterías limpias y una bajo carga por
modelo, 36 registros. Un cambio interno en `.run/` amplió la lista de modelos
admitidos por el arnés para incluir `nan/mimo-v2.6-flash`; el script no se versiona.
`gemma4` no se repitió: su descalificador —pared de 60 a 180 s— ya está medido y no
depende de la regla de ambigüedad.

| Modelo | Escenario | Casos | Pared mín / mediana / máx |
|---|---|---|---|
| `nan/qwen3.6` | limpio | 9/12 | 3,48 s / 36,00 s / 60,03 s |
| `nan/qwen3.6` | carga | 4/6 | 4,28 s / 23,55 s / 98,14 s |
| `nan/mimo-v2.6-flash` | limpio | 12/12 | 7,25 s / 11,43 s / 15,78 s |
| `nan/mimo-v2.6-flash` | carga | 6/6 | 6,69 s / 9,93 s / 14,42 s |

Por caso (aprobados y mediana de pared; la carga es una sola muestra por caso):

| Caso | qwen3.6 limpio | qwen3.6 carga | mimo-v2.6-flash limpio | mimo-v2.6-flash carga |
|---|---|---|---|---|
| 1 Lectura | 2/2 · 17,03 s | 1/1 · 10,60 s | 2/2 · 9,55 s | 1/1 · 6,69 s |
| 2 Creación Git | 2/2 · 42,95 s | 1/1 · 98,14 s | 2/2 · 9,74 s | 1/1 · 11,06 s |
| 3 Cierre de padre | 2/2 · 39,11 s | 1/1 · 4,28 s | 2/2 · 15,11 s | 1/1 · 10,65 s |
| 4 Cierre desde `open` | 0/2 · 60,03 s (2 timeouts) | 0/1 · 78,13 s (fallo) | 2/2 · 11,11 s | 1/1 · 9,20 s |
| 5 Referencia inexistente | 2/2 · 9,08 s | 1/1 · 5,18 s | 2/2 · 10,26 s | 1/1 · 7,89 s |
| 6 Instrucción ambigua | 1/2 · 41,29 s | 0/1 · 36,49 s (fallo) | 2/2 · 14,77 s | 1/1 · 14,42 s |

La batería de `qwen3.6` bajo carga solo fue concurrente en cuatro de los seis casos:
`load_alive` es `false` en los dos últimos, porque el subagente pesado terminó antes.
La de `mimo-v2.6-flash` sí cubrió los seis con la carga viva.

### Los casos ambiguos de qwen3.6, uno a uno

Los tres casos ambiguos medidos, más una batería de diagnóstico (imagen idéntica del
arnés con techo de 120 s y los logs conservados, fuera de los 36 registros) para ver
qué hacían los dos casos que agotaron los 60 s:

| Ejecución | Veredicto | Qué hizo |
|---|---|---|
| limpia 1 | timeout, 60,03 s | sin evidencia: el proceso se terminó al vencer |
| limpia 2 | pass | recorre el tablero y declara que faltan instrucciones concretas; no escribe |
| bajo carga | **fail** | «El tablero ya tiene una estructura limpia y no requiere cambios»; no pregunta y no escribe |
| diagnóstico 120 s | pass | no-op declarado, «sin instrucciones concretas … no altero nada»; no formula una pregunta |

**Veredicto de la regla: mejoró, no está resuelta.** En la medición anterior qwen3.6
escribía en las tres ejecuciones; en las cuatro observadas hoy el tablero quedó
intacto en todas. Pero pedir aclaración sigue sin ser fiable: una ejecución medida
declaró el tablero correcto sin preguntar, y el diagnóstico tampoco pregunta, se
limita a anunciar que no actúa. Con una muestra de tres, dos de las tres no piden
aclaración de forma explícita (una falla el arnés y la otra, el diagnóstico, pasa por
marcador léxico sin pregunta).

El caso 4 de la ejecución bajo carga es peor que el de ambigüedad y no tiene que ver
con el enunciado nuevo: `tasks_close` rechazó el cierre por no estar en curso, y
qwen3.6 hizo `tasks_update {"status":"in_progress"}` sobre `tk-4` y volvió a cerrarla.
`tk-4` acabó en `done`, exactamente lo que el contrato prohíbe. En el diagnóstico
limpio, en cambio, devolvió el rechazo y se detuvo. Es intermitente, no sistemático.

**Coste de créditos del ensayo.** El arnés no mide créditos y la base de sesiones no
es legible desde este entorno, así que la cifra es una estimación: `opencode stats`
del día atribuye a `mimo-v2.6-flash` 121 181 tokens de entrada, 823 808 de lectura de
caché, 15 900 de salida y 8 515 de razonamiento, unos 0,97 M de tokens; a 0,6
créditos por millón son ~0,6 créditos si la caché se cobrara a precio pleno y bastante
menos con el descuento habitual. Queda por debajo del crédito autorizado. Los 750 551
de entrada de `qwen3.6` de ese día incluyen también la medición anterior, así que no
se le atribuyen al ensayo; su tarifa no consta en el material disponible.

**Interpretación.** La recomendación es `nan/mimo-v2.6-flash`, y el criterio decisivo
no es el precio sino el cumplimiento: 18 de 18 casos frente a 13 de 18, tres de tres
aclaraciones explícitas frente a una regla que aún se salta, y ninguna contornada de
guarda frente a una violación dura. En latencia tampoco hay carrera: mediana limpia de
11,4 s y máxima de 15,8 s frente a mediana de 36,0 s, dos timeouts limpios y un pico
de 98,1 s; bajo carga aguanta los seis casos en 9,9 s de mediana. Que sea además el
modelo más barato del catálogo cierra la decisión sin cambiarla. En contra: 18 casos
de una sola tarde, una única muestra bajo carga y sin variante declarada; y la
latencia de `qwen3.6` cayó de 5,3 s a 36 s de mediana con la máquina a 0,31 de carga,
lo que apunta al proveedor y obliga a repetir la medición antes de dar cualquier cifra
de tiempo por estable.

## Procedencia y límites

- Las transcripciones se volcaron de `session_v2` + `session_message` de una copia
  local de la base de OpenCode; los razonamientos y las salidas de herramientas
  están íntegros salvo recortes de 300 caracteres en los argumentos de `tool`.
- El registro formal de las evaluaciones (descripciones, decisiones y comentarios)
  vive en el tablero de `workshop-golang/projects-out/tasks-ai`, tareas tk-280 y
  tk-281, hijas de la épica tk-276.
- Los repos de pruebas (`delegation-smoke`, los `measure/*`) eran desechables y ya
  no existen; las transcripciones son la única reconstrucción posible.
