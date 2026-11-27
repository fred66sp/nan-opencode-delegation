/**
 * Orchestrator phase — plugin de servidor de OpenCode.
 *
 * Inyecta en el system prompt de cada petición de chat la fase actual del runner
 * del orquestador, su nombre y la evidencia que se espera para avanzar: el modelo
 * deja de olvidar en qué fase está al alargarse la sesión.
 *
 * Solo lectura: ejecuta el binario `orchestrator-runner --json` en el directorio
 * del proyecto. Si no hay estado, el runner falla, excede el tiempo o responde
 * algo que no es el contrato, no se inyecta nada y la petición sigue su curso.
 *
 * Una sola copia sirve a los dos hosts: la factory V1 devuelve los hooks y
 * `setup` registra el equivalente en V2 (mismo reparto que ignore-file.ts).
 *
 * shortcut: el parseo del contrato --json solo tiene cobertura en un humo
 * efímero fuera del repo; versionar el humo en check-package.sh cuando el
 * parseo crezca.
 */

import { execFile } from "node:child_process"
import { existsSync } from "node:fs"
import { homedir } from "node:os"
import { join } from "node:path"
import type { Plugin } from "@opencode-ai/plugin"

/**
 * Binario del runner: `$OPENCODE_RUNNER_BIN`, la copia instalada junto a los
 * agentes o, por último, `orchestrator-runner` resuelto por el PATH.
 */
const RUNNER_INSTALLED = join(homedir(), ".config", "opencode", "agents", "orchestrator-runner")

function runnerBinary(): string {
  const override = process.env.OPENCODE_RUNNER_BIN
  if (override) return override
  if (existsSync(RUNNER_INSTALLED)) return RUNNER_INSTALLED
  return "orchestrator-runner"
}

/** Corto a propósito: el system prompt no espera a un runner colgado. */
const QUERY_TIMEOUT_MS = 3_000

type Evidence = {
  readonly phase: number
  readonly text: string
}

type Phase = {
  readonly number: number
  readonly name: string
  readonly evidence: string
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
}

/** Entero de verdad: descarta `true`, strings y no-números; 6.0 sí es 6 una vez parseado. */
function isPhaseNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value)
}

function parsePhase(value: unknown): Phase | undefined {
  if (!isRecord(value)) return undefined
  const { number, name, evidence } = value
  if (!isPhaseNumber(number) || typeof name !== "string" || typeof evidence !== "string") return undefined
  return { number, name, evidence }
}

function parseEvidence(value: unknown): Evidence | undefined {
  if (!isRecord(value)) return undefined
  const { phase, text } = value
  if (!isPhaseNumber(phase) || typeof text !== "string") return undefined
  return { phase, text }
}

/**
 * Texto de inyección, o undefined si el contrato no permite afirmar la fase.
 * Depende solo de la fase y de la evidencia registrada —nunca del reloj— para no
 * invalidar el caché del prompt entre peticiones sin transición.
 */
function injectionText(raw: string): string | undefined {
  let value: unknown
  try {
    value = JSON.parse(raw)
  } catch {
    return undefined
  }
  if (!isRecord(value) || value.version !== 1) return undefined
  const { phases, state } = value
  if (!Array.isArray(phases) || !isRecord(state)) return undefined
  if (state.exists !== true || state.valid !== true) return undefined

  const phase = state.phase
  const parsedPhases: Phase[] = []
  for (const item of phases) {
    const parsed = parsePhase(item)
    // Contrato estricto: una fase que no parsea invalida todo el documento.
    if (!parsed) return undefined
    parsedPhases.push(parsed)
  }
  if (!isPhaseNumber(phase) || phase < 1 || phase > parsedPhases.length) return undefined
  const current = parsedPhases.find((item) => item.number === phase)
  if (!current) return undefined

  // La evidencia esperada se nombra, nunca viaja como argumento de un comando listo
  // para copiar: un modelo literal registraría la plantilla y avanzaría sin evidencia.
  // El mecanismo se nombra por su operación AIF, no por una ruta que pueda quedar obsoleta.
  const lines = [
    `Orquestador: fase ${phase}/${parsedPhases.length} · ${current.name}. ` +
      `La evidencia que se espera para dejar la fase: ${current.evidence}. ` +
      `Cuando la tengas, regístrala con la operación phase_next del runner por AIF.`,
  ]

  // Historial inválido: la fase sigue siendo cierta, la última evidencia no.
  if (state.evidence_valid === true && Array.isArray(state.evidence)) {
    let last: Evidence | undefined
    for (const item of state.evidence) last = parseEvidence(item) ?? last
    if (last) lines.push(`Última evidencia registrada (fase ${last.phase}): ${last.text}`)
  }
  return lines.join("\n")
}

/** Consulta tolerante: cualquier fallo es ausencia de señal, nunca un error de la petición. */
function queryInjection(directory: string): Promise<string | undefined> {
  return new Promise((resolve) => {
    execFile(runnerBinary(), ["--json"], { cwd: directory, timeout: QUERY_TIMEOUT_MS }, (error, stdout) => {
      resolve(error ? undefined : injectionText(stdout))
    })
  })
}

const OrchestratorPhasePlugin: Plugin = async ({ directory }) => {
  const cwd = directory || process.cwd()
  return {
    // En el último bloque existente: algunos modelos rechazan varios mensajes system.
    "experimental.chat.system.transform": async (_input, output) => {
      const text = await queryInjection(cwd)
      if (!text) return
      if (output.system.length > 0) output.system[output.system.length - 1] += "\n\n" + text
      else output.system.push(text)
    },
  }
}

// ─── Adaptador V2 ────────────────────────────────────────────────────────────
// V2 no consume el objeto de la factory: los hooks se registran con
// `session.hook("context", ...)` y el system prompt llega como partes de texto.
type V2SystemPart = { type: "text"; text: string }
type V2Registration = { dispose: () => Promise<void> | void }
type V2Request = { system: V2SystemPart[] }
type V2Context = {
  location?: { directory?: string }
  session: {
    hook(name: "context", callback: (request: V2Request) => Promise<void> | void): Promise<V2Registration>
  }
}

async function setupOrchestratorPhaseV2(ctx: V2Context): Promise<() => Promise<void>> {
  const cwd = ctx.location?.directory || process.cwd()
  const registration = await ctx.session.hook("context", async (request) => {
    const text = await queryInjection(cwd)
    if (!text) return
    const last = request.system[request.system.length - 1]
    if (last) request.system[request.system.length - 1] = { ...last, text: `${last.text}\n\n${text}` }
    else request.system.push({ type: "text", text })
  })
  return async () => {
    await registration.dispose()
  }
}

// V1 hosts call server(); V2 reads id + setup() and ignores server().
export default { id: "orchestrator-phase", server: OrchestratorPhasePlugin, setup: setupOrchestratorPhaseV2 }
