# PRD del paquete de delegación nan para OpenCode

## 1. Propósito

El paquete resuelve un problema de repetición: la delegación de subagentes con modelos del
proveedor `nan` ya funciona en una máquina concreta y viva en su `~/.config/opencode`
global, donde no se versiona ni se traslada. Reproducirla en otra máquina exige recordar
los frontmatters de los agentes, un bloque de configuración y dos lecciones de permisos que
solo viven en la cabeza de quien lo montó.

El producto es una carpeta autocontenida, versionada en Git, que contiene los artefactos
canónicos, los instaladores para las dos familias de sistema, la puerta de verificación que
los comprueba y la documentación que explica qué instala, cómo se comprueba y dónde se
acaba. Quien copie la carpeta y lance el
instalador de su sistema debe quedarse con la misma delegación que la máquina de origen,
sin leerse la historia de cómo se llegó a ella.

## 2. Usuarios y casos de uso

- La persona que mantiene el paquete, al incorporar una máquina nueva: instalar en
  minutos, comprobar con comandos explícitos y seguir trabajando.
- Un agente de IA (OpenCode u otro), al preparar o revisar una máquina: la documentación
  declara fuentes canónicas y criterios observables, de modo que el agente comprueba en
  lugar de adivinar.

Fuera de alcance personas que no usen OpenCode: el paquete es inútil sin él y no intenta
ser una introducción a la orquestación en general.

## 3. Alcance aprobado

### Objetivos

- Transportar la delegación tal cual: los ficheros de agente son copias literales
  con su frontmatter de modelo, pasos y permisos. Con Tasks-AI v0.2.0 el tablero no lleva
  subagente: el orquestador lo gestiona directamente por AIF.
- Un instalador por familia de sistema: `install.sh` para Linux, macOS y WSL, e
  `install.ps1` para Windows, con el mismo comportamiento observable.
- Un comando nativo `commands/odd-fix.md`, instalado junto a los agentes, que abre el flujo
  del orquestador con la descripción de un defecto y no define herramientas ni pasos propios.
- Documentación viva completa: guía HTML, README, PRD, CONTRIBUTING, CHANGELOG, AGENTS y
  llms.txt, en castellano y sin prose que mienta sobre los artefactos.

### No objetivos

- No trae código ejecutable fuera de los dos instaladores, la puerta de verificación
  `check-package.sh`, el runner Go de `runner-go/` —distribuido como binario por
  plataforma—, el plugin de servidor `plugins/orchestrator-phase.ts` y el plugin TUI
  `plugins/orchestrator-status`: las fuentes de los plugins las carga OpenCode en runtime,
  sin `node_modules` ni nada que compilar.
- No inyecta la fase por agente ni por subagente: el plugin de servidor escribe en el system
  prompt del host y no distingue quién responde, de modo que la fase alcanza cada petición de
  chat y no se restringe el alcance a los subagentes.
- No orquesta nada por sí mismo: la delegación es un contrato por prompt que OpenCode
  carga con los agentes; sin OpenCode el paquete es papel.
- No gestiona el acceso al proveedor `nan`: la API key se configura con
  `opencode auth login` y el paquete solo la menciona en la comprobación.
- No incluye la skill `playwright-cli-usage`: el agente `ui` la necesita, el orquestador la
  adjunta por ruta al delegar y su instalación queda fuera por venir de otro catálogo.
- No hay `/odd-status`: se descartó porque `boot` puede escribir —`git status` y
  `acm start`— y su `--json` se solapa con el panel `/orquestador`.

## 4. Requisitos aprobados

### Funcionales

- R1. El instalador copia los agentes de `agents/` a `~/.config/opencode/agents/`,
  honrando `XDG_CONFIG_HOME` si está definido, e instala allí el binario
  `orchestrator-runner` (`.exe` en Windows) con permiso de ejecución; si ya existe un fichero con el mismo
  nombre y contenido distinto, deja copia `.bak.<fecha>` antes
  de reemplazarlo, con la política de R7. Después copia el plugin de servidor
  `plugins/orchestrator-phase.ts` a `~/.config/opencode/plugins/`, con la misma copia
  `.bak.<fecha>` si reemplaza una versión distinta y sin registrarlo en `cli.json`: lo carga
  el servidor, no la TUI.
