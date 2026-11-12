#!/usr/bin/env python3
"""Guía determinista del flujo del orquestador: la fase actual, su lista operativa y sus
advertencias. Sin dependencias y sin reloj: la misma entrada produce la misma salida.
"""

import json
import os
import sys

# El estado vive en el .run/ del directorio actual, que es el proyecto en el que se trabaja.
# shortcut: invocar desde la raíz del proyecto en el que se trabaja (este paquete u otro);
# desde otro directorio crea su propio .run/.
STATE = os.path.join(".run", "orchestrator-runner.json")

PHASES = [
    {
        "name": "Inicio de sesión",
        "when": "al empezar cualquier sesión, antes de lanzar subagentes o tocar archivos",
        "steps": [
            "Recuperar el contexto con `mem_context`; `mem_search` si la petición toca algo ya trabajado.",
            "Leer el `AGENTS.md` del proyecto y su mapa de fuentes antes de decidir nada.",
            "Consultar el tablero (en curso y listas para empezar) a través de `tasks-agent`.",
            "Comprobar la fase con `~/.config/opencode/agents/orchestrator-runner.py`; si la sesión anterior dejó otra y empiezas trabajo nuevo, borra `.run/orchestrator-runner.json` para volver a la fase 1.",
        ],
        "warnings": [
            "Nada se lanza antes de leer la memoria de la sesión y el tablero.",
            "El runner no comprueba el mundo real: informa antes de que falles; las guardas del tablero y del producto siguen siendo las que bloquean.",
        ],
    },
    {
        "name": "Clasificar el encargo",
        "when": "en cuanto llega la petición, antes de cualquier escritura",
        "steps": [
            "Clasificar la petición: pregunta, revisión, plan o implementación.",
            "Pregunta, revisión o plan son solo lectura: no se escribe sin una petición explícita de implementar.",
            "Detectar planificación: ejemplos, matices sobre decisiones ya tomadas, «es un ejemplo», «ya lo abordaremos más adelante». Ahí no se abre tablero ni se lanzan subagentes hasta el acuerdo explícito.",
            "Separar hecho (comprobado en el repo o con una herramienta) de inferencia (razonada pero sin verificar) y de decisión (elección de diseño, del humano).",
        ],
        "warnings": [
            "Sin tablero ni subagentes hasta el acuerdo explícito: una petición de plan no autoriza a implementar.",
            "Ante la duda entre plan e implementación, pregunta; no conviertas la ambigüedad en trabajo.",
        ],
    },
    {
        "name": "Tamaño y tablero",
        "when": "tras clasificar como implementación y antes de la primera escritura",
        "steps": [
            "Aplicar el test de la sesión interrumpida: si la sesión se parara ahora, ¿alguien podría terminar el trabajo con la petición original y `git diff`?",
            "Respuesta sí: trabajo pequeño. No se crea tarea nueva, salvo que la persona la quiera.",
            "Respuesta no: trabajo grande. Antes de escribir, registrar en el tablero, vía `tasks-agent`, una épica (`epic` + `ODD` + tipo) y sus subtareas (`task` + `ODD` + tipo) con el criterio observable de cada una y la petición original en un comentario.",
            "Comprobar el catálogo con `tasks-ai tags`: las etiquetas nacen ahí antes de usarse.",
        ],
        "warnings": [
            "Una tarea sin criterio no se empieza: el criterio es lo que después se contrasta con el resultado real.",
            "La épica pasa a `in_progress` cuando empieza su primera hija y no cierra hasta que cierran todas.",
        ],
    },
    {
        "name": "Explorar",
        "when": "antes de proponer o editar, en proporción a la petición",
        "steps": [
            "Leer el código y los requisitos que la petición toca.",
            "En repositorios con `.codegraph/`, `codegraph explore \"...\"` antes de buscar a mano.",
            "Usar `lsp_*` para símbolos, referencias y tipos; `grep` para texto, configuración y documentación.",
            "Mapear lo que el cambio debe alcanzar: llamadores, pruebas, fixtures, configuración y exportaciones.",
        ],
        "warnings": [
            "Lo razonado sin verificar es inferencia: contrástalo con una herramienta antes de actuar sobre ello.",
        ],
    },
    {
        "name": "Decidir",
        "when": "cuando aparece una duda de diseño o de producto",
        "steps": [
            "Resolver los hechos investigando: repo, herramientas, documentación del paquete.",
            "Elevar solo las decisiones que son del humano: producto, alcance, formato, permisos, pérdida de datos.",
            "Preguntar de una en una y esperar la respuesta.",
            "Formalizar más (un plan, una especificación) es elección explícita de la persona, nunca un cambio silencioso de proceso.",
        ],
        "warnings": [
            "Ni preguntas innecesarias ni autoridad no concedida.",
            "Puedes advertir que un cambio tiene implicaciones arquitectónicas y proponer formalizarlo; no lo decidas tú.",
        ],
    },
    {
        "name": "Delegar",
        "when": "antes de lanzar un subagente",
        "steps": [
            "Todo encargo lleva cuatro cosas: objetivo, archivos de su propiedad, comprobaciones esperadas y criterio de aceptación; si hay tablero, la referencia de la tarea.",
            "Elegir el agente por razón con nombre: `general` para implementar, `explore` para un mapa que exige más de unas cinco consultas, `docs` para documentación, `verify` y `review` para verificación independiente, `tasks-agent` para el tablero.",
            "Usar `background` solo para unidades independientes con archivos distintos, y esperar el aviso antes de integrar.",
            "Implementar en línea solo lo trivial, inequívoco y de bajo riesgo, cuando delegar no aporte valor.",
        ],
        "warnings": [
            "El subagente no toca el tablero ni inventa estados: cada operación de `tasks-*` va a `tasks-agent`.",
            "Revisa la salida del subagente antes de integrarla; un encargo sustancial nace además con alcance y criterios en su tarea.",
        ],
    },
    {
        "name": "Verificar",
        "when": "tras cualquier cambio y antes del commit o del cierre",
        "steps": [
            "Elegir el escalón: trivial → comprobación localizada; funcional → tests afectados y prueba funcional; transversal → suite relevante e integración; crítico → `verify` y, si cambia comportamiento o contrato, `review`.",
            "`verify` es el mínimo tras cualquier cambio no trivial.",
            "Ejecutar la batería del repo (en este paquete, `bash check-package.sh`) o la suite que corresponda al cambio.",
            "Contrastar el criterio de aceptación de la tarea con el resultado real, no con el informe del subagente.",
        ],
        "warnings": [
            "La salida de un subagente no es evidencia: se revisa antes de integrarla.",
            "En un cambio crítico, `review` es obligatoria.",
            "Toda verificación no ejecutada se declara.",
        ],
    },
    {
        "name": "Riesgo",
        "when": "en todo cambio de código, antes de preparar el commit",
        "steps": [
            "Recorrer los seis ítems: 1 datos o efectos irreversibles; 2 seguridad; 3 contratos que otros consumen; 4 concurrencia; 5 entrega o entorno; 6 nada que detecte una regresión en lo que cambia.",
            "Escribir la línea `Risk: item N (motivo)` si toca alguno, o `Risk: none` si no toca ninguno.",
            "Riesgo alto: añadir verificación independiente, además de las comprobaciones propias; otro agente lee el criterio de la tarea y lo prueba contra el resultado real (`verify` y `review`).",
        ],
        "warnings": [
            "La línea viaja en el cuerpo del commit: sin ella, el cambio no está cerrado.",
        ],
    },
    {
        "name": "Commit",
        "when": "cuando el criterio está verificado y el trabajo está listo para integrarse",
        "steps": [
            "Preparar el commit y presentar el informe de preparación: qué entra, qué queda fuera y qué comprobaciones se ejecutaron.",
            "Esperar la confirmación explícita: cubre lo presentado, y un cambio de alcance pide otra.",
            "Redactar en Conventional Commits, en inglés, en imperativo y sin punto final, una unidad coherente por commit.",
            "Cerrar el cuerpo con `Refs: tk-n`, una por tarea, y la línea de riesgo.",
            "Seguir `CONTRIBUTING.md` para rama, integración y versiones; `push`, merge y etiqueta piden autorización.",
        ],
        "warnings": [
            "Sin firmas de IA ni `Co-Authored-By` salvo petición expresa.",
            "El tablero no habla de Git: no escribas estados Git en comentarios del tablero; el estado de Git vive en Git.",
            "Ningún commit con cabos sueltos: lo que aparece al prepararlo se arregla o se pregunta antes.",
        ],
    },
    {
        "name": "Cierre de tarea",
        "when": "tras el commit, al terminar la unidad de trabajo",
        "steps": [
            "Comprobar el criterio de la tarea contra el resultado real.",
            "Dejar la bitácora como comentario en términos del producto: qué se hizo y qué se comprobó, vía `tasks-agent`.",
            "Cerrar con `tasks-ai close <ref> --reason done` solo con la revisión de la persona o su pedido expreso.",
            "Dejar el trabajo a medias: `tasks-ai update <ref> --status open` y un comentario con el siguiente paso.",
            "Épica: cerrarla cuando cierren todas sus hijas; cerrar con hijas abiertas se rechaza sin escribir nada.",
        ],
        "warnings": [
            "La bitácora va en términos del producto, sin estados vivos ni estados de Git: ni hashes, ni ramas, ni «mergeado».",
            "`done` no sustituye la revisión: sin ella, la tarea sigue abierta.",
        ],
    },
    {
        "name": "Memoria y sesión",
        "when": "tras cada decisión, bugfix o hallazgo, y antes de decir «done»",
        "steps": [
            "Guardar con `mem_save` tras una decisión, un bugfix o un hallazgo no obvio, agrupado por tema.",
            "Antes de decir «done» o cerrar la sesión, `mem_session_summary` con objetivo, hallazgos, lo hecho, siguiente paso y archivos relevantes.",
            "Dejar el runner en fase 1 al terminar la sesión; si queda trabajo abierto, en la fase donde queda.",
        ],
        "warnings": [
            "Lo que no se guarda se pierde en la próxima sesión.",
        ],
    },
]

