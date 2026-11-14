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
  con su frontmatter de modelo, pasos y permisos, incluido `tasks-agent`, el escritor
  del tablero que concentra las herramientas `tasks_*`.
- Un instalador por familia de sistema: `install.sh` para Linux, macOS y WSL, e
  `install.ps1` para Windows, con el mismo comportamiento observable.
- Documentación viva completa: guía HTML, README, PRD, CONTRIBUTING, CHANGELOG, AGENTS y
  llms.txt, en castellano y sin prose que mienta sobre los artefactos.

### No objetivos

- No trae código ejecutable fuera de los dos instaladores, la puerta de verificación
  `check-package.sh`, la herramienta del flujo `agents/orchestrator-runner.py` y el plugin
  TUI `plugins/orchestrator-status`: fuentes que OpenCode carga en runtime, sin `node_modules`
  ni nada que compilar.
- No orquesta nada por sí mismo: la delegación es un contrato por prompt que OpenCode
  carga con los agentes; sin OpenCode el paquete es papel.
- No gestiona el acceso al proveedor `nan`: la API key se configura con
  `opencode auth login` y el paquete solo la menciona en la comprobación.
- No incluye la skill `playwright-cli-usage`: el agente `ui` la necesita, el orquestador la
  adjunta por ruta al delegar y su instalación queda fuera por venir de otro catálogo.

## 4. Requisitos aprobados

### Funcionales

- R1. El instalador copia los agentes de `agents/` y la herramienta del flujo
  `agents/orchestrator-runner.py` a `~/.config/opencode/agents/`, honrando
  `XDG_CONFIG_HOME` si está definido. En Unix la herramienta queda con permiso de ejecución;
  si ya existe una con el mismo nombre y contenido distinto, deja copia `.bak.<fecha>` antes
  de reemplazarla, con la política de R7.
- R2. Si la configuración global no existe, el instalador crea una desde su plantilla con
  el bloque `agents` de `explore` y `general` y la regla de `permissions` que deniega
  `tasks_*`.
- R3. Si ya existe, la fusión toca solo `agents.explore`, `agents.general` y la regla
  global de `permissions`, conserva las claves y los agentes ajenos de la configuración y
  deja copia con fecha y hora antes de escribir.
- R4. Si el fichero de configuración no es JSON estricto (comentarios JSONC), no hay
  `python3` en Linux o la clave `agents` no es un objeto, el instalador aborta la escritura
  con el fichero intacto, la copia previa hecha y los bloques exactos para pegar a mano.
- R5. Reejecutar el instalador no duplica agentes ni corrompe la configuración.
- R6. Los dos instaladores son idempotentes y no piden interacción.
- R7. Antes de reemplazar un agente existente con el mismo nombre, el instalador guarda una
  copia `.bak.<fecha>`; si el contenido es idéntico, no crea copia.
- R8. Los instaladores dejan en la configuración global una regla que deniega `tasks_*` a
  todos los agentes; solo `tasks-agent` las reabre en su frontmatter. La regla se añade una
  vez y reejecutar no la duplica; si `permissions` existe y no es una lista, avisan y no la
  tocan.
- R9. `check-package.sh` ejecuta de una vez la batería de verificación: sintaxis de
  `install.sh`, plantilla JSON estricta, instalación limpia en un `HOME` temporal, fusión
  con claves ajenas y copia `.bak`, camino JSONC con el fichero intacto y el bloque manual
  impreso, `diff` contra los agentes y la herramienta instalados, y las guardas de skills
  —los cuatro subagentes (verify, review, docs y ui) con el permiso `skill` denegado,
  `tasks-agent` conservando el catálogo, ningún agente que lo abra en
  `allow` o `ask` y las skills citadas presentes en una raíz de descubrimiento—. No pide interacción,
  solo escribe en directorios temporales y devuelve 0 si todo está en verde o un código distinto de 0 con
  el nombre de la comprobación fallida. Si la máquina no tiene agentes instalados, omite el
  `diff` con un aviso y no lo cuenta como fallo; si no tiene ninguna raíz de descubrimiento
  de skills, avisa y tampoco lo cuenta como fallo.
