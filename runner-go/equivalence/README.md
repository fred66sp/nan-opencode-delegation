# Equivalencia runner-Go / runner-Python

Artefactos de verificación de WP-04. No son producto: son el aceite de la
sustitución reversible.

| Fichero | Qué hace |
|---|---|
| `run.sh` | Secuencia comparada (next/skip/reset/boot/terminal), migración del fixture, ciclo `deck_*`, `export-legacy` y continuación del Python. Compara `--json` por valor y el estado final con `compare_state.py`; sale 0 solo si todo coincide. |
| `metrics.sh` | Bytes y líneas de stdout por operación clave frente a su baseline Python. |
| `concurrency.sh` | Procesos reales compitiendo por el mismo deck; comprueba serialización, denegación del duplicado y ninguna pérdida. |
| `compare_state.py` | `version` exacta; de `phases`, número, nombre y evidencia esperada —no `steps` ni `warnings`—; `state` exacto salvo las divergencias declaradas: evidencia de las fases 1 y 11 y el registro de `start`. |
| `test_compare_state.py` | Pruebas dirigidas de `compare_state.py`: acepta las excepciones declaradas y rechaza cualquier otra divergencia. |
| `validate_legacy.py` | Copia literal del validador de `.run/delegations.jsonl` de `check-package.sh`. |
| `fixtures/` | Estado y ledger legados representativos (cinco líneas: `completed`, `launched`, `blocked`, tarea `null`, nota y sesión). |
| `fixtures/pinned/` | Copia pinned del runner y del checker Python: baseline histórica que el arnés usa cuando `agents/` ya no los contiene. |

## Uso

```bash
./run.sh
./metrics.sh
./concurrency.sh
python3 test_compare_state.py
```

Todos construyen el binario en un directorio temporal. `WORK=/tmp/x` fija el
directorio; `GO_BIN=/ruta/bin` usa un binario ya compilado. La baseline Python se
resuelve en este orden: `PY` (runner) y `PY_CHECKER` (checker), la copia pinned
de `fixtures/pinned/`, `agents/` y la instalación en `~/.config/opencode/agents`.

## Divergencias declaradas

- **Nombres de comando**: el Python usa `next/skip/reset/boot`; el Go usa el
  catálogo AIF (`phase_next`, `phase_skip`, `phase_reset`, `start`). La
  secuencia lógica es la misma; la CLI antigua no se replica en Go.
- **Códigos de error**: el Python sale con 1 en cualquier error de entrada; el
  Go distingue `invalid` (2), contrato (3) y `denied` (4). El arnés compara
  éxito/error y registra los códigos exactos.
- **`--json` antes de la primera mutación**: el Go es estrictamente de solo
  lectura y no lee el legado; la migración ocurre en el primer mutador. El
  Python ya vería la fase legada.
- **Evidencia esperada de las fases 1 y 11**: el runner Go nombra `runner start`
  y la fase terminal (`phase_reset`); la baseline pinned conserva la prosa de
  `acm start` y `reset`. `compare_state.py` admite exactamente ese par de
  textos por fase, en orden (Python, Go). Cualquier otro cambio en la evidencia,
  el nombre, el número o el recuento de fases sigue fallando.
- **`start` registra la sesión**: el arranque Go crea la base y registra la
  sesión en la fase 1 (`boot.StoreFor`); el `boot` Python no muta. El paso 6
  declara solo `state.exists: False != True` y comprueba además ambos lados
  (Python sin estado, Go en fase 1). El resto del estado se contrasta igual.
- **Tras el rollback**: una vez exportado y continuado con el Python, la base
  Go no reimporta (`migrated_legacy`); la vuelta exige migración nueva. Es el
  punto de conmutación único.
- **`deck_*`**: no tiene operaciones en el Python; se verifica su contrato contra
  los valores esperados, no contra el otro runner.
- El fixture es sintético y representativo; `.run/` del proyecto está excluido
  del acceso del agente, así que no se copia la muestra real.

No modificar `run.sh` una vez verde: WP-05 lo usa como prueba de sustitución.
