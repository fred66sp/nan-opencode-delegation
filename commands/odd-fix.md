---
description: "Corrige un defecto concreto (ODD): reprodúcelo, arréglalo, verifícalo e informa"
agent: orchestrator
subagent: false
---
Corrige el defecto descrito en $ARGUMENTS siguiendo el contrato del orquestador.

- Si $ARGUMENTS falta o no describe un defecto observable, pide la descripción y no toques archivos.
- Conserva el flujo, el tablero y las fases existentes: no dupliques sus responsabilidades ni añadas pasos propios.
- No hagas commit, push, merge ni etiqueta sin la confirmación explícita de la persona.