- R10. El estado de orquestación se puede ver desde la sesión. El runner gana
  `--json`: un único objeto con `version` (1), las once fases —número, nombre, cuándo,
  evidencia esperada, pasos y advertencias— y el `state` con si existe el fichero de estado,
  la fase actual, si es válida, si lo es el historial y la evidencia registrada; se acepta solo
  y sin otros argumentos, no
  crea ni modifica el estado y no escribe avisos. El plugin `plugins/orchestrator-status`, que
  los instaladores copian con sus fuentes a `~/.config/opencode/cli-plugins/orchestrator-status/`
  y registran en la clave `plugins` de `cli.json` —un paquete solo-TUI no puede vivir en
  `plugins/`, el servidor lo cargaría y colisiona con su OpenTUI interno—
  —sin `node_modules`, con copia `.bak.<fecha>` si reemplaza una versión distinta—, se abre
  con `/orquestador` y muestra en un diálogo modal con scroll la fase actual, su operativa, sus
  advertencias y el historial de evidencias. El panel solo lee: consulta
  `python3 ~/.config/opencode/agents/orchestrator-runner.py --json` desde el directorio del
  proyecto y con `r` vuelve a consultar; sin estado muestra la fase 1 y lo dice.
  `check-package.sh` comprueba el contrato `--json` y la instalación del plugin.

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
- Los instaladores funden solo `agents.explore`, `agents.general` y la regla global de
  `permissions`; el resto de claves y agentes ajenos se conservan.
- El orquestador conserva el catálogo de skills y lo reparte: los subagentes salvo
  `tasks-agent` —que conserva el suyo— y los agentes integrados `explore` y `general`
  llevan el permiso `skill` denegado (`action: skill`,
  `resource: "*"`, `effect: deny`), que oculta la skill al modelo y rechaza la carga, y no
  pueden cargarlas por su cuenta; cuando una aplica, el encargo lleva su ruta absoluta y
  el subagente la lee antes de empezar. Ninguna skill vive en el paquete. `tools` es un
  campo de la V1 que OpenCode V2 acepta en el esquema pero ignora: el mecanismo vigente es
  el permiso.
- El tablero tiene una sola puerta: la configuración global deniega `tasks_*` a todos los
  agentes y `tasks-agent` las reabre en su frontmatter; los demás le delegan las escrituras
  del tablero, mientras las lecturas van directas por la CLI `tasks-ai`. A `tasks-agent` no se le delega ninguna
   lectura ni investigación —ni a través de la intención del encargo—: el encargo lleva
   cada operación con sus campos, etiquetas y referencias ya resueltas por el orquestador,
   y ante una instrucción que no pueda ejecutar con `tasks_*`, el escritor devuelve un
   fallo de una línea y para. Los lotes de
  creación —épica, subtareas, etiquetas, comentario— van en un único encargo ordenado, en
  segundo plano, y se comprueban por CLI antes de usar su resultado; el cierre y el
  comentario de bitácora del momento van en primer plano.
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

- Con `HOME` limpio, `bash install.sh` crea los agentes, la herramienta del flujo y una
  configuración nueva desde la plantilla; salida por pantalla nombra ambos destinos.
- La herramienta queda en `~/.config/opencode/agents/orchestrator-runner.py` con permiso de
  ejecución y su contenido coincide con el del paquete. Reejecutar el instalador con el
  contenido idéntico no crea `.bak`; con contenido distinto, deja
  `orchestrator-runner.py.bak.<fecha>` antes de reemplazarlo.
- Invocada desde el directorio de un proyecto arbitrario, la herramienta crea su estado en
  el `.run/orchestrator-runner.json` de ese proyecto y guía el flujo; nunca escribe estado
  junto al script instalado.
- `next` sin evidencia no avanza: pide la evidencia esperada para la fase actual y se
  queda donde está; con evidencia (`next "texto"`), la registra en el historial del
  estado y avanza. Un registro viejo con solo la fase sigue aceptándose.
