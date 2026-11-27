# DESIGN — Runner de orquestación (nan-opencode-delegation)

Estado: revisión WP-03.1. Requisitos en `docs/runner/PRD.md`, discurso en `docs/runner/PRODUCT.md`. Este documento fija las decisiones técnicas, su justificación y lo que queda pendiente.

## 1. Arquitectura

Un binario Go, módulo propio dentro del paquete (directorio `runner-go/`). Dependencias: AIF como contrato de interacción, `modernc.org/sqlite` para persistencia (decisión D, §5) y la biblioteca estándar. Sin ORM, sin capas genéricas de persistencia.

```
runner-go/
  cmd/orchestrator-runner/main.go   wiring: subcomandos, perfil AIF (stdin/stdout/códigos), --json
  internal/phase/    fases como datos, transiciones, recuperación
  internal/ledger/   delegaciones: identidad, ciclo, guardas
  internal/report/   contrato de forma del informe (lógica interna del retorno)
  internal/boot/     contexto integrado degradante (acm, tablero, git, fase)
  internal/store/    SQLite: esquema, transacciones, migración, export-legacy
  internal/aifops/   catálogo de operaciones AIF sobre interaction.Executor
```

`modernc.org/sqlite` (SQLite puro Go, sin CGo) es la misma dependencia que ya usa ACM: patrón de conexión conocido — fichero por proyecto, `busy_timeout`, transacciones `BEGIN IMMEDIATE`, FK activas. Su coste (~10 MB de binario) es el precio de tener concurrencia e integridad nativas en lugar de un mecanismo propio sobre JSON.

## 2. Interfaz del runner

AIF es la interfaz principal del agente con el runner (patrón ACM/Tasks-AI). Nombres provisionales.

| Entrada | Rol | Efectos |
|---|---|---|
| `runner start` | Contexto operativo integrado (memoria, tablero, Git, fase, pendientes), degradante por sección, formato legible | Solo lectura salvo abrir/recuperar la sesión ACM, como hace `acm start` |
| `runner aif` | Descubrimiento y ejecución de operaciones AIF: envelope JSON por stdin, una respuesta por stdout, código de salida normado | Depende de la operación |
| `runner --version` | Identificación del ejecutable (versión y commit) | Ninguno |
| `runner --json` | Consulta específica de solo lectura para plugins y TUI (R5) | Ninguno; sin base de datos devuelve el estado vacío sin crearla |
| `runner --json --latest` | Variante de `--json` para superficies de solo lectura sin entorno de sesión (el panel TUI): responde el estado de la sesión que más recientemente avanzó, con su id en `state.session`, o el estado vacío si no hay ninguna; no crea base ni ficheros | Ninguno |
| `runner --help` | Uso convencional | Ninguno |

El modo AIF es el subcomando explícito `aif`: **no** se detecta por el contenido de stdin. Las transiciones de fase y la gestión del proceso viven en el catálogo AIF (`phase_next`, `phase_skip`, `phase_reset`); el contrato definitivo es el catálogo, no la CLI antigua. Los subcomandos `next/skip/reset` se conservan temporalmente **solo** como superficie del arnés de equivalencia con el runner Python (§8) y se retiran al cerrar la transición. `start` es la única entrada fuera del contrato AIF, con la misma justificación con la que ACM mantiene `acm start`: el contexto de continuidad se presenta, no se opera.

## 3. Catálogo de operaciones AIF

Prefijos: `phase_` para el proceso, `deck_` para delegaciones. La validación del informe es lógica interna de `deck_return` (§3.2): no hay operación previa obligatoria. El catálogo completo se descubre por AIF; las descripciones de las operaciones llevan la regla de uso (patrón tasks-ai).

### 3.1 Proceso

| Operación | Campos | Resultado |
|---|---|---|
| `phase_next` | `evidence` | Avanza la fase registrando la evidencia; la respuesta trae la fase siguiente y, si existen, los encargos abiertos sin conciliar |
| `phase_skip` | `target`, `reason` | Salto hacia delante con motivo; sin fases intermedias omisibles, `invalid` |
| `phase_reset` | `reason` | Reinicia fase y evidencia; nunca toca delegaciones |

### 3.2 Delegaciones — granularidad por necesidad, no CRUD

