# PRD del paquete de delegación nan para OpenCode

## 1. Propósito

El paquete resuelve un problema de repetición: la delegación de subagentes con modelos del
proveedor `nan` ya funciona en una máquina concreta y viva en su `~/.config/opencode`
global, donde no se versiona ni se traslada. Reproducirla en otra máquina exige recordar
cinco frontmatters, un bloque de configuración y dos lecciones de permisos que solo viven
en la cabeza de quien lo montó.

El producto es una carpeta autocontenida, versionada en Git, que contiene los artefactos
canónicos, los instaladores para las dos familias de sistema y la documentación que
explica qué instala, cómo se comprueba y dónde se acaba. Quien copie la carpeta y lance el
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

- Transportar la delegación tal cual: los cinco ficheros de agente son copias literales
  con su frontmatter de modelo, pasos y permisos.
- Un instalador por familia de sistema: `install.sh` para Linux, macOS y WSL, e
  `install.ps1` para Windows, con el mismo comportamiento observable.
- Documentación viva completa: guía HTML, README, PRD, CONTRIBUTING, CHANGELOG, AGENTS y
  llms.txt, en castellano y sin prose que mienta sobre los artefactos.

### No objetivos

- No trae código ejecutable fuera de los dos instaladores: nada que compilar ni
  dependencias que gestionar.
- No orquesta nada por sí mismo: la delegación es un contrato por prompt que OpenCode
  carga con los agentes; sin OpenCode el paquete es papel.
- No gestiona el acceso al proveedor `nan`: la API key se configura con
  `opencode auth login` y el paquete solo la menciona en la comprobación.
- No incluye la skill `playwright-cli`: el agente `ui` la necesita y su instalación queda
  fuera por venir de otro catálogo.

## 4. Requisitos aprobados

### Funcionales

- R1. El instalador copia los cinco agentes a `~/.config/opencode/agents/`, honrando
  `XDG_CONFIG_HOME` si está definido.
- R2. Si la configuración global no existe, el instalador crea una desde su plantilla con
  el bloque `agent` de `explore` y `general`.
- R3. Si ya existe, la fusión toca solo `agent.explore` y `agent.general`, conserva las
  claves y agentes ajenos y deja copia con fecha y hora antes de escribir.
- R4. Si el fichero de configuración no es JSON estricto (comentarios JSONC) o no hay
  `python3` en Linux, el instalador aborta la escritura con el fichero intacto, la copia
  previa hecha y el bloque exacto para pegar a mano.
- R5. Reejecutar el instalador no duplica agentes ni corrompe la configuración.
- R6. Los dos instaladores son idempotentes y no piden interacción.

### Reglas de negocio

- Los artefactos de `agents/` son copias byte a byte de la configuración que funciona; el
  paquete no es el lugar de donde nacen los cambios, sino por donde viajan.
- La variante de modelo usa `#variant` solo en el frontmatter Markdown; en la clave
  `agent` de la configuración va en campo `variant` propio.
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

- Con `HOME` limpio, `bash install.sh` crea los cinco agentes y una configuración nueva
  desde la plantilla; salida por pantalla nombra ambos destinos.
- Con una configuración JSON preexistente con claves ajenas (`share`, `mcp`), la fusión
  conserva esas claves y crea la copia `.bak` con fecha.
- Con una configuración JSONC con comentarios, el instalador termina con código 0, imprime
  el bloque manual y el fichero queda byte a byte igual a su copia previa.
- `diff -r` entre `agents/` del paquete y la configuración global de la máquina de origen
  no reporta diferencias.
- La plantilla `config/opencode.jsonc` carga como JSON estricto.
- La guía parsea como HTML con etiquetas balanceadas y sus enlaces internos resuelven a
  secciones existentes.
- Los datos de la tabla de modelos coinciden con los frontmatter declarados en `agents/`.
