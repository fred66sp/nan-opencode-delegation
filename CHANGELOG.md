# Registro de cambios

Cambios con valor para quien usa el paquete, por versión. El formato sigue las
recomendaciones de *Keep a Changelog* y el versionado es semántico. Lo que se anota aquí
al publicar; la historia de cada cambio, commit a commit, vive en Git.

## [Sin publicar]

### Añadido

- Plugin de servidor `plugins/orchestrator-phase.ts`: inyecta en el system prompt de cada
  petición de chat la fase actual del runner —su número, su nombre y la evidencia que pide
  `next` para avanzar, más la última registrada— para que el modelo no la olvide al alargarse
  la sesión. Consulta `orchestrator-runner.py --json` en el directorio del proyecto con un
  límite de tres segundos y falla en silencio: sin estado, sin runner, pasado el tiempo o con
  una respuesta fuera de contrato no inyecta nada y la petición sigue su curso. La instalación
  gana un paso: los instaladores lo copian a `~/.config/opencode/plugins/`, con copia
  `.bak.<fecha>` si reemplaza una versión distinta, y no lo registran en `cli.json`, porque lo
  carga el servidor y no la TUI. `check-package.sh` comprueba la copia en la instalación
  limpia y la fusión sobre una copia previa distinta.
- Control en vuelo sobre subagentes en el contrato del orquestador: nueva regla 10 de
  `agents/orchestrator.md` que incorpora el plugin `opencode-subagent-intervene` al ciclo
  de delegación — inspección (`subagents.list`/`subagents.inspect`) para comprobar
  «lanzado no es hecho» y recuperar el árbol de subagentes tras una compactación,
  redirección en caliente (`subagent_send`) para corregir un encargo en marcha sin
  re-lanzarlo, y parada quirúrgica (`subagent_stop`) con motivo y actualización del libro
  `.run/delegations.jsonl`. La regla fija que un subagente parado solo se continúa
  deliberadamente, que los prompts de permiso los responde la persona y que la parada no
  sustituye al re-encargo de la regla 6.
- El runner gana el subcomando `boot`: imprime en una sola invocación el arranque de la
  fase 1 en cuatro secciones de texto —memoria reciente de la sesión (`engram context`),
  tablero con lo que está en curso y lo que está listo para empezar, rama y estado de git, y
  la fase actual— para que un agente lo lea de un vistazo al empezar. Es solo lectura: no
  crea ni modifica `.run/orchestrator-runner.json`. Cada fuente falla por su cuenta y su
  sección responde «no disponible» con el motivo, de modo que un proyecto sin tablero, sin
  git o sin estado del runner sigue dando el resto del arranque con código 0.
- Cuatro adopciones en el contrato de delegación. Un libro de encargos en disco:
  `.run/delegations.jsonl` del proyecto, una línea JSON por encargo con `ts`, `phase`,
  `task`, `fingerprint`, `agent` y `status` (`launched`, `completed` o `blocked`), que se
  consulta antes de lanzar para no repetir la misma huella mientras sigue en `launched`.
  Un comprobador determinista del retorno, `agents/delegation-report-checker.py`, que se
  instala junto a los agentes y dice si el informe cumple su forma —`status` único sin
  contradicciones entre líneas, una línea de
  `validation` por comprobación esperada y ni una referencia indefinida—, con códigos
  0/1/2 y `--json` para un programa. Una taxonomía de cuatro ejes —selección de fuente,
  interpretación, estrategia de búsqueda y ejecución— para clasificar la causa antes de
  re-encargar. Y el criterio de aceptación literal de la tarea al encargo, con la salida
  observada en cada línea de `validation`. `check-package.sh` cubre el comprobador en
  siete pasos —incluidos el `status` contradictorio y la anáfora sin ruta— y valida
  mecánicamente el `.run/delegations.jsonl` del proyecto si existe: una línea JSON objeto
  con los seis campos, `phase` entero y `status` en conjunto.
- La fase 11 del runner incorpora la revisión dirigida de memorias: listar candidatas a
  poda y presentarlas a la persona antes de tocar nada; ningún borrado de memoria sin
  su autorización explícita.
- El contrato de encargo distingue el obstáculo corriente —error, timeout, herramienta que
  falla—, que se diagnostica y se supera, del bloqueo deliberado —fichero intocable, acceso
  retirado a propósito—, que se respeta y se comunica; y exige el informe de retorno
  entendible sin haber visto el trabajo, con el diff y el resumen contrastados los dos
  contra el criterio en la fase de verificación.

