#!/usr/bin/env python3
"""Comprueba el retorno estructurado de un subagente contra el contrato del encargo: el
`status`, una línea de `validation` por comprobación esperada y ni una referencia
indefinida. Sin dependencias: la misma entrada produce la misma salida.
"""

import json
import re
import sys
from difflib import SequenceMatcher
from pathlib import Path

STATUSES = ("completed", "partial", "blocked")

# Tolerantes a la viñeta, a las negritas y a la indentación con la que se escribe el informe.
STATUS_LINE = re.compile(r"(?im)^\s*(?:[-*]\s+)?\**status\**\s*:\s*(.*)$")
QUESTION_LINE = re.compile(r"(?im)^\s*(?:[-*]\s+)?\**(?:pregunta|question)\**\s*:")

# Una mención anafórica («el fichero», «esa línea», «el paso») solo se sostiene si la misma
# línea trae la ruta, el fichero o la referencia `path:line` que la resuelve.
ANAPHORA = re.compile(
    r"(?i)\b(?:el|la|ese|esa|este|esta|dicho|dicha|dichos|dichas)\s+"
    r"(?:fichero|archivo|paso|l[ií]nea|funci[oó]n|m[oó]dulo|tabla|secci[oó]n|comando|script|campo)s?\b"
)
PATH_LIKE = re.compile(
    r"(?:[\w.-]+/[\w./-]+|[\w.-]+\.(?:py|sh|md|json|jsonl|jsonc|tsx?|txt|ya?ml|toml|cfg|ini)"
    r"|\b[\w.-]+:\d+\b)"
)

HELP = """\
Comprueba el retorno estructurado de un subagente con las reglas del orquestador, sin
juicio del modelo: dice lo que falta en el informe, no si el trabajo está bien hecho.

Uso, desde la raíz del proyecto en el que se trabaja:
  ~/.config/opencode/agents/delegation-report-checker.py --checks COMPROBACIONES [INFORME]
  ~/.config/opencode/agents/delegation-report-checker.py --json --checks COMPROBACIONES [INFORME]
  ~/.config/opencode/agents/delegation-report-checker.py --help

INFORME es el fichero con el informe del subagente; sin él, o con `-`, se lee la entrada
estándar. COMPROBACIONES es un fichero con una comprobación esperada por línea —el comando
tal como se pidió en el encargo—; las líneas vacías y las que empiezan por `#` se ignoran.

Qué exige, en este orden:
  1. Una línea `status: completed`, `status: partial` o `status: blocked`. Sin ella, o con
     otro valor, el informe se rechaza.
  2. Ninguna línea `pregunta:` ni `question:`: un bloqueo de decisión no se devuelve como
     estado, se devuelve la pregunta al humano y el orquestador la eleva.
  3. Una sección `validation` (encabezado suelto `validation`, `validation:` o
     `## validation`) con una línea `<comando>: <resultado observado>` por comprobación
     esperada. El comando se compara con el esperado tras normalizar espacios y `./`: vale
     el mismo comando, el esperado como prefijo del observado y la variación trivial (ratio
     de similitud de 0.9). La línea sin resultado observado también incumple.
  4. Ninguna referencia indefinida: una mención de «el fichero», «ese paso» o «esa línea»
     sin la ruta, el fichero o la referencia `path:line` en la misma línea. Es una heurística
     de forma, no una lectura del contenido: puede marcar prosa legítima.

Con `--json` imprime por stdout un único objeto con `ok` (veredicto), `status` (el valor
leído, o null), `checks` (cada comprobación esperada con el resultado observado, o null si
no apareció) y `violations` (los incumplimientos en texto). Sin `--json`, el veredicto y los
incumplimientos se imprimen como líneas.

Códigos de salida: 0 conforme; 1 no conforme, con la lista de incumplimientos; 2 error de
uso —falta `--checks`, el fichero no se lee, un argumento desconocido—. El script no edita
el informe ni escribe nada en disco.
"""

USAGE = "uso: delegation-report-checker.py [--json] --checks FICHERO [INFORME]"


class UsageError(Exception):
    """Error de uso: el mensaje va a stderr y el código de salida es 2."""


def read_text(path):
    try:
        return Path(path).read_text(encoding="utf-8")
    except OSError as e:
        raise UsageError("no se pudo leer %s: %s" % (path, e))