- R2. Si la configuración global no existe, el instalador crea una desde su plantilla con
  el bloque `agents` de `explore` y `general`.
- R3. Si ya existe, la fusión toca solo `agents.explore` y `agents.general`, conserva las
  claves y los agentes ajenos de la configuración y deja copia con fecha y hora antes de
  escribir. Además retira, con la misma garantía de conservación de lo ajeno, el
  `tasks-agent.md` previo —con copia `.bak.<fecha>` siempre antes de borrarlo, porque el
  paquete ya no distribuye el fichero y una versión personalizada solo se conserva con la
  copia— y la regla global de `permissions` que denegaba `tasks_*`,
  solo cuando coincide exactamente con la entrada conocida; si difiere, la conserva e
  informa.
- R4. Si el fichero de configuración no es JSON estricto (comentarios JSONC), no hay
  `python3` en Linux o la clave `agents` no es un objeto, el instalador aborta la escritura
  con el fichero intacto, la copia previa hecha y los bloques exactos para pegar a mano.
- R5. Reejecutar el instalador no duplica agentes ni corrompe la configuración.
- R6. Los dos instaladores son idempotentes y no piden interacción.
- R7. Antes de reemplazar un agente existente con el mismo nombre, el instalador guarda una
  copia `.bak.<fecha>`; si el contenido es idéntico, no crea copia.
- R8. Los instaladores retiran de instalaciones anteriores el `tasks-agent.md` distribuido
  por este paquete —el escritor del tablero, retirado con Tasks-AI v0.2.0, que dejó a AIF
  como única superficie del tablero— y la regla global exacta `{"action": "tasks_*",
  "resource": "*", "effect": "deny"}`; una regla `tasks_*` que no coincida exactamente se
  conserva e informa. Reejecutar no duplica ni rompe nada; si `permissions` existe y no es
  una lista, avisan y no la tocan.
- R9. `check-package.sh` ejecuta de una vez la batería de verificación: sintaxis de
  `install.sh`, plantilla JSON estricta, instalación limpia en un `HOME` temporal, fusión
  con claves ajenas y copia `.bak`, camino JSONC con el fichero intacto y el bloque manual
  impreso, `diff` contra los agentes y el binario instalados, la compilación del módulo Go,
  la instalación limpia del
  comando nativo y su reemplazo con `.bak` en una segunda pasada, y las guardas de skills
  —los cuatro subagentes (verify, review, docs y ui) con el permiso `skill` denegado,
  ningún agente que lo abra en
  `allow` o `ask` y las skills citadas presentes en una raíz de descubrimiento—. No pide interacción,
  solo escribe en directorios temporales y devuelve 0 si todo está en verde o un código distinto de 0 con
  el nombre de la comprobación fallida. Si la máquina no tiene agentes instalados, omite el
  `diff` con un aviso y no lo cuenta como fallo; si no tiene ninguna raíz de descubrimiento
  de skills, avisa y tampoco lo cuenta como fallo.
- R10. El estado de orquestación se puede ver desde la sesión. El runner ofrece
  `--json`: un único objeto con `version` (1), las once fases —número, nombre, cuándo,
  evidencia esperada, pasos y advertencias— y el `state` de la sesión invocante —fase
  actual, validez del estado y del historial y evidencia registrada—; se acepta solo y sin
  otros argumentos, no crea ni modifica la base y no escribe avisos. El plugin
  `plugins/orchestrator-status`, que
  los instaladores copian con sus fuentes a `~/.config/opencode/cli-plugins/orchestrator-status/`
  y registran en la clave `plugins` de `cli.json` —un paquete solo-TUI no puede vivir en
  `plugins/`, el servidor lo cargaría y colisiona con su OpenTUI interno—
  —sin `node_modules`, con copia `.bak.<fecha>` si reemplaza una versión distinta—, se abre
  con `/orquestador` y muestra en un diálogo modal con scroll la fase actual, su operativa, sus
  advertencias y el historial de evidencias. El panel solo lee: invoca
  `orchestrator-runner --json` desde el directorio del proyecto y con `r` vuelve a
  consultar; sin estado muestra la fase 1 y lo dice.
  `check-package.sh` comprueba el contrato `--json` y la instalación del plugin.