| Operación | Campos | Garantías |
|---|---|---|
| `deck_prepare` | `agent`, `goal`, `scope[]`, `checks[]`, `criteria`, `task?` | Calcula la huella (§4), bloquea duplicados abiertos, crea la ejecución con `id` propio: `denied` con el `id` existente si hay abierto con la misma huella |
| `deck_launch` | `id`, `session?`, `subagent_session?` | `prepared→launched`; separa sesión del orquestador (defecto `OPENCODE_SESSION_ID`) de la identidad de ejecución del subagente. La confirmación del lanzamiento ocurre **después** de la acción efectiva de OpenCode, nunca antes; si la identidad del subagente ya se conoce en ese momento, se ata en la misma invocación — sin seguimiento adicional |
| `deck_return` | `id`, `status`, `checks[]` (`check`, `observed`, `failed?`), `risks?` | Una sola invocación: valida la estructura del informe con la lógica interna del checker, **conserva la evidencia válida**, registra el retorno y **devuelve los pendientes**. Las comprobaciones esperadas de la preparación son la referencia: cada una queda identificada como ejecutada (con resultado observado), fallida (`failed` con motivo) o no ejecutada (esperada y no reportada); los pendientes derivados son únicamente los que el software puede determinar — lo esperado sin reportar y los fallos declarados. `invalid` lista los incumplimientos de forma |
| `deck_close` | `id`, `outcome` (`done/superseded/absorbed`), `pendings?` | Cierre formal; guardas de §4: no elude una ejecución activa |
| `deck_note` | `id`, `text`, `observed?`, `subagent_session?` | Seguimiento y conciliación en una operación: `observed` marca lo que el agente declara haber observado en OpenCode (aserto, no verificación); `subagent_session` ata la identidad de ejecución |
| `deck_list` | `state?` (`open/orphans/all`) | Proyección mínima: abiertos, retornos pendientes de integración, avisos de contraste |

Decisiones de granularidad justificadas: **seguimiento y conciliación fusionados** en `deck_note` — la conciliación no es una operación distinta, es una nota que declara lo observado; separarlas obligaría a dos invocaciones para un mismo hecho. **El cierre sigue siendo una operación independiente** porque retorno recibido ≠ trabajo aceptado: el orquestador valora verdad y cobertura antes de `deck_close`, y el cierre es la decisión, no el dato. **La validación del informe va dentro de `deck_return`** porque la necesidad real es una (procesar el retorno); el checker queda como lógica interna reutilizable, no como paso obligatorio.

Proyección mínima: cada respuesta lleva su resultado y, solo si existen, los avisos de contraste afectados (≤3 líneas). La fase no se repite en cada respuesta: llega por el plugin o por `start`.

## 4. Identidad y ciclo de las delegaciones

**Identidad.** `id` = 12 hex aleatorios (`crypto/rand`), único por **ejecución**: dos lanzamientos legítimos del mismo encargo tienen ids distintos y conservan cada uno su historial completo (retornos, notas, cierre).

**Huella.** La calcula el runner, no el agente: `sha256` sobre la concatenación normalizada de `agent | task | goal | scope (ordenado) | checks (en orden) | criteria`, con normalización NFC, recorte y colapso de espacios. La huella identifica el **contenido del encargo**; el `id`, la ejecución.

**Estados y bloqueo de duplicados.** `prepared → launched → returned → closed`, cierre con `done | superseded | absorbed`. Bloquean un duplicado con la misma huella los estados abiertos: `prepared`, `launched` y **`returned` (pendiente de integración)**. Cerrados no bloquean: un re-encargo tras cierre es una ejecución nueva con `id` propio.

**Guardas del cierre.** No se permite usar `deck_close` (formal o `superseded`) para eludir la detección de una ejecución activa:

- cierre desde `prepared` o `launched` **sin retorno retenido**: `denied` salvo que la propia invocación de cierre lleve `observed` con el contraste declarado (p. ej. «el subagente `ses_…` no existe en OpenCode» o «la ejecución terminó sin retorno»); una observación cualquiera anterior no basta para concluir que una ejecución ha terminado — el contraste se declara en el cierre y el runner lo registra verbatim sin juzgarlo. Motivo: un encargo `prepared` puede haberse lanzado realmente antes de una interrupción, y el estado registrado no puede descartarlo;
- cierre desde `returned`: permitido — hay retorno retenido que el orquestador ya valoró.

**Preparación y lanzamiento separados** (se conserva): entre ambas hay una acción externa de OpenCode —el lanzamiento del subagente—, por lo que son invocaciones distintas con estados distintos. La interrupción entre ellas es un estado posible y legítimo (§5, ejemplos).

