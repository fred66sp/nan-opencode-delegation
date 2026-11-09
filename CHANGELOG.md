# Registro de cambios

Cambios con valor para quien usa el paquete, por versión. El formato sigue las
recomendaciones de *Keep a Changelog* y el versionado es semántico. Lo que se anota aquí
al publicar; la historia de cada cambio, commit a commit, vive en Git.

## [Sin publicar]

### Añadido

- Flujo de trabajo ODD (Organic Driven Development) en el orquestador: el camino cotidiano
  es orgánico —explorar antes de modificar, decisiones humanas solo donde pertenecen,
  verificación proporcional al cambio y evidencia recuperable— y formalizar más (plan,
  especificación) es siempre elección explícita de la persona, nunca una transición
  automática por tamaño o riesgo. Incluye el lenguaje hecho/inferencia/decisión, la
  escalera de verificación (trivial, funcional, transversal, crítico) y la convención de
  que una tarea sustancial nace con objetivo, alcance y criterios de aceptación.

### Añadido

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