- R11. El registro de delegaciones lo lleva el runner Go como único escritor, en la base
  SQLite `.run/orchestrator.db` del proyecto: huella calculada sobre el contenido del
  encargo, bloqueo de duplicados en estado abierto, ciclo `deck_prepare`, `deck_launch`,
  `deck_return`, `deck_close` y `deck_note`, y consulta `deck_list`. La migración importa
  el estado y el registro de delegaciones anteriores una sola vez, y `export-legacy`
  permite revertir al formato del runner retirado sin pérdida. Requisitos y garantías:
  `docs/runner/PRD.md` R2 y `docs/runner/DESIGN.md` §4 y §6; no se duplican aquí.
- R12. La validación del informe de retorno es lógica interna de la operación
  `deck_return`, no una invocación aparte: exige `status` único y válido, una línea por
  comprobación esperada y ninguna referencia indefinida, conserva la evidencia y devuelve
  los pendientes. La verdad y la cobertura siguen siendo juicio del orquestador. Reglas y
  criterio: `docs/runner/PRD.md` R3.
- R14. El runner permite recorrer solo las fases que aportan, con la operación
  `phase_skip` de su catálogo AIF: solo hacia una fase posterior, con al menos una fase
  de por medio y con el motivo obligatorio. Conserva el historial anterior y registra el
  salto con su fase de partida, su motivo y su destino, así que las fases omitidas quedan
  visibles en el panel y en `--json`. Un salto no sustituye ninguna autorización humana:
  si omite Commit, lo advierte y la confirmación de la persona sigue pendiente. Semántica
  completa: `docs/runner/PRD.md` R1; `check-package.sh` cubre el salto válido, los
  inválidos y la consulta de solo lectura. El contrato del orquestador decide el recorrido
  con evidencia, sin convertirlo en un procedimiento.
- R13. El plugin de servidor `plugins/orchestrator-phase.ts` inyecta en el system prompt de
  cada petición de chat la fase actual del runner —número, nombre y la evidencia que pide
  la transición para avanzar, más la última evidencia registrada cuando es válida—, para
  que el modelo no la olvide al alargarse la sesión, y solo cuando cambia la fase o hay
  avisos pendientes. Consulta `orchestrator-runner --json` (ruta instalada, `PATH` o
  `OPENCODE_RUNNER_BIN`) desde el directorio del proyecto con un límite de tres segundos y
  su tolerancia es total: sin estado, con el runner ausente o no ejecutable, pasado el
  tiempo o con una respuesta fuera del contrato `--json`, no inyecta nada y la petición
  sigue su curso. Solo lee, y una sola copia sirve a los dos hosts (V1 y V2).
  `check-package.sh` comprueba su instalación y que no figura en `cli.json`.
- R15. Los instaladores copian `commands/odd-fix.md` a `~/.config/opencode/commands/`,
  respetando `XDG_CONFIG_HOME`; si el destino difiere, guardan una copia `.bak.<fecha>`.
  El comando selecciona `orchestrator` en la sesión actual, recibe `$ARGUMENTS` y solicita
  una descripción si falta; no sustituye el contrato ni concede autorizaciones de Git.
  Reinstalar el mismo contenido no crea otra copia previa.
- R16. El paquete incluye el runner del proceso como producto propio: un binario Go
  (`runner-go/`, binario `orchestrator-runner`) con AIF como interfaz nativa y SQLite
  como persistencia, distribuido por plataforma desde las releases y sin runtime en
  destino. Sus requisitos —fases, delegaciones, informe, arranque, superficie JSON,
  portabilidad, recuperación y reversión— viven en `docs/runner/PRD.md` y su diseño en
  `docs/runner/DESIGN.md`; el paquete solo exige que instale, se identifique con
  `--version`, pase `go test ./...` y el arnés de equivalencia de
  `runner-go/equivalence/`, y no distribuya binarios compilados en el repositorio.

