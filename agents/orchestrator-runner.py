#!/usr/bin/env python3
"""Guía determinista del flujo del orquestador: la fase actual, su lista operativa, sus
advertencias y la evidencia que hace falta para dejarla. Sin dependencias y sin reloj: la
misma entrada produce la misma salida.
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
        "evidence": "qué se leyó (memoria de la sesión, tablero y `AGENTS.md` del proyecto)",
        "steps": [
            "Recuperar el contexto con `mem_context`; `mem_search` si la petición toca algo ya trabajado.",
            "Leer el `AGENTS.md` del proyecto y su mapa de fuentes antes de decidir nada.",
            "Consultar el tablero (en curso y listas para empezar) con la CLI `tasks-ai` del proyecto, directamente: las lecturas del tablero no se delegan.",
            "Comprobar la fase con `~/.config/opencode/agents/orchestrator-runner.py`; si la sesión anterior dejó otra y empiezas trabajo nuevo, ejecuta su subcomando `reset` para volver a la fase 1. El fichero `.run/orchestrator-runner.json` no se borra a mano: el agente no puede tocarlo.",
        ],
        "warnings": [
            "Nada se lanza antes de leer la memoria de la sesión y el tablero.",
            "El runner no comprueba el mundo real: informa antes de que falles; las guardas del tablero y del producto siguen siendo las que bloquean.",
        ],
    },
    {
        "name": "Clasificar el encargo",
        "when": "en cuanto llega la petición, antes de cualquier escritura",
        "evidence": "la clasificación del encargo y, si es planificación, el acuerdo explícito de la persona",
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
        "evidence": "la referencia de la tarea (`tk-n`) o «pequeño sin tarea, con autorización de la persona»",
        "steps": [
            "Aplicar el test de la sesión interrumpida: si la sesión se parara ahora, ¿alguien podría terminar el trabajo con la petición original y `git diff`?",
            "Respuesta sí: trabajo pequeño. No se crea tarea nueva, salvo que la persona la quiera.",
            "Respuesta no: trabajo grande. Lanza a `tasks-agent` en background un único encargo con la secuencia ordenada de registro: una épica (`epic` + `ODD` + tipo), sus subtareas (`task` + `ODD` + tipo), el criterio observable de cada una y la petición original en un comentario. Continúa con tu proceso y, antes de la primera edición, comprueba por CLI que la épica quedó con su criterio.",
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
        "evidence": "las fuentes exploradas: código, requisitos y lo que el cambio debe alcanzar",
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
        "evidence": "la decisión tomada o «sin decisiones del humano»",
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
        "evidence": "los subagentes lanzados y la tarea de cada uno",
        "steps": [
            "Todo encargo lleva cuatro cosas: objetivo, archivos de su propiedad, comprobaciones esperadas y criterio de aceptación; si hay tablero, la referencia de la tarea.",
            "En el encargo, distinguir obstáculo corriente —error, timeout, herramienta que falla—, que se diagnostica y se supera, de bloqueo deliberado —fichero marcado para no tocar, acceso retirado a propósito—, que se respeta y se comunica buscando otra vía segura; el informe de retorno se entiende sin haber visto el trabajo.",
            "Elegir el agente por razón con nombre: `general` para implementar, `explore` para un mapa que exige más de unas cinco consultas, `docs` para documentación, `verify` y `review` para verificación independiente, `tasks-agent` para escribir en el tablero; sus lecturas van directas por la CLI `tasks-ai`.",
            "Usar `background` solo para unidades independientes con archivos distintos, y esperar el aviso antes de integrar.",
            "Implementar en línea solo lo trivial, inequívoco y de bajo riesgo, cuando delegar no aporte valor.",
        ],
        "warnings": [
            "El subagente no toca el tablero ni inventa estados: cada escritura de `tasks-*` va a `tasks-agent`; una escritura lanzada no es evidencia, se comprueba por CLI antes de usarse.",
            "Revisa la salida del subagente antes de integrarla; un encargo sustancial nace además con alcance y criterios en su tarea.",
        ],
    },
    {
        "name": "Verificar",
        "when": "tras cualquier cambio y antes del commit o del cierre",
        "evidence": "el escalón de verificación aplicado y su resultado",
        "steps": [
            "Elegir el escalón: trivial → comprobación localizada; funcional → tests afectados y prueba funcional; transversal → suite relevante e integración; crítico → `verify` y, si cambia comportamiento o contrato, `review`.",
            "`verify` es el mínimo tras cualquier cambio no trivial.",
            "Ejecutar la batería del repo (en este paquete, `bash check-package.sh`) o la suite que corresponda al cambio.",
            "Contrastar el criterio de aceptación de la tarea con el resultado real, no con el informe del subagente: el diff y el resumen se contrastan los dos, porque el resumen tiende a omitir cambios pequeños.",
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
        "evidence": "la línea `Risk:` escrita",
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
        "evidence": "la revisión o confirmación de la persona recibida",
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
        "evidence": "la tarea cerrada o el siguiente paso declarado",
        "steps": [
            "Comprobar el criterio de la tarea contra el resultado real.",
            "Dejar la bitácora como comentario en términos del producto: qué se hizo y qué se comprobó, vía `tasks-agent` en primer plano.",
            "Cerrar con `tasks-agent` en primer plano (`tasks-ai close <ref> --reason done`) solo con la revisión de la persona o su pedido expreso.",
            "Dejar el trabajo a medias: lanza a `tasks-agent` en background el paso a `open` y un comentario con el siguiente paso, y comprueba por CLI que quedó registrado.",
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
        "evidence": "ninguna: es la última fase y no hay transición que justificar; el reinicio lo hace `reset`",
        "steps": [
            "Guardar con `mem_save` tras una decisión, un bugfix o un hallazgo no obvio, agrupado por tema.",
            "Antes de decir «done» o cerrar la sesión, `mem_session_summary` con objetivo, hallazgos, lo hecho, siguiente paso y archivos relevantes.",
            "Antes de dar la sesión por cerrada, `reset`: una vez cerrada la sesión el estado debe quedar en fase 1 y sin historial, nunca en la fase 11; si queda trabajo abierto, la próxima sesión lo recoge desde su tarea.",
        ],
        "warnings": [
            "Lo que no se guarda se pierde en la próxima sesión.",
        ],
    },
]

HELP = """\
Guía determinista del flujo del orquestador. La fase la decide este registro, no el agente.