- La guía web da al runner el peso de pieza core: sección propia «04 · El runner» con su
  contrato —la fase la decide el registro, las once fases, `next` con la evidencia exigida,
  `reset`, el estado en `.run/` del proyecto y `--json` sin efectos— y un diagrama de las once
  fases agrupadas en las tres etapas del flujo (Figura 1), en el sistema visual de las
  figuras existentes y con tema claro y oscuro. Las secciones posteriores se renumeran y la
  del plugin TUI queda para lo suyo.
- Un botón «Ver el código del runner» en la sección 04 de la guía web abre un diálogo modal
  con el código fuente íntegro de `agents/orchestrator-runner.py` como bloque de código
  resaltado, con el botón de copiar de la casa, cierre con Esc, clic en el fondo o su botón.
  El código va embebido en la página y `check-package.sh` comprueba que la copia embebida
  sigue coincidiendo con el fichero del paquete.

### Cambiado

- Dimensionado de encargos contra el presupuesto de pasos: la regla 3 de
  `agents/orchestrator.md` obliga a estimar las llamadas que exige un encargo (lecturas,
  ediciones y validación) contra el `steps` del agente elegido —público en su
  frontmatter— antes de lanzarlo, y a dividir por fichero o paquete pequeño cuando no cabe
  con margen. Un parcial por presupuesto agotado queda clasificado como fallo de
  dimensionamiento del orquestador, no de ejecución del subagente. La regla 6 añade la
  pre-masticación: el encargo nace con el contexto adjunto —informe previo, rutas exactas y
  decisiones ya tomadas—, y si esa preparación exige explorar mucho, `explore` produce ese
  mapa por adelantado; cada lectura que el subagente evita es un paso que va a la escritura.
  La guía web (`docs/web/index.html`) incorpora ambos puntos en sus resúmenes del contrato.
- Contratos alineados en tres puntos: las escrituras del tablero sin herramienta `tasks_*` —ADR (`adr create/edit/import/export`) y catálogo de etiquetas (`tag create/edit/delete`)— van directas por la CLI `tasks-ai` y no se delegan a `tasks-agent` (asignar etiquetas al crear o actualizar una tarea sí viaja en su encargo); un `partial` de `verify` causado por una denegación del entorno no se re-encarga idéntico —el orquestador conserva el informe, completa desde la raíz lo que faltó y presenta el resultado consolidado—; y la regla de skills del orquestador adjunta `ponytail-review` a los encargos de `review` y guía la planificación de pruebas con `software-testing`. La allowlist de `verify` abre `gofmt -l *` —listado de formato, nada que escriba— y `verify` nombra el comando exacto bloqueado cuando el límite es del entorno.
- Umbral de creación de tarea en el contrato del orquestador: no crear tarea queda
  reservado a cambios muy triviales —uno o dos archivos como máximo, p. ej. una entrada en
  `.gitignore`—; todo lo demás lleva tarea o épica según su importancia, aunque pase el test
  de la sesión interrumpida. La regla 2 de `agents/orchestrator.md` y la fase «Tamaño y
  tablero» del runner quedan alineadas.
- Advertencias nuevas en las fases 10 y 11 del runner contra el doble cierre y la reapertura
  sin autorización: una tarea ya cerrada con `done` no se vuelve a cerrar, reabrirla exige
  una justificación importante y la autorización explícita de la persona, y «podemos cerrar»
  apunta a la sesión, no a la tarea, así que no se repite el cierre ni se reabre sin permiso.
- El panel `/orquestador` avisa del desajuste entre la fase y el árbol de trabajo: de la fase 2
  a la 8, con ficheros sin commit, nombra cuántos hay y avisa de que el flujo aún no los ha
  llevado a «Riesgo y Commit»; se evalúa antes que el aviso de fase 1 con tareas en curso y
  ninguna evidencia, que sigue igual.

### Corregido

- Instrucciones de reinicio del runner: la fase 1 ordenaba borrar a mano `.run/orchestrator-runner.json`,
  lo que la regla `.llmignore` impide al agente; ahora ordena ejecutar su subcomando `reset` y deja claro
  que el fichero no se toca. La fase 11 pedía el `reset` al terminar la sesión sin convertirlo en obligación:
  ahora exige dejar el estado en fase 1 y sin historial antes de dar la sesión por cerrada.

