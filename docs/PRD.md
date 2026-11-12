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
  con su frontmatter de modelo, pasos y permisos, incluido `tasks-agent`, el operador
  del tablero que concentra las herramientas `tasks_*`.
- Un instalador por familia de sistema: `install.sh` para Linux, macOS y WSL, e
  `install.ps1` para Windows, con el mismo comportamiento observable.
- Documentación viva completa: guía HTML, README, PRD, CONTRIBUTING, CHANGELOG, AGENTS y
  llms.txt, en castellano y sin prose que mienta sobre los artefactos.

### No objetivos

- No trae código ejecutable fuera de los dos instaladores y la puerta de verificación
  `check-package.sh`: nada que compilar ni dependencias que gestionar.
- No orquesta nada por sí mismo: la delegación es un contrato por prompt que OpenCode
  carga con los agentes; sin OpenCode el paquete es papel.
- No gestiona el acceso al proveedor `nan`: la API key se configura con
  `opencode auth login` y el paquete solo la menciona en la comprobación.
- No incluye la skill `playwright-cli`: el agente `ui` la necesita y su instalación queda
  fuera por venir de otro catálogo.

## 4. Requisitos aprobados

### Funcionales

- R1. El instalador copia los agentes de `agents/` a `~/.config/opencode/agents/`, honrando
  `XDG_CONFIG_HOME` si está definido.
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
  impreso, y `diff` contra los agentes instalados. No pide interacción, solo escribe en
  directorios temporales y devuelve 0 si todo está en verde o un código distinto de 0 con
  el nombre de la comprobación fallida. Si la máquina no tiene agentes instalados, omite el
  `diff` con un aviso y no lo cuenta como fallo.

### Reglas de negocio

- Los artefactos de `agents/` son copias byte a byte de la configuración que funciona; el
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
- El tablero tiene una sola puerta: la configuración global deniega `tasks_*` a todos los
  agentes y `tasks-agent` las reabre en su frontmatter; los demás delegan en él.
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

- Con `HOME` limpio, `bash install.sh` crea los agentes y una configuración nueva
  desde la plantilla; salida por pantalla nombra ambos destinos.
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
  no reporta diferencias, excluidas las copias `.bak` que el propio instalador deja (R7).
- La plantilla `config/opencode.jsonc` carga como JSON estricto.
- La guía parsea como HTML con etiquetas balanceadas y sus enlaces internos resuelven a
  secciones existentes.
- Los datos de la tabla de modelos coinciden con los frontmatter declarados en `agents/`.
- Con `HOME` limpio, `bash check-package.sh` termina con código 0 y nombra en verde cada
  comprobación.
- Con un `install.sh` roto o un `config/opencode.jsonc` inválido, `bash check-package.sh`
  termina con código distinto de 0 y nombra la comprobación fallida.
- Ejecutar `bash check-package.sh` no cambia la configuración real del usuario: solo la lee
  para el `diff` contra los agentes instalados.