**Sesiones.** La sesión del **orquestador** es contexto de registro (defecto `OPENCODE_SESSION_ID`); la identidad de ejecución del **subagente** en OpenCode es otro dato (`subagent_session`), que se conoce tras el lanzamiento y se ata por `deck_launch` si ya se conoce o por `deck_note` después. El runner no asume que uno identifica al otro.

**Idempotencia de peticiones ≠ deduplicación de encargos.** Las operaciones mutadoras aceptan el campo opcional `request_id`: su primera ejecución almacena la respuesta junto a la clave y una reinvocación con la misma clave devuelve **exactamente esa respuesta** sin efectos nuevos — es la protección ante una respuesta perdida después del commit de SQLite. Sin `request_id`, rigen las reglas normales: la deduplicación de encargos compara huellas y estados, no peticiones. Las operaciones de fase también lo aceptan: reintentar `phase_next` tras una respuesta perdida sin la clave avanzaría dos fases.

**Aislamiento del estado de fases por sesión del orquestador.** El estado de fase vive por sesión (`OPENCODE_SESSION_ID` o campo explícito): una sesión no avanza ni reinicia inadvertidamente el estado de otra. Las delegaciones siguen siendo del proyecto — visibles y compartidas—, con sus sesiones como datos. Las consultas `--json` devuelven el estado de la sesión invocante (el plugin hereda el entorno de su sesión) y, sin sesión identificable, el estado vacío; la variante `--latest` responde, para superficies sin entorno de sesión, el estado de la sesión que más recientemente avanzó —nombrándola— o el vacío, sin presentar una fase por defecto como real.

**Coherencia del contrato.** El catálogo AIF es la única fuente de las operaciones: la ayuda se genera de él, `recover` aparece en el descubrimiento y las respuestas de transición llevan el siguiente contexto necesario (fase siguiente, avisos pendientes) — el agente no depende del plugin para saber qué sigue.

## 5. Interacción — ejemplos completos

Salida condensada; los códigos de salida salen del resultado AIF (0 `executed`/descubrimiento, 2 `invalid`/`input_needed`, 3 contrato, 4 `denied`, 1 `failed`).

**Encargo normal** (5 invocaciones del runner: las 4 de fases de hoy son independientes):

```
$ runner aif <<< '{"aif":"0.2","operation":"deck_prepare","input":{"agent":"general",
  "task":"tk-99","goal":"Actualizar los tres documentos con WP-03.1",
  "scope":["docs/runner/PRD.md","docs/runner/DESIGN.md","docs/runner/orchestrator-proposal.md"],
  "checks":["grep de los ajustes en los tres documentos"],
  "criteria":"Los tres documentos reflejan WP-03.1 sin ampliar alcance."}}'
→ {"result":"executed","outcome":{"id":"a41f…","fingerprint":"9c2b…","state":"prepared"}}

$ runner aif <<< '{"aif":"0.2","operation":"deck_launch","input":{"id":"a41f…"}}'
→ {"result":"executed","outcome":{"state":"launched","orchestrator_session":"ses_…"}}

# OpenCode lanza al subagente (acción externa) y devuelve su sessionID
$ runner aif <<< '{"aif":"0.2","operation":"deck_note","input":{"id":"a41f…",
  "subagent_session":"ses_…"}}'
→ {"result":"executed"}

$ runner aif <<< '{"aif":"0.2","operation":"deck_return","input":{"id":"a41f…",
  "status":"completed","checks":[{"check":"grep de los ajustes","observed":"5 ajustes en 3/3 documentos"}]}}'
→ {"result":"executed","outcome":{"pendientes":[]}}

$ runner aif <<< '{"aif":"0.2","operation":"deck_close","input":{"id":"a41f…","outcome":"done"}}'
→ {"result":"executed","outcome":{"pendientes":[]}}
```

**Retorno parcial y corrección** (la evidencia anterior nunca se pierde; el retorno v2 es una corrección, no un reemplazo):

```
$ deck_return {"id":"a41f…","status":"partial","checks":[
    {"check":"grep de los ajustes","observed":"2 ajustes en 3/3"},
    {"check":"wc -l","failed":"no ejecutado: presupuesto agotado"}]}
→ {"result":"executed","outcome":{"pendientes":["wc -l sin ejecutar","docs pendientes: …"]}}

$ deck_return {"id":"a41f…","status":"completed","checks":[
    {"check":"wc -l","observed":"DESIGN 149, PRD 78, proposal 46"}]}
→ {"result":"executed","outcome":{"returns":2,"pendientes":[]}}   # v1 y v2 conservadas
```