HELP = """\
Guía determinista del flujo del orquestador. La fase la decide este registro, no el agente.

Uso, desde la raíz del proyecto en el que se trabaja:
  ~/.config/opencode/agents/orchestrator-runner.py         muestra la fase actual sin cambiar el estado
  ~/.config/opencode/agents/orchestrator-runner.py next    avanza a la siguiente fase y la muestra
  ~/.config/opencode/agents/orchestrator-runner.py --help  esta ayuda

Estado: .run/orchestrator-runner.json, en el directorio actual, con la forma {"phase": N}.
El directorio actual es el proyecto en el que se trabaja; el estado nunca se escribe junto al
script instalado.
Si falta, la fase inicial es la 1 y la primera invocación crea el fichero. Un fichero
corrupto o inválido vuelve a la fase 1 con un aviso por stderr. Para reiniciar, bórralo.
`next` es la única forma de avanzar; en la fase 11 avisa de que es la última y no avanza.

Fases:
""" + "".join("  %2d  %s\n" % (i, p["name"]) for i, p in enumerate(PHASES, 1))


def load_phase():
    """Devuelve (fase, corrupto). Un estado ausente es la fase 1 sin aviso."""
    if not os.path.exists(STATE):
        return 1, False
    try:
        with open(STATE, encoding="utf-8") as f:
            data = json.load(f)
        phase = data["phase"]
        if type(phase) is int and 1 <= phase <= len(PHASES):
            return phase, False
    except (OSError, ValueError, KeyError, TypeError):
        pass
    return 1, True


