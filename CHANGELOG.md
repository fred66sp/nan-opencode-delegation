# Registro de cambios

Cambios con valor para quien usa el paquete, por versión. El formato sigue las
recomendaciones de *Keep a Changelog* y el versionado es semántico. Lo que se anota aquí
al publicar; la historia de cada cambio, commit a commit, vive en Git.

## [Sin publicar]

### Añadido

- Paquete instalable con la delegación de subagentes `nan` para OpenCode V2: los cinco
  agentes de configuración global (`orchestrator` primario; `verify`, `review`, `docs` y
  `ui` subagentes con permisos acotados), los overrides de modelo de los agentes
  integrados `explore` y `general`, y copias de seguridad automáticas al tocar una
  configuración que ya exista.

- Instaladores para Windows (`install.ps1`) y Linux/macOS/WSL (`install.sh`): copian los
  agentes, funden el bloque `agent` en la configuración global conservando las claves
  ajenas y, si el fichero lleva comentarios JSONC, dejan el fichero intacto e imprimen el
  bloque para pegarlo a mano.

- Guía completa de la orquestación en `index.html`: arquitectura, contrato de delegación,
  permisos con sus lecciones, instalación en ambas familias de sistema, verificación,
  errores y límites conocidos. Se abre en `file://` sin red ni dependencias; tema claro
  y oscuro con cero JavaScript.

- Tablero de desarrollo propio: el paquete lleva su instalación de tasks-ai en
  `.tasks-ai/` (instrucciones, contrato de etiquetas y ajustes versionados; base y
  extensiones fuera de Git) y el proyecto carga su extensión de OpenCode desde el
  `opencode.jsonc` raíz.

- `serve.sh` para dejar el servidor del tablero en segundo plano (start, status y stop,
  con registro y pid en `.run/`) y visor de escritorio en `tasks-viewer/` con el binario
  oficial de tasks-ai para Windows. El tablero del paquete escucha en el 8891 para convivir
  con el de tasks-ai, que mantiene el 8890.