- Con una configuración JSON preexistente con claves ajenas (`share`, `mcp`), la fusión
  conserva esas claves y crea la copia `.bak` con fecha.
- Con una configuración JSONC con comentarios, el instalador termina con código 0, imprime
  el bloque manual y el fichero queda byte a byte igual a su copia previa.
- La fusión añade la regla de `permissions` una sola vez: reejecutar el instalador no la
  duplica ni repite avisos; en el camino JSONC el bloque impreso incluye `agents` y
  `permissions`.
- Con una configuración que use la clave obsoleta `agent` solo para `explore` y `general`,
  el instalador los mueve a `agents` y retira `agent`; si `agent` tiene otras claves, las
  conserva y avisa.
- Con un `orchestrator.md` propio de contenido distinto en la configuración global, el
  instalador lo reemplaza y deja `orchestrator.md.bak.<fecha>`; una segunda ejecución sin
  cambios no crea otra copia.
- `diff -r` entre `agents/` del paquete y la configuración global de la máquina de origen
  no reporta diferencias —incluida `agents/orchestrator-runner.py`—, excluidas las copias
  `.bak` que el propio instalador deja (R7).
- La plantilla `config/opencode.jsonc` carga como JSON estricto.
- Ningún subagente abre el permiso de acción `skill` en `allow` ni `ask`, y los cuatro de
  verificación y documentación (verify, review, docs y ui) lo
  deniegan (`action: skill`, `resource: "*"`, `effect: deny`): no reciben catálogo ni
  pueden cargarlas por su cuenta; `tasks-agent` conserva el suyo.
- Con `HOME` limpio, `agents.explore.permissions` y `agents.general.permissions` contienen
  `{"action": "skill", "resource": "*", "effect": "deny"}`; la fusión sobre claves ajenas
  deja la misma regla.
- Toda skill citada por el paquete existe en una raíz de descubrimiento
  (`~/.config/opencode/skills`, `~/.agents/skills` o `~/.claude/skills`); si la máquina no
  tiene ninguna, `check-package.sh` avisa y no falla.
- La guía parsea como HTML con etiquetas balanceadas y sus enlaces internos resuelven a
  secciones existentes.
- Los datos de la tabla de modelos coinciden con los frontmatter declarados en `agents/`.
- Con `HOME` limpio, `bash check-package.sh` termina con código 0 y nombra en verde cada
  comprobación.
- Con un `install.sh` roto o un `config/opencode.jsonc` inválido, `bash check-package.sh`
  termina con código distinto de 0 y nombra la comprobación fallida.
- Ejecutar `bash check-package.sh` no cambia la configuración real del usuario: solo la lee
  para el `diff` contra los agentes instalados.
- `orchestrator-runner.py --json` imprime un único objeto con `version` 1 y las once fases en
  orden, cada una con nombre, cuándo, evidencia esperada, pasos y advertencias; no crea
  `.run/` ni modifica el estado existente, y con otro argumento termina en 1.
- `--json` marca un estado corrupto con `valid` en false y la fase 1, y un historial que no
  es lista con `evidence_valid` en false sin perder la fase.
- Con `HOME` limpio, los dos instaladores dejan
  `~/.config/opencode/cli-plugins/orchestrator-status/` registrado en `cli.json`, con `tui.tsx`, `package.json`,
  `tsconfig.json` y `types/opencode-plugin.d.ts`, sin `node_modules`.
- Un plugin previo de contenido distinto se reemplaza tras dejar `tui.tsx.bak.<fecha>`; una
  segunda pasada con el contenido idéntico no crea otra copia.
- En una sesión de OpenCode con el paquete instalado, `/orquestador` abre el diálogo modal con
  scroll con la fase actual, su operativa, sus advertencias y el historial de evidencias; sin estado
  muestra la fase 1 y lo indica, un estado corrupto vuelve a la fase 1 con aviso, y sin
  `python3` o sin runner muestra un error de consulta.
