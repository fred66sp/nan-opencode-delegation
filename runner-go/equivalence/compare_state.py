#!/usr/bin/env python3
"""Compara el contrato --json de los dos runners por valor, no por prosa.

Uso: compare_state.py PY.json GO.json [etiqueta] [--declared NOMBRE]
Código 0 si version, los campos contrastados de phases (número, nombre y
evidencia esperada; no steps/warnings) y state coinciden salvo las excepciones
exactas declaradas; 1 y diferencias por stderr.
"""

import json
import sys

# Divergencias de evidencia esperada declaradas: fase -> (texto Python, texto Go),
# en ese orden. Es una excepción exacta: si cualquiera de los dos textos cambia o
# el par llega invertido, el contraste vuelve a fallar. La baseline pinned vive en
# fixtures/pinned y no se toca.
DECLARED_EVIDENCE = {
    1: (
        "qué se leyó (memoria de la sesión, tablero y `AGENTS.md` del proyecto)",
        "el `runner start` leído y el `AGENTS.md` del proyecto",
    ),
    11: (
        "ninguna: es la última fase y no hay transición que justificar; el reinicio lo hace `reset`",
        "ninguna: es la fase terminal y no hay transición que justificar; el cierre del runner lo hace `phase_reset`",
    ),
}

# Excepciones de estado por nombre de declaración: clave -> (valor Python, valor Go).
# `start` registra la sesión en la fase 1 (boot.StoreFor); `boot` no crea base.
DECLARED_STATE = {
    "start": {"exists": (False, True)},
}


def load(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def main(a_path, b_path, label="secuencia", declared=None):
    a, b = load(a_path), load(b_path)
    problems = []
    if a.get("version") != b.get("version"):
        problems.append("version: %r != %r" % (a.get("version"), b.get("version")))
    # Fases: se contrasta la estructura (número, nombres, evidencia esperada),
    # no el texto de pasos/advertencias: la operativa de las fases sigue siendo
    # la misma, pero las instrucciones nombran artefactos distintos (p. ej. la
    # fase 1 ya no cita al runner Python) y esa divergencia es declarada, no un
    # defecto. Ver DESIGN §8, arnés de equivalencia.
    if a.get("phases") != b.get("phases"):
        pa_list, pb_list = a.get("phases", []), b.get("phases", [])
        if len(pa_list) != len(pb_list):
            problems.append("el número de fases difiere")
        else:
            for i, (pa, pb) in enumerate(zip(pa_list, pb_list), 1):
                if pa.get("number") != pb.get("number"):
                    problems.append("phase %d: número distinto" % i)
                if pa.get("name") != pb.get("name"):
                    problems.append("phase %d: nombre distinto (%r != %r)"
                                    % (i, pa.get("name"), pb.get("name")))
                if pa.get("evidence") != pb.get("evidence"):
                    declared_evidence = DECLARED_EVIDENCE.get(pa.get("number"))
                    if (pa.get("evidence"), pb.get("evidence")) != declared_evidence:
                        problems.append("phase %d: evidencia esperada distinta" % i)

    sa, sb = a.get("state", {}), b.get("state", {})
    allowed = DECLARED_STATE.get(declared, {}) if declared else {}
    for key in sorted(set(sa) | set(sb)):
        if sa.get(key) != sb.get(key):
            if allowed.get(key) == (sa.get(key), sb.get(key)):
                continue
            problems.append("state.%s: %r != %r" % (key, sa.get(key), sb.get(key)))

    if problems:
        print("DIFERENCIA (%s):" % label, file=sys.stderr)
        for problem in problems:
            print("  - " + problem, file=sys.stderr)
        return 1
    print("equivalente (%s): phase=%s evidence=%s exists=%s" % (
        label, sa.get("phase"), len(sa.get("evidence", [])), sa.get("exists")))
    return 0


if __name__ == "__main__":
    args = sys.argv[1:]
    declared = None
    if "--declared" in args:
        index = args.index("--declared")
        if index + 1 >= len(args):
            print("uso: compare_state.py PY.json GO.json [etiqueta] [--declared NOMBRE]", file=sys.stderr)
            sys.exit(2)
        declared = args[index + 1]
        del args[index:index + 2]
    if len(args) < 2:
        print("uso: compare_state.py PY.json GO.json [etiqueta] [--declared NOMBRE]", file=sys.stderr)
        sys.exit(2)
    sys.exit(main(args[0], args[1], args[2] if len(args) > 2 else "secuencia", declared))