### Reglas de negocio

- Los ficheros `agents/*.md` son copias byte a byte de la configuración que funciona; el
  paquete no es el lugar de donde nacen los cambios, sino por donde viajan.
- La variante de modelo usa `#variant` en el frontmatter Markdown y en la forma abreviada
  de `agents`; en `agents` también puede ir en el objeto `model` (`providerID`, `model`,
  `variant`), y no existe un campo `variant` suelto.
- El permiso de edición no cubre el shell: `review`, `docs` y `ui` pueden ejecutar comandos.
  En `review` la edición está denegada y en `docs` limitada a Markdown, `docs/` y
  `CHANGELOG`; lo que impide escribir archivos o código por shell vive en el prompt.
- La clave `agent` en singular está obsoleta en OpenCode V2: el instalador escribe en
  `agents` y migra ahí los overrides `explore` y `general`, conservando el resto de claves.
- Los instaladores funden solo `agents.explore` y `agents.general`; el resto de claves y
  agentes ajenos se conservan.
- El orquestador conserva el catálogo de skills y lo reparte: los subagentes y los agentes
  integrados `explore` y `general` llevan el permiso `skill` denegado (`action: skill`,
  `resource: "*"`, `effect: deny`), que oculta la skill al modelo y rechaza la carga, y no
  pueden cargarlas por su cuenta; cuando una aplica, el encargo lleva su ruta absoluta y
  el subagente la lee antes de empezar. Ninguna skill vive en el paquete. `tools` es un
  campo de la V1 que OpenCode V2 acepta en el esquema pero ignora: el mecanismo vigente es
  el permiso.
- El tablero no lleva intermediario: con Tasks-AI v0.2.0 el orquestador ejecuta él mismo
  las operaciones deterministas del tablero por AIF (`tasks-ai aif`), el subcomando
  autodescriptivo; AIF es la fuente del contrato de sus operaciones y su regla vive en
  `.tasks-ai/AGENTS.md` del proyecto. Las escrituras se comprueban por CLI antes de usar
  su resultado; los lotes de creación —épica, subtareas, etiquetas, comentario— se
  registran ordenados y el cierre y el comentario de bitácora del momento van en primer
  plano. Las lecturas van directas por la CLI `tasks-ai`. Las operaciones que AIF no cubre
  —los ADR y el catálogo de etiquetas— van por la CLI convencional, y asignar etiquetas
  va dentro de `create`/`update`.
- Ante un `partial`, un retorno malformado o un fallo que pida re-encargo, la causa se
  clasifica con cuatro ejes antes de relanzar: selección de fuente (el subagente fue al
  sitio equivocado), interpretación (datos correctos con el significado equivocado: leyó
  mal el criterio), estrategia de búsqueda (se detuvo pronto o se desvió) y ejecución
  (error en el cálculo o en la forma del retorno). El eje ajeno al subagente queda para el
  bloqueo de decisión o del entorno. El re-encargo cita el eje diagnosticado y lo que lo
  corrige, y no repite el encargo sin cambiar nada. Cuando el eje es el entorno —una
  denegación que bloqueó una comprobación al subagente—, el orquestador conserva el informe
  `partial`, completa él la comprobación pendiente desde la raíz y presenta el resultado
  consolidado, sin re-encargo idéntico.
- El criterio de aceptación viaja literal de la tarea al encargo, y cada línea de
  `validation` cita la salida observada: la transcripción entre llamadas introduce errores,
  así que el criterio no se resume ni se reescribe al delegar.
- La prosa documental vive en castellano; los identificadores, rutas y comandos no se traducen.
- La guía solo usa el verde donde algo se ha comprobado; los datos de medición nombran su
  método y su alcance.

### No funcionales

- La guía vive en `docs/web`: un `index.html` más assets locales (tokens y chrome CSS,
  `docs.js`, desarrollo compartido con la guía de tasks-ai, vendor de iconos, Vue,
  MicroLighter y tipografías por sistema) que viaja con el paquete. Sin red: nada carga
  de un CDN. Tema claro y oscuro por preferencia del sistema, con conmutador persistente.
