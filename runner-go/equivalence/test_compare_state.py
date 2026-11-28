#!/usr/bin/env python3
"""Pruebas dirigidas de compare_state.py: las excepciones son exactas.

Uso: python3 test_compare_state.py  (desde este directorio)
"""

import contextlib
import io
import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import compare_state  # noqa: E402


def phase(number, evidence, name=None):
    return {"number": number, "name": name or "fase %d" % number, "evidence": evidence}


def document(phases, exists=False, phase_number=1):
    return {
        "version": 1,
        "phases": phases,
        "state": {
            "exists": exists,
            "phase": phase_number,
            "valid": True,
            "evidence_valid": True,
            "evidence": [],
        },
    }


class CompareStateTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)

    def compare(self, a, b, declared=None):
        paths = []
        for name, doc in (("a", a), ("b", b)):
            path = os.path.join(self.tmp.name, name + ".json")
            with open(path, "w", encoding="utf-8") as handle:
                json.dump(doc, handle)
            paths.append(path)
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            return compare_state.main(paths[0], paths[1], "test", declared)

    def baseline(self):
        declared = compare_state.DECLARED_EVIDENCE
        phases = [phase(n, "evidencia %d" % n) for n in range(1, 12)]
        phases[0] = phase(1, declared[1][0])
        phases[10] = phase(11, declared[11][0])
        return document(phases)

    def go_baseline(self):
        declared = compare_state.DECLARED_EVIDENCE
        doc = self.baseline()
        doc["phases"][0] = phase(1, declared[1][1])
        doc["phases"][10] = phase(11, declared[11][1])
        return doc

    def test_declared_evidence_accepted(self):
        self.assertEqual(self.compare(self.baseline(), self.go_baseline()), 0)

    def test_declared_evidence_inverted_rejected(self):
        self.assertEqual(self.compare(self.go_baseline(), self.baseline()), 1)

    def test_undeclared_phase_evidence_rejected(self):
        other = self.go_baseline()
        other["phases"][1] = phase(2, "otra evidencia")
        self.assertEqual(self.compare(self.baseline(), other), 1)

    def test_declared_phase_changed_text_rejected(self):
        other = self.go_baseline()
        other["phases"][0] = phase(1, "texto nuevo del Go")
        self.assertEqual(self.compare(self.baseline(), other), 1)

    def test_declared_phase_wrong_number_rejected(self):
        other = self.go_baseline()
        other["phases"][0] = phase(2, compare_state.DECLARED_EVIDENCE[1][1])
        self.assertEqual(self.compare(self.baseline(), other), 1)

    def test_declared_state_start_accepted(self):
        self.assertEqual(self.compare(self.baseline(), self.go_baseline(), "start"), 0)

    def test_declared_state_wrong_direction_rejected(self):
        b = self.go_baseline()
        b["state"]["exists"] = False
        a = self.baseline()
        a["state"]["exists"] = True
        self.assertEqual(self.compare(a, b, "start"), 1)

    def test_declared_state_other_key_rejected(self):
        b = self.go_baseline()
        b["state"]["phase"] = 2
        self.assertEqual(self.compare(self.baseline(), b, "start"), 1)

    def test_exists_diff_rejected_without_declaration(self):
        b = self.go_baseline()
        b["state"]["exists"] = True
        self.assertEqual(self.compare(self.baseline(), b), 1)

    def test_version_name_and_count_rejected(self):
        other = self.go_baseline()
        other["version"] = 2
        self.assertEqual(self.compare(self.baseline(), other), 1)
        other = self.go_baseline()
        other["phases"][1]["name"] = "otro nombre"
        self.assertEqual(self.compare(self.baseline(), other), 1)
        other = self.go_baseline()
        other["phases"].pop()
        self.assertEqual(self.compare(self.baseline(), other), 1)


if __name__ == "__main__":
    unittest.main()