**Re-encargo legítimo** (misma huella, ejecución nueva):

```
$ deck_prepare {…mismos campos…}
→ {"result":"denied","outcome":{"existing":"a41f…","state":"returned"}}   # abierto: bloquea
$ deck_close  {"id":"a41f…","outcome":"superseded"}                        # con constancia previa (§4)
$ deck_prepare {…mismos campos…}
→ {"result":"executed","outcome":{"id":"b7e2…","fingerprint":"9c2b…"}}     # huella igual, id nuevo
```

**Interrupción entre preparación y lanzamiento** (la sesión muere entre ambas):

```
# estado: a41f… en prepared, sin launched
$ runner start
→ …Pendientes: 1 encargo prepared sin lanzar: a41f… — contrasta en OpenCode si el subagente vive…

$ deck_launch {"id":"a41f…"}            # si vivía: se confirma
$ deck_close {"id":"a41f…","outcome":"superseded",
              "observed":"el subagente no existe en OpenCode: la sesión terminó sin lanzarlo"}
# cierre desde prepared sin retorno: exige el contraste declarado en el propio cierre (§4)
```

Comparación de invocaciones con el proceso actual, por encargo: hoy son 2-3 **ediciones manuales** del ledger (alta y actualización de status), una comprobación de duplicados a ojo y opcionalmente 1 invocación del checker — sin garantías. Con el runner son 4-5 invocaciones AIF, cero escrituras manuales, con deduplicación, estructura validada y pendientes devueltos. Las transiciones de fase mantienen el mismo número de invocaciones que hoy (`next`/`skip`); `start` reduce a una la recomposición de contexto que hoy exige varias.

## 6. Persistencia

**Decisión: SQLite directo con `modernc.org/sqlite`**, adoptada por instrucción del PO tras el diagnóstico del CAS: comparar hash y renombrar después permite que dos procesos superen la comparación y uno sobrescriba al otro — no es concurrencia, es ilusión de concurrencia. No se construye un mecanismo propio sobre JSON.

- **Base de datos local por proyecto**: `.run/orchestrator.db`. Sin ORM: el esquema vive en SQL, el acceso en `internal/store` con `database/sql`.
- **Transacciones e integridad nativas**: `BEGIN IMMEDIATE` para toda mutación, `busy_timeout`, FK activas, `CHECK` en los enums (`status`, `outcome`), `UNIQUE(id)` y un índice parcial que garantiza la unicidad de huella entre filas abiertas. Cada operación mutadora es una transacción: la atomicidad de la operación es la de SQLite.
- **Aislamiento por sesión cuando corresponda**: la base es del proyecto; las filas de delegaciones llevan la sesión del orquestador y la del subagente como columnas.
- **Consultas de plugins estrictamente de solo lectura, incluso sin base**: `--json` y `deck_list` abren en modo `ro` (o sin conexión si el fichero no existe) y devuelven el estado vacío (fase 1, cero delegaciones) sin crear nada. Solo las operaciones mutadoras crean la base.
- **Recuperación explícita y segura**: ante fichero dañado, toda mutación devuelve `failed E_STATE_CORRUPT` y las consultas declaran `corrupt`; `runner recover` archiva el fichero, intenta rescatar tabla a tabla lo legible, reconstruye lo mínimo (fase 1) y presenta qué se conservó, qué se archivó y qué se perdió. Nunca reinicio silencioso ni pérdida no declarada.
- **Migración sin pérdida**: la primera operación mutadora con estado legado presente importa `.run/orchestrator-runner.json` (fase y evidencias) y `.run/delegations.jsonl` (88 líneas de la muestra, campos nuevos con defecto), renombra los originales a `.migrated` y lo informa en `start`. Idempotente: sin legado, no hace nada.
- **Rollback y sustitución reversible** (matiz del PO: conservar el Python no basta si el estado deja de ser compatible): una vez migrado a SQLite, el Python solo ve ficheros viejos. La reversión es una operación del propio runner: `runner export-legacy` regenera el estado JSON y `delegations.jsonl` a partir de la base (fase, evidencias y delegaciones completas) para que el Python continúe donde quedó. Durante la transición ambos artefactos conviven; el punto de conmutación es único (§8) y la batería prueba la ida (migrar) y la vuelta (exportar y seguir con el Python) sin pérdida.

