# Evidencia de las evaluaciones de los agentes

Este directorio conserva las pruebas con las que se validaron los cinco agentes de
delegación antes de empaquetarlos. El material se generó los días 6 y 7 de noviembre de
2026 en temporales que luego se borraron; se recuperó de la base de sesiones de OpenCode
(copia hecha por la persona de `opencode.db`) y de los resúmenes de sesión de Engram.
No es una batería ejecutable: es el registro de lo que se probó y lo que falló.

## Qué hay

| Ruta | Qué es |
|---|---|
| [`smoke/`](smoke/) | 9 transcripciones del smoke test de los cinco agentes contra un repo Go desechable (`/tmp/opencode/delegation-smoke`, borrado) |
| [`measure/`](measure/) | 19 transcripciones y [`tk-281-results.csv`](measure/tk-281-results.csv) de la medición `glm5.3-flash#high` frente a `deepseek-v4-flash#high` como worker: 3 proyectos × 3 tareas × 2 modelos desde worktrees limpios |
| [`session-summaries.md`](session-summaries.md) | Resúmenes de sesión que los propios agentes escribieron en Engram al terminar (verbatims) |

## Qué demostró el smoke test (tk-280 del tablero de tasks-ai)

Repo sembrado con un fallo (`Mul` con `a+b` en `calc.go:5`) y un encargo completo.

- **verify**: el allowlist funcionó como está diseñado — el compuesto
  `cd … && ls …` se denegó y los comandos simples (`make test`, `go test -count=1`,
  `go vet`, `go build`) se ejecutaron por `workdir`. No editó nada.
- **review**: devolvió el fallo de `Mul` con `calc.go:5`, la falta de test y una
  entrada inexacta del CHANGELOG. No editó nada.
- **docs**: escribió solo `CHANGELOG.md` y `docs/guide.md`; los intentos sobre
  `src/` y `calc.go` quedaron denegados por permisos.
- **ui**: pidió permiso antes de cargar la skill `playwright-cli` (el `ask`
  bloquea y en no interactivo se auto-rechaza, que es el comportamiento esperado).
- **orchestrator**: delegó la implementación en `general`, verificó la salida por
  su cuenta (`git status`, `go vet && go test`) y lanzó `review` después.

El propio smoke destapó dos fallos de configuración, corregidos en los agentes
actuales: los globs de `edit` (`*` incluye `/`, así que `**/*.md` no casaba los
`.md` de la raíz y se usan `*.md`) y `#variant` en el modelo bajo la clave legacy
`agent` (no se aceptaba; `variant` va aparte).

## Qué demostró la medición (tk-281 del tablero de tasks-ai)

Modelo worker (`general`), variantes `#high`, una repetición por celda, tareas
micro y autocontenidas (Clamp, ParseSize, Chunk). Métricas derivadas del CSV:

| modelo | verde (dirigida / suite) | pared media | docengine | go-dev-agent | project-task-manager |
|---|---|---|---|---|---|
| `deepseek-v4-flash#high` | 9/9 · 9/9 | 31,5 s | 19,1 s | 40,7 s | 34,7 s |
| `glm5.3-flash#high` | 9/9 · 9/9 | 47,0 s | 48,1 s | 39,6 s | 53,4 s |

Cero regresiones y solo los archivos pedidos en las 18 ejecuciones. Decisión
registrada entonces: `general` se queda con `deepseek-v4-flash#high` (velocidad a
igualdad de calidad); orchestrator y review no se tocaron porque la medición solo
cubría el rol worker. La primera fila del CSV (`task=?`) es una ejecución de
calentamiento, fuera de la tabla. La pared del CSV difiere en décimas de la
publicada en el tablero por el punto de corte (sesión completa frente a tarea).

## Procedencia y límites

- Las transcripciones se volcaron de `session_v2` + `session_message` de una copia
  local de la base de OpenCode; los razonamientos y las salidas de herramientas
  están íntegros salvo recortes de 300 caracteres en los argumentos de `tool`.
- El registro formal de las evaluaciones (descripciones, decisiones y comentarios)
  vive en el tablero de `workshop-golang/projects-out/tasks-ai`, tareas tk-280 y
  tk-281, hijas de la épica tk-276.
- Los repos de pruebas (`delegation-smoke`, los `measure/*`) eran desechables y ya
  no existen; las transcripciones son la única reconstrucción posible.