def save_phase(phase):
    os.makedirs(os.path.dirname(STATE), exist_ok=True)
    with open(STATE, "w", encoding="utf-8") as f:
        json.dump({"phase": phase}, f)
        f.write("\n")


def render(phase):
    p = PHASES[phase - 1]
    lines = ["Fase %d/%d · %s" % (phase, len(PHASES), p["name"]), "Cuándo: %s." % p["when"], "", "Operativa:"]
    lines += ["  %d. %s" % (i, s) for i, s in enumerate(p["steps"], 1)]
    lines += ["", "Advertencias:"] + ["  - %s" % w for w in p["warnings"]]
    return "\n".join(lines)


def main(argv):
    args = argv[1:]
    if args and args[0] in ("--help", "-h"):
        if len(args) > 1:
            return usage_error(args)
        print(HELP, end="")
        return 0
    if args and args[0] != "next":
        return usage_error(args)
    if len(args) > 1:
        return usage_error(args)

    phase, corrupt = load_phase()
    if corrupt:
        print("aviso: estado inválido en %s; se vuelve a la fase 1" % STATE, file=sys.stderr)
    if args and phase < len(PHASES):
        phase += 1
    try:
        save_phase(phase)
    except OSError as e:
        print("error: no se pudo escribir %s: %s" % (STATE, e), file=sys.stderr)
        return 1

    print(render(phase))
    if args and phase == len(PHASES):
        print("")
        print("Última fase (%d/%d): el runner no avanza más." % (phase, len(PHASES)))
    return 0


def usage_error(args):
    print("error: argumento no reconocido: %s" % " ".join(args), file=sys.stderr)
    print("uso: orchestrator-runner.py [next | --help]", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
