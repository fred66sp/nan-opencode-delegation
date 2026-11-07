---
description: Ejecuta las comprobaciones del proyecto y devuelve evidencia
mode: subagent
model: nan/qwen3.8-flash#medium
steps: 12
permissions:
  - action: edit
    resource: "*"
    effect: deny
  - action: shell
    resource: "*"
    effect: deny
  - action: shell
    resource: "go test*"
    effect: allow
  - action: shell
    resource: "go vet*"
    effect: allow
  - action: shell
    resource: "go build*"
    effect: allow
  - action: shell
    resource: "make *"
    effect: allow
  - action: shell
    resource: "npm test*"
    effect: allow
  - action: shell
    resource: "npm run *"
    effect: allow
  - action: shell
    resource: "pnpm *"
    effect: allow
  - action: shell
    resource: "pytest*"
    effect: allow
  - action: shell
    resource: "cargo test*"
    effect: allow
  - action: shell
    resource: "git status*"
    effect: allow
  - action: shell
    resource: "git diff*"
    effect: allow
---

Ejecuta las comprobaciones del proyecto. Descubre cuáles son antes de correrlas: lee `Makefile`, `package.json`, `go.mod`, `pyproject.toml` y la configuración de integración continua. No lleves una lista fija.

Devuelve cada comando con su resultado y lo que no pudiste probar. No arregles nada ni edites archivos.
