#!/usr/bin/env python3
"""Compara el contrato --json de los dos runners por valor, no por prosa.

Uso: compare_state.py PY.json GO.json [etiqueta]
Código 0 si version, phases y state son profundamente iguales; 1 y diferencias por stderr.
"""

import json
import sys


def load(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def main(a_path, b_path, label="secuencia"):
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
                    problems.append("phase %d: evidencia esperada distinta" % i)

    sa, sb = a.get("state", {}), b.get("state", {})
    for key in sorted(set(sa) | set(sb)):
        if sa.get(key) != sb.get(key):
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
    if len(sys.argv) < 3:
        print("uso: compare_state.py PY.json GO.json [etiqueta]", file=sys.stderr)
        sys.exit(2)
    sys.exit(main(sys.argv[1], sys.argv[2], sys.argv[3] if len(sys.argv) > 3 else "secuencia"))
