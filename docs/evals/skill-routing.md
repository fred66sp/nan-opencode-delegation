# Reparto de skills: lo que se midió

Registro de la comprobación que sostiene el contrato de `tk-25` del tablero: los
subagentes no reciben catálogo de skills y el orquestador conserva el catálogo y adjunta
la que cada encargo necesita. Medido el 13 de noviembre de 2026 con `opencode v2.3.0` en
la máquina de origen, sobre la configuración instalada por `install.sh`.

## Método

Sesiones no interactivas con `opencode run --agent <id> --format json "<prompt>"`, con el
paquete ya instalado en `~/.config/opencode`. Se cuentan los tokens de entrada y el
resultado de la herramienta `skill` que devuelve el flujo JSON. No se volcó el prompt
ensamblado: la evidencia es la conducta observable del agente y el coste que declara.

Los cuatro casos usan el mismo prompt de enumeración, con una instrucción que impide
usar herramientas y obliga a copiar los IDs tal cual, o a responder `LISTA_VACIA`.

## Casos

| Caso | Agente | Configuración | Resultado |
|---|---|---|---|
| Control negativo | `verify` | `tools: skill: false` (V1) | **Cargó** `go-testing-and-verification`: el campo se ignora |
| Subagente con frontmatter | `verify` | `permissions`: `action: skill`, `resource: "*"`, `effect: deny` | `LISTA_VACIA`; la carga devuelve `Permission denied: skill` |
| Subagente integrado | `general` | misma regla en `agents.general.permissions` | `LISTA_VACIA` |
| Control positivo | `orchestrator` | sin regla: conserva el catálogo | Enumera IDs reales (`adr-software`, `chained-pr`, …) y **carga** `source-cartographer` |

## Detalle de las ejecuciones

- **Control negativo.** Con el frontmatter anterior, `verify` llamó a `skill` con
  `go-testing-and-verification` y la herramienta **completó**: leyó
  `~/.config/opencode/skills/go-testing-and-verification/SKILL.md`. Entrada: 18.744
  tokens.
- **`verify` con la regla deny.** La misma petición devuelve
  `{"error":{"type":"permission.rejected","message":"Permission denied: skill"}}` y, al
  pedirle la lista, `LISTA_VACIA`. Entrada observada: 406 tokens más 13.312 de caché.
  Preguntado primero por el número, el modelo contestó «14 skills» y a la vez que la
  skill «no existía en el catálogo»: una confabulación que la enumeración literal
  desmiente. Por eso el caso bueno es el que obliga a copiar IDs.
- **`general` con la regla en la configuración.** `LISTA_VACIA`. Entrada: 9.214 tokens
  más 4.096 de caché.
- **`orchestrator`.** Enumera los ocho primeros IDs que ve y carga
  `source-cartographer` con la herramienta `skill`, que **completa** y devuelve su
  contenido y su base directory. Entrada: 19.957 tokens en frío. Es el control que
  valida el instrumento: si el orquestador también dijera `LISTA_VACIA`, la prueba no
  estaría midiendo el reparto.

## Qué demuestra y qué no

Demuestra que el permiso `skill` con `effect: deny` **oculta las skills al subagente y
rechaza la carga**, en el frontmatter y en la configuración de un agente integrado, y que
el orquestador conserva ambas cosas.

No demuestra que el orquestador adjunte siempre la skill adecuada: eso depende de su
prompt. La guarda automática no existe; la cubre la regla del punto 4 de
`agents/orchestrator.md`, y el límite está declarado en el README.

El coste baja, pero la cifra exacta depende de la caché de cada ejecución: la lectura en
frío con catálogo ronda los 18–20 mil tokens y sin catálogo los 13–14 mil. Es una
diferencia de orden, no una medición de precisión.

`tools: skill: false` **no** sirve en V2: es un campo V1 que el esquema acepta y el
motor ignora. Queda anotado aquí porque el paquete llegó a escribirlo y la guarda de
`check-package.sh` vigila que no vuelva.
