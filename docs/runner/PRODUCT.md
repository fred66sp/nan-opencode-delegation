# PRODUCT.md — Runner de orquestación (nan-opencode-delegation)

Estado: aprobado por el PO (WP-02), con los ajustes de la revisión incorporados. Fecha: 2026-11-27.

## Problema

El proceso de orquestación vive repartido entre un runner casi exclusivamente renderizador y un contrato en prosa que el agente ejecuta sin garantías de software. Consecuencias observadas:

- El agente edita a mano `.run/delegations.jsonl`: sin atomicidad, sin unicidad de huella, sin transiciones válidas, sin conciliación con el estado real de OpenCode. En la muestra actual hay 2 encargos `launched` sin cerrar y sesión registrada solo en 15 de 88 líneas.
- El formato del informe de subagente vive en prosa: el checker valida su forma después, cuando el error ya ocurrió, y no puede probar verdad ni cobertura.
- La operativa está duplicada (fases 6/7 en runner y contrato) y reproducida en cuatro artefactos que deben mantenerse sincronizados a mano.
- El agente reconstruye su contexto operativo invocando herramientas dispares (`acm start`, `tasks-ai list`, `git`, ficheros): 69 líneas del CLI del tablero donde 1 bastaría; la consulta del plugin ya pesa 14 KB por llamada.
- El binario Python y su ruta están fijados en cuatro sitios; el contrato `--json` tiene dos parsers con rigor distinto.

## Propósito

Un ejecutable único, del proyecto, que gestione el **proceso de trabajo** del orquestador: estado, contratos, validación y registro deterministas. Gestiona el trabajo; no dirige el razonamiento.

## Propuesta de valor

1. **Estado operativo accionable en una invocación**: fase actual, operación disponible, asuntos pendientes y advertencias, con el principio AIF de una necesidad, una invocación.
2. **Contratos ejecutados, no recitados**: lo que hoy es prosa que el agente debe recordar pasa a ser validación que el software aplica.
3. **Un escritor lógico del registro de delegaciones**, con transiciones válidas, deduplicación exacta y conservación de retornos y pendientes.
4. **Menos ruido y menos instrucciones**: el contrato pierde su parte administrativa; las respuestas se proyectan al mínimo que decide.

## Responsabilidades del runner

- **Ciclo de fases**: las once fases y sus transiciones, con evidencia registrada y reinicio.
- **Registro de delegaciones**: preparación, detección de duplicidades exactas, registro con sesión, cierre con estado válido, conservación de retornos y pendientes. Distinción explícita entre **estado registrado** y **estado real** de OpenCode: la conciliación no se concluye por antigüedad; un encargo sin cerrar produce advertencia de contraste, nunca abandono automático.
- **Contratos de forma**: evidencia por fase y estructura del informe de retorno como operaciones AIF (composición esperada y comprobaciones declaradas).
- **Arranque integrado y consultas**: el arranque integrado presenta memoria, tablero, Git y fase en una invocación degradante y puede abrir una sesión ACM; las consultas de estado —las que consumen los plugins y el panel— son estrictamente de solo lectura y no tienen efectos.
- **Recuperación segura**: un estado corrupto no provoca reinicio silencioso ni pérdida de delegaciones, evidencias o pendientes; la recuperación es una operación explícita que conserva lo rescatable y declara lo perdido.

## Responsabilidades que permanecen en el orquestador

- La proporcionalidad del recorrido y qué fases aportan.
- El contenido y la veracidad de cada evidencia: el runner valida forma, no verdad.
- El juicio sobre retornos de subagentes y su integración.
- Los re-encargos y las correcciones con criterio ya masticado.
- Las autorizaciones de la persona: registrarlas no es verificarlas; el runner las anota y las recuerda, nunca las supone.
- La resolución de cada encargo que no concilia: advertencia del runner, decisión del agente.
- El registro en ACM (decisiones, aprendizajes, checkpoints, cierres) y en Tasks-AI: el agente interactúa directamente con ambos por sus propias interfaces AIF; el runner solo presenta su contexto en el arranque.

## Papel de AIF