## 7. Integración

| Artefacto | Destino | Cambio |
|---|---|---|
| `orchestrator-runner.py` | Se retira al cerrar la transición | `export-legacy` mantiene su compatibilidad hasta entonces |
| `delegation-report-checker.py` | Se retira tras equivalencia (D2) | La lógica pasa a `internal/report`; consumidores localizados: 58 referencias en batería, README, PRD del paquete y guía web |
| Plugins de fase y TUI | Adapta | Llamada al binario; parsers intactos (R5). **Inyección sin redundancia**: el plugin solo inyecta cuando cambia la fase o hay avisos pendientes, y con el mínimo (fase + evidencia esperada + advertencias), porque el contexto completo ya lo da `start` |
| `commands/odd-fix.md` | Conserva | Sin cambios |
| `orchestrator.md` | Sustituye | Propuesta completa en `docs/runner/orchestrator-proposal.md` |
| `install.sh` / `install.ps1` | Adapta | Descarga el binario estable de GitHub Releases (D1); compilación Go solo en desarrollo |
| `check-package.sh` | Adapta | `py_compile`→build/vet; grupo del checker→equivalencia de `deck_return`; grupo del ledger→operaciones `deck_*`; la copia embebida del runner en la guía desaparece |
| Guía web, README, CHANGELOG, PRD del paquete | Adapta | En la implementación |
| ACM, Tasks-AI, Git | Sin cambios | Responsabilidades e interfaces propias (PRODUCT §Relación) |

## 8. Validación

1. **Unidad** (`go test`): transiciones y recuperación, identidad y ciclo (duplicados abiertos, guardas de cierre, idempotencias, reintentos tras respuesta perdida), retorno parcial con correcciones conservadas, huella normalizada.
2. **Contrato AIF**: conformidad CJ-01..CJ-14 contra el binario; campos por operación (`input_needed`, `invalid`, desconocidos); sin detección implícita de modo.
3. **Equivalencia funcional con el Python**: el arnés usa la CLI antigua conservada temporalmente (§2): misma secuencia contra ambos, comparando estados, códigos de salida y contrato `--json`. Equivalencia del checker: los casos del checker actual ejecutados contra `deck_return`.
4. **Persistencia**: concurrencia real (dos procesos mutando: serialización por transacción, sin pérdida); corrupción y `recover`; migración del estado y ledger existentes byte a byte (la muestra de 88 líneas como fixture); consultas `ro` sin base creada.
5. **Rollback**: migrar → trabajar → `export-legacy` → el Python continúa con todo el trabajo; ida y vuelta sin pérdida.
6. **Batería del paquete**: grupos de `check-package.sh` adaptados (§7) en verde en ambas plataformas.
7. **Recorridos con agentes reales**: trivial, funcional y crítico en proyecto sandbox con el contrato propuesto activo.
8. **Métricas**: comparación antes/después con la metodología WP-01 (bytes y líneas por operación clave) y recuento de invocaciones por encargo (§5).

## 9. Decisiones adoptadas y limitaciones

Adoptadas en WP-03.1: SQLite directo con `modernc.org/sqlite` y rollback vía `export-legacy` (§6); AIF como interfaz principal con `start`/`aif`/`--version`/`--json` y fases en el catálogo, sin detección implícita de modo (§2); catálogo por necesidades, checker interno, conciliación fusionada en `deck_note`, cierre separado justificado (§3); identidad por ejecución con huella calculada por el runner y guardas de cierre (§4); semántica de retorno, correcciones e idempotencia (§5); D1 = compilación en desarrollo + binarios estables por plataforma en GitHub Releases, sin binarios en el repositorio (§7); D2 = checker integrado y retirado tras equivalencia con sus consumidores comprobados (§7).

Limitaciones y pendientes declaradas: el arnés de equivalencia exige conservar la CLI antigua hasta cerrar la transición (deuda explícita, retirada al final); el runner sigue sin observar OpenCode — las guardas de cierre son deterministas sobre declaraciones, no sobre el mundo real; `modernc.org/sqlite` entra en el `go.mod` en la implementación (no se instala nada en este WP); la inyección sin redundancia del plugin es un cambio de comportamiento observable que la batería deberá cubrir con un caso propio.
