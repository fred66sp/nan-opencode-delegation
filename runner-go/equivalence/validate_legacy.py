#!/usr/bin/env python3
"""Validador mecánico del ledger legado, copia literal del que ejecuta
check-package.sh (R11): seis campos, phase entero y status en conjunto.

Uso: validate_legacy.py .run/delegations.jsonl
"""

import json
import sys

STATUSES = {"launched", "completed", "blocked"}
FIELDS = ("ts", "phase", "task", "fingerprint", "agent", "status")


def main(path):
    lines = [line for line in open(path, encoding="utf-8").read().splitlines() if line.strip()]
    assert lines, "el ledger está vacío"
    for n, line in enumerate(lines, 1):
        doc = json.loads(line)
        assert isinstance(doc, dict), "la línea %d no es un objeto JSON" % n
        missing = [field for field in FIELDS if field not in doc]
        assert not missing, "a la línea %d le faltan %s" % (n, missing)
        assert isinstance(doc["ts"], str) and doc["ts"].strip(), "ts vacío en la línea %d" % n
        assert isinstance(doc["phase"], int) and not isinstance(doc["phase"], bool), \
            "phase no es entero en la línea %d" % n
        assert doc["task"] is None or (isinstance(doc["task"], str) and doc["task"].strip()), \
            "task no es referencia o null en la línea %d" % n
        assert isinstance(doc["fingerprint"], str) and doc["fingerprint"].strip(), \
            "fingerprint vacío en la línea %d" % n
        assert isinstance(doc["agent"], str) and doc["agent"].strip(), \
            "agent vacío en la línea %d" % n
        assert doc["status"] in STATUSES, "status fuera del conjunto en la línea %d" % n
    return len(lines)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("uso: validate_legacy.py FICHERO", file=sys.stderr)
        sys.exit(2)
    try:
        count = main(sys.argv[1])
    except (AssertionError, ValueError) as error:
        print("ledger inválido: %s" % error, file=sys.stderr)
        sys.exit(1)
    print("ledger válido: %d línea(s)" % count)