- El modelo de `tasks-agent` en la documentación: la guía, el README y la verificación de
  tasks-agent declaraban `nan/mimo-v2.6-flash` (low/medium según la página) mientras el
  frontmatter canónico y la configuración desplegada fijan `nan/deepseek-v4-flash#high`. La
  documentación se sincroniza con el frontmatter, que manda; el registro histórico de las
  mediciones se conserva con una nota de la decisión vigente.

- Contrato de retorno y disciplina de delegación: el orquestador exige a cada subagente un
  resultado estructurado —`status` (`completed`, `partial` o `blocked`, o la pregunta al humano
  si el bloqueo es de decisión), `validation` con una línea `<comando>: <resultado observado>`
  por comprobación, riesgos y lo no verificado— y un retorno en prosa sin esa forma se vuelve a
  pedir; `tasks-agent` se rige por su contrato propio de una línea por operación. Antes de
  lanzar comprueba los encargos ya lanzados para no relanzar el mismo mientras
  sigue en curso, y los mapas de `explore` y la evidencia de `verify` vienen acotados a unos
  2k tokens con `path:line`; `verify`, además, no reproduce el texto del encargo. El README y la
  guía web narran el contrato: la regla del encargo y la de delegación de la sección de contrato
  de la guía, y el párrafo ODD del README, recogen el retorno estructurado, el dedup y el handoff
  acotado.

- Estado de orquestación visible desde la sesión: el runner gana `--json`, un único objeto con
  las once fases y el estado que no crea ni modifica el `.run/orchestrator-runner.json`, y el
  plugin TUI `plugins/orchestrator-status` abre con `/orquestador` un diálogo modal con scroll,
  la fase actual, su operativa, sus advertencias y el historial de evidencias, con `r` para
  refrescar y `esc` para cerrar. El diálogo solo lee; sin estado muestra la fase 1 y lo dice. Los instaladores copian el plugin a
  `~/.config/opencode/cli-plugins/orchestrator-status/`, registrado en `cli.json`, sin `node_modules` y con copia `.bak.<fecha>`
  al reemplazar una versión previa, y `check-package.sh` verifica la instalación junto al
  contrato `--json`.

- Herramienta del flujo `agents/orchestrator-runner.py`: guía determinista del orquestador
  que muestra la fase actual con su lista operativa y sus advertencias, y solo avanza con
  `next` — que exige la evidencia de la fase completada: sin ella, pide la que toca y no
  avanza; con ella, la registra en el historial del estado. Sin dependencias y sin reloj.
  Los instaladores la depositan junto a los agentes
  (`~/.config/opencode/agents/`), ejecutable en Unix, con la misma idempotencia y copia
  `.bak.<fecha>` que los agentes; su estado vive en el `.run/` del proyecto en el que se
  trabaja, no junto al script. Para que el agente no toque el registro a mano, el paquete
  lleva un `.llmignore` con la regla, aplicable con el plugin `opencode-ignore-file`
  instalado globalmente (acción manual de máquina; los instaladores no lo tocan).
  El PRD ajusta su alcance: sigue sin código que compilar, con
  los dos instaladores, la puerta de verificación y esta herramienta.

- Subagente `tasks-agent`: escritor exclusivo del tablero tasks-ai. Recibe la intención de
  otro agente y la ejecuta con las herramientas `tasks_*`; no planifica ni toca nada fuera
  del tablero. Sus permisos deniegan todo y reabren solo `tasks_*`, corre en una sesión
  hija con contexto limpio y el orquestador le delega las escrituras del tablero, mientras
  las lecturas van directas por la CLI `tasks-ai`.

- Regla global de `permissions` que deniega `tasks_*` a todos los agentes; solo
  `tasks-agent` las reabre en su frontmatter. Los instaladores la añaden una sola vez y, en
  el camino JSONC, imprimen el bloque a mano junto al de `agents`.

- Flujo de trabajo ODD (Organic Driven Development) en el orquestador: el camino cotidiano
  es orgánico —explorar antes de modificar, decisiones humanas solo donde pertenecen,
  verificación proporcional al cambio y evidencia recuperable— y formalizar más (plan,
  especificación) es siempre elección explícita de la persona, nunca una transición
  automática por tamaño o riesgo. Incluye el lenguaje hecho/inferencia/decisión, la
  escalera de verificación (trivial, funcional, transversal, crítico) y la convención de
  que una tarea sustancial nace con objetivo, alcance y criterios de aceptación.