- La guía se revisa servida: `serve-web.sh` la deja en `http://127.0.0.1:8766` desde la
  raíz del paquete. Abierta por `file://` el navegador bloquea las máscaras de los
  iconos y el módulo de resaltado: la página se lee, pero pierde iconos y color de
  código. Es la razón de `serve-web.sh`.
- Los repos copian los assets compartidos de su origen, con el mismo convenio que
  `agents/`: el origen canónico es la guía de tasks-ai; las divergencias son la identidad
  de color, la marca y la marca de `docs.js`.
- Comandos y fragmentos de la guía se copian pegando; nada depende de una ruta de la
  máquina de origen.

## 5. Criterios de aceptación

- Con `HOME` limpio, la instalación copia `commands/odd-fix.md` sin diferencias; al
  reemplazar un comando distinto deja una copia previa y la siguiente instalación idéntica
  no crea otra.
- Con `HOME` limpio, `bash install.sh` crea los agentes, instala el binario
  `orchestrator-runner` y una configuración nueva desde la plantilla; la salida por
  pantalla nombra los destinos.
- El binario queda en `~/.config/opencode/agents/orchestrator-runner` con permiso de
  ejecución, se identifica con `--version` y responde al catálogo AIF. Reejecutar el
  instalador con el contenido idéntico no crea `.bak`; con contenido distinto, deja
  `orchestrator-runner.bak.<fecha>` antes de reemplazarlo.
- Invocado desde el directorio de un proyecto arbitrario, el runner escribe su estado en
  `.run/orchestrator.db` y guía el flujo; nunca escribe estado junto al binario instalado.
- `phase_next` sin `evidence` no avanza: devuelve `input_needed` y sigue en la fase
  actual; con evidencia, la registra en el historial y avanza. Una base anterior con solo
  la fase sigue aceptándose tras la migración.
- `phase_skip` avanza a una fase posterior, exige motivo y conserva la evidencia previa;
  un salto inválido —retroceso, fase actual, siguiente, fuera de rango, sin motivo o con
  motivo que empieza por `--`— devuelve `invalid` sin escribir. La entrada del historial
  nombra la fase de partida y el destino, y `--json` la expone sin cambiar `version` ni la
  forma de `state`; un estado anterior sin saltos se sigue leyendo.
- Con una configuración JSON preexistente con claves ajenas (`share`, `mcp`), la fusión
  conserva esas claves y crea la copia `.bak` con fecha.
- Con una configuración JSONC con comentarios, el instalador termina con código 0, imprime
  el bloque manual y el fichero queda byte a byte igual a su copia previa.
- La fusión no deja regla de `permissions` nueva: la plantilla no la lleva y en una
  configuración anterior la regla exacta de `tasks_*` se retira; reejecutar el instalador
  no añade ni repite avisos; en el camino JSONC el bloque impreso incluye solo `agents`.
- Con una configuración que use la clave obsoleta `agent` solo para `explore` y `general`,
  el instalador los mueve a `agents` y retira `agent`; si `agent` tiene otras claves, las
  conserva y avisa.
- Con un `orchestrator.md` propio de contenido distinto en la configuración global, el
  instalador lo reemplaza y deja `orchestrator.md.bak.<fecha>`; una segunda ejecución sin
  cambios no crea otra copia.
- `diff -r` entre `agents/` del paquete y la configuración global de la máquina de origen
  no reporta diferencias excluidas las copias `.bak` que el propio instalador deja (R7); el
  binario `orchestrator-runner` no forma parte de ese `diff`.
- La plantilla `config/opencode.jsonc` carga como JSON estricto.
- Ningún subagente abre el permiso de acción `skill` en `allow` ni `ask`, y los cuatro de
  verificación y documentación (verify, review, docs y ui) lo
  deniegan (`action: skill`, `resource: "*"`, `effect: deny`): no reciben catálogo ni
  pueden cargarlas por su cuenta.