Uso, desde la raíz del proyecto en el que se trabaja:
  ~/.config/opencode/agents/orchestrator-runner.py                muestra la fase actual sin cambiar el estado
  ~/.config/opencode/agents/orchestrator-runner.py next           no avanza: muestra la fase y la evidencia que se espera para dejarla
  ~/.config/opencode/agents/orchestrator-runner.py next "texto"   registra el texto como evidencia, avanza y muestra la nueva fase
  ~/.config/opencode/agents/orchestrator-runner.py reset          borra el estado: vuelve a la fase 1 sin historial
  ~/.config/opencode/agents/orchestrator-runner.py --json         imprime las fases y el estado en JSON, sin cambiar nada
  ~/.config/opencode/agents/orchestrator-runner.py --help         esta ayuda

`next` exige evidencia: sin texto no avanza (código 0, es una consulta) y pide la evidencia de
la fase actual. Un único argumento de texto; varios puntos van en el mismo texto separados por
`;`. Texto vacío o solo espacios, texto que empieza por `--`, o más de un argumento, son error
de uso (código 1): así `next --json` no avanza por accidente. En la fase 11 no hay transición:
`next`, con o sin evidencia, avisa de que es la última y no avanza.

`--json` se acepta solo y sin otros argumentos: imprime por stdout un único objeto JSON con
`version` (1), `phases` (las 11 fases en orden, con número, nombre, cuándo, evidencia esperada,
pasos y advertencias) y `state` (si existe el fichero de estado con `exists`, fase actual, si el
estado es válido, si el historial lo es y la evidencia registrada). No crea ni modifica el
estado y no emite avisos: su stdout es para un programa, no para una persona. Estado corrupto:
fase 1 con `valid` en false y `exists` en true; historial que no es una lista: `evidence_valid`
en false con la evidencia descartada y la fase conservada. Sin fichero de estado: `exists` en
false, fase 1 y `valid` en true.

Estado: .run/orchestrator-runner.json, en el directorio actual, con la forma
{"phase": N, "evidence": [{"phase": N, "text": "..."}]}, donde cada entrada es la evidencia con
la que se dejó esa fase. Un estado viejo con solo {"phase": N} se acepta como fase válida y
historial vacío. El directorio actual es el proyecto en el que se trabaja; el estado nunca se
escribe junto al script instalado. Si falta, la fase inicial es la 1 y la primera invocación
crea el fichero. Un fichero corrupto o con la fase fuera de rango vuelve a la fase 1 con un
aviso por stderr; una evidencia que no es una lista se descarta con aviso y la fase se conserva.
Para reiniciar, `reset`: borra el fichero completo —fase e historial— sin tocar nada más.

Códigos de salida: 0 consulta o avance correctos; 1 error de uso o no se pudo escribir el estado.