- Protocolo ODD desarrollado y línea de riesgo: la guía explica cómo se decide, se registra
  y se cierra el trabajo —tamaño, tablero, commits, riesgo y reparto— y el contrato del
  orquestador incorpora el test de la sesión interrumpida, el registro del trabajo grande
  en el tablero a través de `tasks-agent` antes de escribir y la línea `Risk: item N
  (motivo)` o `Risk: none`, con verificación independiente para el riesgo alto.
  `CONTRIBUTING` alinea la línea de riesgo en los commits.

- Puerta de verificación `check-package.sh`: ejecuta de una vez la batería que CONTRIBUTING
  describe en prosa —sintaxis del instalador, plantilla JSON estricta, instalación limpia,
  fusión con claves ajenas y copia `.bak`, camino JSONC y `diff` contra la máquina—, sin
  interacción y escribiendo solo en temporales. El PRD ajusta su alcance: el paquete sigue
  sin código que compilar, ahora con los dos instaladores y este script.

- Paquete instalable con la delegación de subagentes `nan` para OpenCode V2: los agentes
  de configuración global (`orchestrator` primario; `verify`, `review`, `docs` y
  `ui` subagentes con permisos acotados), los overrides de modelo de los agentes
  integrados `explore` y `general`, y copias de seguridad automáticas al tocar una
  configuración que ya exista.

- Instaladores para Windows (`install.ps1`) y Linux/macOS/WSL (`install.sh`): copian los
  agentes, funden el bloque `agents` en la configuración global conservando las claves
  ajenas y, si el fichero lleva comentarios JSONC, dejan el fichero intacto e imprimen el
  bloque para pegarlo a mano.

- Guía completa de la orquestación en `docs/web`, sobre el sistema compartido de
  documentación: topbar Vue, índice lateral con scroll-spy, bloques de código con
  resaltado y botón de copiar, tema claro y oscuro con conmutador persistente y reveal
  en cascada. Todos los assets son locales: se abre sin red. Para verla completa va con
  `serve-web.sh`, que sirve el paquete en el 8766: por `file://` las máscaras de los
  iconos y el resaltado de código no cargan.

- Diagramas SVG en la guía: la sección 13 (Diagrama de arquitectura) dibuja el orquestador
  con los seis agentes que toca y la sección 14 (Diagrama de flujo) recorre el contrato de
  delegación, de la clasificación al cierre. Ambas heredan tema claro y oscuro.

- Página de evaluación en `docs/web/evals.html` con la evidencia de las pruebas de los
  agentes: el smoke test de los cinco agentes con su diagrama y veredictos, la medición
  del modelo trabajador con su gráfico de barras, y las decisiones vivas que sostienen el
  reparto de modelos. El material crudo —transcripciones y CSV— vive en `docs/evals/`.

- Tablero de desarrollo propio: el paquete lleva su instalación de tasks-ai en
  `.tasks-ai/` (instrucciones, contrato de etiquetas y ajustes versionados; base y
  extensiones fuera de Git) y el proyecto carga su extensión de OpenCode desde el
  `opencode.jsonc` raíz.

- `serve.sh` para dejar el servidor del tablero en segundo plano (start, status y stop,
  con registro y pid en `.run/`) y visor de escritorio en `tasks-viewer/` con el binario
  oficial de tasks-ai para Windows. El tablero del paquete escucha en el 8891 para convivir
  con el de tasks-ai, que mantiene el 8890.

### Modificado

- Reparto de skills: el orquestador conserva el catálogo y adjunta al encargo la ruta
  absoluta del `SKILL.md` que cada trabajo necesita, con una línea de por qué aplica. Los
  cuatro subagentes de verificación y documentación —verify, review, docs y ui— y los
  agentes integrados `explore` y `general` llevan el permiso `skill`
  denegado (`action: skill`, `resource: "*"`, `effect: deny`), que oculta la skill al modelo
  y rechaza la carga; `tasks-agent` conserva el catálogo. `tools` es
  un campo de la V1 que V2 ignora, así que no se usa. `check-package.sh` gana tres guardas:
  los cuatro subagentes con el permiso denegado, ningún agente ni la plantilla de
  configuración que lo abra en `allow` o `ask`, y las skills citadas presentes en una raíz
  de descubrimiento —con aviso, no fallo, si la máquina no tiene ninguna—.