- Con `HOME` limpio, `agents.explore.permissions` y `agents.general.permissions` contienen
  `{"action": "skill", "resource": "*", "effect": "deny"}`; la fusión sobre claves ajenas
  deja la misma regla.
- Toda skill citada por el paquete existe en una raíz de descubrimiento
  (`~/.config/opencode/skills`, `~/.agents/skills` o `~/.claude/skills`) o en el
  paquete de plugin que la aporta; la batería solo puede comprobar las raíces de
  descubrimiento. Si la máquina no tiene ninguna raíz, `check-package.sh` avisa y
  no falla.
- La guía parsea como HTML con etiquetas balanceadas y sus enlaces internos resuelven a
  secciones existentes.
- Los datos de la tabla de modelos coinciden con los frontmatter declarados en `agents/`.
- Con `HOME` limpio, `bash check-package.sh` termina con código 0 y nombra en verde cada
  comprobación.
- Con un `install.sh` roto o un `config/opencode.jsonc` inválido, `bash check-package.sh`
  termina con código distinto de 0 y nombra la comprobación fallida.
- Ejecutar `bash check-package.sh` no cambia la configuración real del usuario: solo la lee
  para el `diff` contra los agentes instalados.
- `orchestrator-runner --json` imprime un único objeto con `version` 1 y las once fases en
  orden, cada una con nombre, cuándo, evidencia esperada, pasos y advertencias; no crea la
  base ni modifica el estado existente, y con otro argumento termina en 1.
- `--json` marca un estado corrupto con `valid` en false y la fase 1, y un historial que no
  es lista con `evidence_valid` en false sin perder la fase; sin sesión identificable
  devuelve el estado vacío sin crear la base.
- Con `HOME` limpio, los dos instaladores dejan
  `~/.config/opencode/cli-plugins/orchestrator-status/` registrado en `cli.json`, con `tui.tsx`, `package.json`,
  `tsconfig.json` y `types/opencode-plugin.d.ts`, sin `node_modules`.
- Un plugin previo de contenido distinto se reemplaza tras dejar `tui.tsx.bak.<fecha>`; una
  segunda pasada con el contenido idéntico no crea otra copia.
- Con `HOME` limpio, los dos instaladores dejan
  `~/.config/opencode/plugins/orchestrator-phase.ts` y no lo registran en `cli.json`; un plugin
  previo de contenido distinto se reemplaza tras dejar `orchestrator-phase.ts.bak.<fecha>`, y
  una segunda pasada con el contenido idéntico no crea otra copia.
- Con un estado válido, cada petición de chat recibe en su system prompt la fase actual, su
  nombre y la evidencia que pide la transición, y la última evidencia registrada cuando la
  hay, y solo cuando la fase cambia o hay avisos pendientes. Sin
  estado, sin runner, pasado el límite de tres segundos o con una respuesta fuera del contrato
  `--json`, no se inyecta nada y la petición sigue su curso.
- En una sesión de OpenCode con el paquete instalado, `/orquestador` abre el diálogo modal con
  scroll con la fase actual, su operativa, sus advertencias y el historial de evidencias —con
  los saltos marcados por su fase de partida y su destino—; sin estado
  muestra la fase 1 y lo indica, un estado corrupto vuelve a la fase 1 con aviso, y sin
  runner o con el binario no disponible muestra un error de consulta.
- `deck_return` devuelve `executed` sobre un informe con `status` válido y una línea por
  comprobación esperada, y `invalid` con los incumplimientos nombrados cuando falta la línea
  `status:` o una comprobación esperada; conserva la evidencia válida y devuelve los
  pendientes en la misma respuesta. Los mismos informes que aceptaba o rechazaba el checker
  retirado producen el mismo veredicto estructural.
- Cada encargo lanzado queda como una fila del registro del runner en `.run/orchestrator.db`,
  con su huella e identificador de ejecución; un duplicado con la misma huella en estado
  abierto se deniega devolviendo el `id` existente, y los cerrados no bloquean un re-encargo.
  La migración del registro de delegaciones anterior conserva todas sus líneas.
