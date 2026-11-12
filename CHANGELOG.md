# Registro de cambios

Cambios con valor para quien usa el paquete, por versión. El formato sigue las
recomendaciones de *Keep a Changelog* y el versionado es semántico. Lo que se anota aquí
al publicar; la historia de cada cambio, commit a commit, vive en Git.

## [Sin publicar]

### Añadido

- Herramienta del flujo `agents/orchestrator-runner.py`: guía determinista del orquestador
  que muestra la fase actual con su lista operativa y sus advertencias, y solo avanza con
  `next`. Sin dependencias y sin reloj. Los instaladores la depositan junto a los agentes
  (`~/.config/opencode/agents/`), ejecutable en Unix, con la misma idempotencia y copia
  `.bak.<fecha>` que los agentes; su estado vive en el `.run/` del proyecto en el que se
  trabaja, no junto al script. El PRD ajusta su alcance: sigue sin código que compilar, con
  los dos instaladores, la puerta de verificación y esta herramienta.

- Subagente `tasks-agent`: operador exclusivo del tablero tasks-ai. Recibe la intención de
  otro agente y la ejecuta con las herramientas `tasks_*`; no planifica ni toca nada fuera
  del tablero. Sus permisos deniegan todo y reabren solo `tasks_*`, corre en una sesión
  hija con contexto limpio y el orquestador le delega siempre las operaciones del tablero.

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

- Configuración migrada a la clave `agents` de OpenCode V2: la plantilla, los instaladores y
  la documentación usan `agents`; el instalador mueve los overrides `explore` y `general`
  desde la clave obsoleta `agent` y conserva el resto de claves.

- `orchestrator` delega por defecto la implementación a `general`, sea cual sea su tamaño;
  solo implementa directamente lo trivial, inequívoco y de bajo riesgo, y prepara los
  encargos con `explore` y la documentación con `docs`.

### Corregido

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