Biblioteca nativa Go, sin ampliaciones de especificación. AIF aporta el envelope, los ocho resultados, la validación declarativa de campos y el descubrimiento. Cada operación del runner declara su contrato; el agente lo aprende por `input_needed`/`invalid` y no lo memoriza. El perfil CLI (stdin/stdout/códigos de salida, hoy duplicado en cada binario del ecosistema) vive en el propio runner.

## Gestión de delegaciones

Ciclo cubierto por el runner: registro → lanzamiento (con sesión) → seguimiento → retención del retorno → cierre con estado válido → pendientes derivados. El runner no lanza, detiene ni interviene sobre subagentes: el lanzado queda constatado en su registro y el estado real se contrasta en OpenCode. La deduplicación es exacta; el parecido semántico entre encargos es juicio del agente.

## Relación con otros componentes

- **OpenCode**: los plugins consultan al binario; el runner no observa sesiones de OpenCode, solo las que el agente o el entorno declaran.
- **ACM**: fuente de memoria de sesión. El runner la presenta en el arranque integrado; el registro de memoria queda en el agente. No duplica su estado.
- **Tasks-AI**: el tablero sigue siendo la única superficie de trabajo registrada; el runner no replica tareas ni asume sus operaciones: presenta su contexto en el arranque y enlaza referencias.
- **Git**: el estado del árbol lo informan los comandos de Git; el runner no sabe de ramas ni de commits más allá de presentarlos en el arranque.

## Límites del producto

- No lanza ni interviene subagentes; no ve sesiones ajenas a las que le declaran.
- No valida verdad, suficiencia ni autorizaciones: valida forma y las promesas que puede hacer cumplir.
- No decide el razonamiento del agente: presenta, recuerda y bloquea solo lo determinista.
- Sin servicios permanentes, sin motor genérico de workflows y sin ampliaciones a AIF en este alcance.
- Una vez demostrada su validez, sustituye por completo al runner Python; mientras tanto conviven sin interferirse.

## Principios de diseño

1. Gestionar el proceso, no el razonamiento.
2. Una necesidad, una invocación; proyección mínima en cada respuesta.
3. El runner solo hace lo que el software puede garantizar; lo demás, advertencia o recuerdo, nunca promesa.
4. Una fuente de verdad por operativa; la duplicación es defecto.
5. Persistencia tan simple como permitan las necesidades observadas: la concurrencia real de invocaciones se resuelve con el diseño técnico, no con complejidad preventiva.
6. No burocracia: el runner facilita; si un contrato añade fricción sin garantía, sobra.

## Alcance mínimo (primera versión)

1. Binario Go del runner con: fases y transiciones, arranque integrado degradante (ACM + tablero + Git), superficie JSON para plugins y registro de delegaciones como operaciones AIF.
2. Instaladores y plugins adaptados al binario; retirada del artefacto Python.
3. Contrato del orquestador reducido a lo cognitivo; la operativa administrativa pasa al runner como contratos ejecutados.
4. Fuera de alcance: lanzar subagentes desde el runner, sesiones propias de AIF, motor de workflows y administración del tablero.

## Criterios de éxito observables

Con línea base del research (WP-01):

| Criterio | Base observada | Resultado esperado |
|---|---|---|
| Edición manual del ledger | 88 líneas a mano, 2 huérfanos, sesión 15/88 | Ninguna escritura del agente: 0 líneas manuales |
| Duplicidades activas | Ninguna comprobación hoy | Duplicidad exacta bloqueada por el runner |
| Contexto operativo | `--json` 14 KB; tablero 69 líneas | Arranque en una invocación trazada; omisiones deliberadas |
| Instrucciones administrativas | Reglas 3/6/9 del contrato en prosa | Reducción medida y justificada instrucción a instrucción, con la operación que la sustituye |
| Fases y proporcionalidad | Once fases intactas | Sin cambio de número de fases ni omisión de skip justificada |
| Regresiones | Batería actual verde | Batería equivalente en verde con el binario |
| Recuperación | Estado corrupto → fase 1 con aviso | Recuperación explícita: lo rescatable conservado, lo perdido declarado |

Los cuantitativos carecen de línea base representativa más allá de la muestra WP-01: se miden contra esa muestra o se marcan cualitativos, sin objetivos arbitrarios.