def read_checks(path):
    checks = [
        line.strip()
        for line in read_text(path).splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]
    if not checks:
        raise UsageError("%s no declara ninguna comprobación esperada" % path)
    return checks


def normalize(command):
    """Unifica espacios y el prefijo `./` de cada token: son variaciones triviales."""
    tokens = command.strip().strip("`").strip().split()
    return " ".join(t[2:] if t.startswith("./") else t for t in tokens)


def matches(expected, observed):
    expected, observed = normalize(expected), normalize(observed)
    return (
        observed == expected
        or observed.startswith(expected)
        or SequenceMatcher(None, expected, observed).ratio() >= 0.9
    )


def validation_section(text):
    """Devuelve (hay_encabezado, [(comando, resultado)]) de las líneas del informe."""
    heading = False
    lines = []
    for raw in text.splitlines():
        line = raw.strip()
        if not line:
            continue
        head = line.lstrip("#*-").strip()
        if head.lower() == "validation" or head.lower().startswith("validation:"):
            heading = True
            if ":" not in head:
                continue
            line = head.split(":", 1)[1].strip()
            if not line:
                continue
        if ":" not in line:
            continue
        command, result = line.split(":", 1)
        lines.append((command.strip().strip("`*").strip(), result.strip()))
    return heading, lines


def check(text, checks):
    """Devuelve (status, [comprobaciones], [incumplimientos])."""
    violations = []

    values = [m.group(1).strip().strip("*`").strip(".,;").lower() for m in STATUS_LINE.finditer(text)]
    if not values:
        violations.append("falta la línea `status: completed|partial|blocked`")
    for value in values:
        if value not in STATUSES:
            violations.append(
                "status fuera del conjunto: «%s»; se espera completed, partial o blocked" % value
            )
    if QUESTION_LINE.search(text):
        violations.append(
            "el informe declara una `pregunta:`: un bloqueo de decisión se devuelve como pregunta al humano, no como estado"
        )

    heading, lines = validation_section(text)
    if not heading:
        violations.append("falta la sección `validation` con una línea por comprobación esperada")

    reported = []
    for expected in checks:
        observed = next((r for c, r in lines if c and matches(expected, c)), None)
        reported.append({"expected": expected, "observed": observed})
        if observed is None:
            violations.append("validation no cita la comprobación esperada: %s" % expected)
        elif not observed:
            violations.append("validation cita el comando sin resultado observado: %s" % expected)

    for line in text.splitlines():
        mention = ANAPHORA.search(line)
        if mention and not PATH_LIKE.search(line):
            violations.append("referencia indefinida sin ruta en la misma línea: «%s»" % mention.group(0))

    status = values[0] if values else None
    return status, reported, violations


def render(status, reported, violations):
    if not violations:
        missing_ok = sum(1 for c in reported if c["observed"])
        return "conforme: status %s y %d comprobación(es) con resultado observado" % (status, missing_ok)
    lines = ["no conforme: %d incumplimiento(s)" % len(violations)]
    lines += ["  - %s" % v for v in violations]
    return "\n".join(lines)


def main(argv):
    args = list(argv[1:])
    json_mode = False
    checks_path = None
    report_path = None
    while args:
        arg = args.pop(0)
        if arg in ("--help", "-h"):
            print(HELP, end="")
            return 0
        if arg == "--json":
            json_mode = True
        elif arg == "--checks":
            if not args:
                raise UsageError("--checks necesita el fichero de comprobaciones esperadas")
            checks_path = args.pop(0)
        elif arg.startswith("-") and arg != "-":
            raise UsageError("argumento no reconocido: %s" % arg)
        elif report_path is not None:
            raise UsageError("solo se admite un informe: ya se indicó %s" % report_path)
        else:
            report_path = arg
    if checks_path is None:
        raise UsageError("falta --checks con el fichero de comprobaciones esperadas")

    checks = read_checks(checks_path)
    text = sys.stdin.read() if report_path in (None, "-") else read_text(report_path)
    status, reported, violations = check(text, checks)

    if json_mode:
        print(json.dumps(
            {"ok": not violations, "status": status, "checks": reported, "violations": violations},
            ensure_ascii=False,
            indent=2,
        ))
    else:
        print(render(status, reported, violations))
    return 1 if violations else 0


if __name__ == "__main__":
    try:
        sys.exit(main(sys.argv))
    except UsageError as e:
        print("error: %s" % e, file=sys.stderr)
        print(USAGE, file=sys.stderr)
        sys.exit(2)