- El tablero tiene dos caminos: las lecturas —consultar, mostrar una tarea, listar
  comentarios, etiquetas o ADR— van directas por la CLI `tasks-ai`, sin delegar, y solo las
  escrituras se delegan a `tasks-agent`. Los lotes de creación —épica, subtareas, etiquetas,
  comentario— van en un único encargo con la secuencia ordenada, en segundo plano, y se
  comprueban por CLI antes de usar su resultado: lanzado no es hecho. El cierre y el
  comentario de bitácora del momento van en primer plano. `tasks-agent` pasa de operador a
  escritor del tablero: ejecuta secuencias ordenadas en un solo encargo y devuelve una línea
  de resultado por operación; sus `steps` suben de 8 a 16. La regla global de `permissions`
  no cambia.

- El contrato del tablero cierra la puerta a las lecturas delegadas: a `tasks-agent` no se le
  encarga ninguna lectura ni investigación, tampoco a través de la intención del encargo; no
  puede leer ficheros, ni contratos como `CONTRATO_ETIQUETAS.md`, ni consultar catálogos o el
  binario, y cada operación viaja con sus campos, etiquetas y referencias ya resueltos por el
  orquestador, que obtiene por CLI lo que le falte antes de delegar. `tasks-agent` añade la
  guarda simétrica: si el encargo trae algo que no se ejecuta con las herramientas `tasks_*`,
  devuelve un fallo de una línea con la causa y para, sin reconstruir contexto leyendo o
  mostrando tareas. README y guía cuentan el mismo contrato.

- Configuración migrada a la clave `agents` de OpenCode V2: la plantilla, los instaladores y
  la documentación usan `agents`; el instalador mueve los overrides `explore` y `general`
  desde la clave obsoleta `agent` y conserva el resto de claves.

- `orchestrator` delega por defecto la implementación a `general`, sea cual sea su tamaño;
  solo implementa directamente lo trivial, inequívoco y de bajo riesgo, y prepara los
  encargos con `explore` y la documentación con `docs`.

### Corregido

- `tasks-agent` recupera el catálogo de skills: la regla que le denegaba el permiso `skill`
  se le había aplicado por error en el reparto general a los subagentes. `check-package.sh`
  ahora exige el permiso denegado solo a los cuatro subagentes de verificación y
  documentación (verify, review, docs y ui).

- La documentación citaba `playwright-cli`, que no es el nombre de la skill que usa el
  agente `ui`: README, PRD y guía nombran ahora `playwright-cli-usage`, la ruta que el
  orquestador adjunta al encargo.

- `orchestrator-runner.py` no ofrecía forma de reiniciar el flujo: su estado está vetado
  al agente por el `.llmignore` del paquete y su ayuda mandaba «borra el fichero» a mano —
  imposible para quien lo ejecuta, y el runner quedaba clavado en la última fase. Ahora
  tiene el subcomando `reset`, que borra fase e historial y devuelve a la fase 1; el
  estado sigue moviéndose solo por el runner.

- `tasks-agent` tardaba demasiado en operaciones simples: recibe ahora un prompt
  compactado a las guardas esenciales y la variante `#low`; sus permisos no
  cambian (`tasks_*`, sólo). Una operación trivial cocinada cae de minutos a segundos;
  el resto del tiempo bajo no era suyo, era del encargo.

- La guía deja de presentar el instalador de Windows como pendiente de prueba: se ha
  ejecutado y verificado con el PowerShell de Windows, también desde WSL, y la guía y
  CONTRIBUTING lo declaran a la par que el de Linux.

- `install.ps1` no retiraba la clave obsoleta `agent` cuando quedaba vacía tras la
  migración: en Windows PowerShell 5.1, `PSObject.Properties.Count` resuelve a `$null`
  sobre un objeto vacío y el borrado saltaba al aviso. Ahora se cuenta envolviendo la
  colección en `@()`.

- Antes de reemplazar un agente existente con el mismo nombre, el instalador guarda una
  copia `.bak` con fecha; si el contenido es idéntico, no crea copia.

- Guía y README corregidos en permisos, clave `agents` y salidas de verificación; el pie
  de la guía deja de llevar fecha y estado de publicación.

- La prosa de README, guía y evals ya no menciona el modelo `nan/glm5.3` sin flash, fuera
  del plan del proveedor; las transcripciones históricas de `docs/evals/` se conservan tal
  como se ejecutaron.

- `verify` deja de quedarse bloqueado en modo estático: su allowlist de shell cubre ahora
  `python3`, `python` y la batería `bash check-package.sh` del paquete, y su prompt ordena
  lanzar los comandos de uno en uno, sin `cd` ni encadenados, porque el scanner comprueba
  cada subcomando por separado y un `cd` encadenado no tiene permiso. La guía refleja los
  quince prefijos.