Fases:
""" + "".join("  %2d  %s\n" % (i, p["name"]) for i, p in enumerate(PHASES, 1))

USAGE = 'uso: orchestrator-runner.py [next ["texto de evidencia"] | reset | --json | --help]'


def load_state():
    """Devuelve (fase, evidencias, corrupto, evidencia_invalida). Estado ausente: fase 1 sin aviso."""
    if not os.path.exists(STATE):
        return 1, [], False, False
    try:
        with open(STATE, encoding="utf-8") as f:
            data = json.load(f)
        phase = data["phase"]
        if not (type(phase) is int and 1 <= phase <= len(PHASES)):
            return 1, [], True, False
        evidence = data.get("evidence", [])
        if not isinstance(evidence, list):
            return phase, [], False, True
        return phase, evidence, False, False
    except (OSError, ValueError, KeyError, TypeError):
        return 1, [], True, False


def save_state(phase, evidence):
    os.makedirs(os.path.dirname(STATE), exist_ok=True)
    with open(STATE, "w", encoding="utf-8") as f:
        json.dump({"phase": phase, "evidence": evidence}, f)
        f.write("\n")


def render(phase):
    p = PHASES[phase - 1]
    lines = ["Fase %d/%d · %s" % (phase, len(PHASES), p["name"]), "Cuándo: %s." % p["when"], "", "Operativa:"]
    lines += ["  %d. %s" % (i, s) for i, s in enumerate(p["steps"], 1)]
    lines += ["", "Advertencias:"] + ["  - %s" % w for w in p["warnings"]]
    return "\n".join(lines)


def render_evidence(phase):
    p = PHASES[phase - 1]
    return "\n".join([
        "",
        "Evidencia para dejar la fase %d/%d · %s:" % (phase, len(PHASES), p["name"]),
        "  Se espera que declares %s." % p["evidence"],
        "  Sin ella el runner no avanza: sigues en la fase %d." % phase,
        '  Para avanzar: orchestrator-runner.py next "texto de evidencia"',
        "  Varios puntos van en el mismo texto, separados por `;`.",
    ])


def render_json():
    """Devuelve el flujo y el estado como JSON, con la misma lectura que una consulta."""
    phase, evidence, corrupt, bad_evidence = load_state()
    return json.dumps({
        "version": 1,
        "phases": [dict(number=i, **p) for i, p in enumerate(PHASES, 1)],
        "state": {
            "exists": os.path.exists(STATE),
            "phase": phase,
            "valid": not corrupt,
            "evidence_valid": not bad_evidence,
            "evidence": evidence,
        },
    }, ensure_ascii=False, indent=2)


def main(argv):
    args = argv[1:]
    if args and args[0] == "--json":
        if len(args) > 1:
            return usage_error(args)
        print(render_json())
        return 0
    if args and args[0] in ("--help", "-h"):
        if len(args) > 1:
            return usage_error(args)
        print(HELP, end="")
        return 0
    if args and args[0] == "reset":
        if len(args) > 1:
            return usage_error(args)
        if os.path.exists(STATE):
            try:
                os.remove(STATE)
            except OSError as e:
                print("error: no se pudo borrar %s: %s" % (STATE, e), file=sys.stderr)
                return 1
            print("estado borrado: la próxima invocación empieza en la fase 1 sin historial")
        else:
            print("sin estado: sigues en la fase 1")
        return 0
    if args and args[0] != "next":
        return usage_error(args)
    if len(args) > 2:
        return usage_error(args)
    text = args[1] if len(args) == 2 else None
    if text is not None and not text.strip():
        print("error: la evidencia no puede estar vacía ni ser solo espacios", file=sys.stderr)
        print(USAGE, file=sys.stderr)
        return 1
    if text is not None and text.startswith("--"):
        print("error: la evidencia no puede empezar por --", file=sys.stderr)
        print(USAGE, file=sys.stderr)
        return 1

    phase, evidence, corrupt, bad_evidence = load_state()
    if corrupt:
        print("aviso: estado inválido en %s; se vuelve a la fase 1" % STATE, file=sys.stderr)
    if bad_evidence:
        print("aviso: historial de evidencia inválido en %s; se descarta" % STATE, file=sys.stderr)

    # La fase 11 no tiene transición: solo informa, con evidencia o sin ella.
    advances = text is not None and phase < len(PHASES)
    if advances:
        evidence.append({"phase": phase, "text": text})
        phase += 1
    if not args or advances:
        try:
            save_state(phase, evidence)
        except OSError as e:
            print("error: no se pudo escribir %s: %s" % (STATE, e), file=sys.stderr)
            return 1

    print(render(phase))
    if phase == len(PHASES):
        print("")
        print("Última fase (%d/%d): el runner no avanza más." % (phase, len(PHASES)))
    elif text is None:
        print(render_evidence(phase))
    return 0


def usage_error(args):
    print("error: argumento no reconocido: %s" % " ".join(args), file=sys.stderr)
    print(USAGE, file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
