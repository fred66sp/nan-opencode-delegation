# PRD — Runner de orquestación (nan-opencode-delegation)

Estado: borrador para revisión del PO (WP-03). El porqué, el valor y los límites viven en `docs/runner/PRODUCT.md`; este documento fija solo requisitos. La técnica vive en `docs/runner/DESIGN.md`. Sin duplicación: cada requisito referencia su criterio observable y nada más.

## Objetivo

Sustituir el runner Python por un ejecutable Go con AIF como biblioteca nativa, que gestione el proceso de trabajo del orquestador con garantías deterministas, conservando las once fases, su proporcionalidad y la autonomía del agente (PRODUCT §Propósito, §Principios).

## Usuarios y consumidores

- El agente orquestador: consumidor primario de las operaciones.
- Los plugins de fase y TUI de OpenCode: consumidores de la superficie de consulta.
- El instalador y la batería de pruebas: consumidores de la superficie de instalación y verificación.
- El PO: consumidor de las métricas de reducción y de la trazabilidad.

## Requisitos funcionales

### R1 — Máquina de fases

Mantener las once fases con su semántica actual: transiciones con evidencia, saltos hacia delante con motivo, fase 11 terminal, reinicio explícito que no toca las delegaciones. Las transiciones y la gestión del proceso forman parte del catálogo AIF. La recuperación ante estado inválido, corrupto o base dañada es una operación explícita: conserva lo rescatable, archiva lo dañado, declara lo perdido y nunca reinicia en silencio.

Criterio de aceptación: las transiciones válidas e inválidas del runner actual producen los mismos estados en el nuevo (verificado por secuencia comparada); una base dañada activa la ruta de recuperación y las delegaciones previas siguen declaradas con lo perdido.

### R2 — Registro de delegaciones

El runner es el único escritor del registro. Ciclo completo como operaciones AIF: preparación con huella **calculada por el runner** a partir del contenido del encargo (agente, tarea, objetivo, alcance, comprobaciones, criterio), confirmación de lanzamiento que distingue la sesión del orquestador de la identidad de ejecución del subagente en OpenCode, seguimiento con conciliación declarada, retención del retorno con su estructura y pendientes devueltos en la misma invocación, y cierre formal con guardas que impiden usarlo para eludir una ejecución activa. Cada ejecución tiene identificador propio e historial conservado. Bloquean duplicados los estados abiertos, incluidos los retornos pendientes de integración; cerrados no. Sin caducidades automáticas ni detección semántica.

Criterio de aceptación: un duplicado con huella igual en estado abierto se rechaza devolviendo el `id` existente; dos ejecuciones legítimas del mismo encargo conviven con ids e historiales propios; un cierre desde lanzado sin retorno ni observación declarada se deniega; un encargo preparado sin confirmar aparece como advertencia de contraste, no como abandono.

### R3 — Contrato de informe

La validación estructural del informe de retorno es lógica interna de la operación que procesa el retorno: no es una invocación previa obligatoria. Aplica las reglas del checker actual (status único, sección de comprobaciones, una línea por comprobación esperada, anáfora sin referencia). Solo forma; la verdad y la cobertura siguen siendo juicio del agente. El checker Python se retira tras demostrar equivalencia funcional y comprobar sus consumidores (D2).

Criterio de aceptación: los mismos informes que hoy acepta y rechaza el checker producen los mismos veredictos estructurales vía el retorno, con la lista de incumplimientos y la evidencia válida conservada.

### R4 — Arranque integrado y consultas de solo lectura

Dos superficies distinguibles: el arranque integrado (`start`), que puede abrir o recuperar una sesión ACM y presenta memoria, tablero, Git, fase y pendientes con degradación por sección; y las consultas de estado (las que consumen los plugins y el panel), estrictamente de solo lectura: sin base de datos devuelven el estado vacío sin crearla, y con base no la mutan.

Criterio de aceptación: las consultas no crean base, no mutan fichero, memoria ni tablero (comprobable por diff del directorio `.run/` y por las pruebas de la batería); el arranque degrada cada sección a «no disponible» sin fallar.

### R5 — Superficie JSON para plugins

Los plugins y el panel consumen una única consulta de solo lectura con contrato estable. La compatibilidad exigida es funcional (misma información, misma forma de contrato): un cambio de emisor no debe obligar a reescribir los parsers, y toda extensión es aditiva. La inyección automática de fase evita redundancia: solo cuando cambia la fase o hay avisos pendientes, y con el mínimo que el agente no tenga ya en su contexto operativo.

Criterio de aceptación: los dos parsers actuales aceptan la salida del nuevo binario sin cambios en su lógica de campos; la inyección no repite lo que `start` ya proporcionó.

### R6 — Interfaz AIF principal

AIF es la interfaz principal del agente con el runner: `start` para el contexto integrado, `aif` para descubrimiento y operaciones con envelope por stdin y una respuesta por stdout, `--version` para identificar el ejecutable, `--json` para los plugins y ayuda convencional. El modo AIF es el subcomando explícito, no una detección por el contenido de stdin. El catálogo incluye las transiciones de fase y la gestión del proceso; cada respuesta proyecta el mínimo que decide y el siguiente contexto necesario (una necesidad, una invocación). La CLI antigua solo se conserva temporalmente para el arnés de equivalencia y se retira al cerrar la transición.

Criterio de aceptación: el descubrimiento devuelve el catálogo completo; una petición con campo desconocido devuelve `invalid` con razón; ninguna operación obliga a una segunda invocación para saber qué sigue; el binario distingue `runner aif` de cualquier otra entrada sin inspeccionar contenido.

### R7 — Portabilidad y despliegue

Un binario estático por plataforma (Linux y Windows), sin runtime en destino. Distribución estable mediante binarios por plataforma en GitHub Releases; compilación Go solo durante el desarrollo; sin binarios compilados en el repositorio ni infraestructura de distribución adicional (D1). La sustitución del runner Python es reversible sin perder trabajo ni delegaciones: la migración importa el estado y el ledger existentes, y `export-legacy` —operación del propio runner— exporta de vuelta el estado en el formato del Python cuando se revierte.

Criterio de aceptación: instalación limpia en ambas plataformas desde el instalador actualizado; la batería prueba migrar, trabajar, exportar y continuar con el Python sin pérdida; el binario se identifica con `--version`.

### R8 — Reducción medida del contrato

La reducción de instrucciones administrativas del orquestador se mide y se justifica instrucción a instrucción: cada regla eliminada señala la operación o garantía que la sustituye; ninguna garantía se retira para reducir líneas.

Criterio de aceptación: la propuesta de contrato incluye la tabla de correspondencia y la medición contra el contrato actual.

### R9 — Ruido contextual

La reducción de invocaciones y ruido se compara contra la línea base del research: bytes y líneas por operación clave (transición, arranque, consulta de plugin, ciclo de una delegación).

Criterio de aceptación: la batería o el informe de implementación incluye la comparación antes/después con la misma metodología del WP-01.

## No objetivos

Los límites del producto viven en PRODUCT §Límites y no se repiten aquí; para el alcance de esta versión rige además: sin esquemas de persistencia alternativos en paralelo y sin funcionalidad sin necesidad demostrada.

## Criterios de éxito

La tabla única vive en PRODUCT §Criterios de éxito; cada requisito la sirve así: R1→Recuperación y Fases; R2→Edición manual y Duplicidades; R3→Edición manual; R4/R6→Contexto operativo; R5→Contexto operativo y Regresiones; R7→Regresiones; R8→Instrucciones administrativas; R9→Contexto operativo.
