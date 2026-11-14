/** @jsxImportSource @opentui/solid */
import { execFile } from "node:child_process"
import { homedir } from "node:os"
import { join } from "node:path"
import { For, Show, createSignal, onCleanup } from "solid-js"
import { Plugin, usePlugin } from "@opencode/plugin/tui"
import type { RGBA } from "@opentui/core"

/** Fichero del runner del orquestador que produce el contrato --json. */
const RUNNER_SCRIPT = join(homedir(), ".config", "opencode", "agents", "orchestrator-runner.py")
const QUERY_TIMEOUT_MS = 10_000

const RUNNER_ERROR = "No se pudo consultar el runner. Comprueba que python3 y ~/.config/opencode/agents/orchestrator-runner.py estén disponibles."
const MALFORMED_ERROR = "El runner respondió algo que no es el JSON esperado."

type Phase = {
  readonly number: number
  readonly name: string
  readonly when: string
  readonly evidence: string
  readonly steps: readonly string[]
  readonly warnings: readonly string[]
}

type Evidence = {
  readonly phase: number
  readonly text: string
}

type RunnerState = {
  readonly exists: boolean
  readonly phase: number
  readonly valid: boolean
  readonly evidence_valid: boolean
  readonly evidence: readonly Evidence[]
}

type RunnerData = {
  readonly version: number
  readonly phases: readonly Phase[]
  readonly state: RunnerState
}

type Snapshot =
  | { readonly kind: "ok"; readonly data: RunnerData; readonly at: number }
  | { readonly kind: "error"; readonly message: string; readonly at: number }

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string")
}

function parsePhase(value: unknown): Phase | undefined {
  if (!isRecord(value)) return undefined
  const { number, name, when, evidence, steps, warnings } = value
  if (typeof number !== "number" || typeof name !== "string" || typeof when !== "string" || typeof evidence !== "string") {
    return undefined
  }
  if (!isStringArray(steps) || !isStringArray(warnings)) return undefined
  return { number, name, when, evidence, steps, warnings }
}

function parseEvidence(value: unknown): Evidence | undefined {
  if (!isRecord(value)) return undefined
  const { phase, text } = value
  if (typeof phase !== "number" || typeof text !== "string") return undefined
  return { phase, text }
}

/** Frontera del contrato: JSON ajeno se valida antes de tocar la TUI. */
function parseRunnerData(raw: string): RunnerData | undefined {
  let value: unknown
  try {
    value = JSON.parse(raw)
  } catch {
    return undefined
  }
  if (!isRecord(value)) return undefined
  const { version, phases, state } = value
  if (version !== 1 || !Array.isArray(phases) || !isRecord(state)) return undefined

  const parsedPhases: Phase[] = []
  for (const item of phases) {
    const phase = parsePhase(item)
    if (!phase) return undefined
    parsedPhases.push(phase)
  }

  const { exists, phase, valid, evidence_valid, evidence } = state
  if (
    typeof exists !== "boolean" ||
    typeof phase !== "number" ||
    typeof valid !== "boolean" ||
    typeof evidence_valid !== "boolean" ||
    !Array.isArray(evidence)
  ) {
    return undefined
  }
  const parsedEvidence: Evidence[] = []
  for (const item of evidence) {
    const entry = parseEvidence(item)
    if (!entry) return undefined
    parsedEvidence.push(entry)
  }
  if (parsedPhases.length === 0) return undefined

  return { version, phases: parsedPhases, state: { exists, phase, valid, evidence_valid, evidence: parsedEvidence } }
}

/** Consulta el runner sin escribir nada; cualquier fallo se convierte en un mensaje estable. */
function queryRunner(cwd: string): Promise<Snapshot> {
  return new Promise((resolve) => {
    execFile("python3", [RUNNER_SCRIPT, "--json"], { cwd, timeout: QUERY_TIMEOUT_MS }, (error, stdout) => {
      const at = Date.now()
      if (error) {
        resolve({ kind: "error", message: RUNNER_ERROR, at })
        return
      }
      const data = parseRunnerData(stdout)
      resolve(data ? { kind: "ok", data, at } : { kind: "error", message: MALFORMED_ERROR, at })
    })
  })
}

function wrap(text: string, width: number): string[] {
  const max = Math.max(width, 1)
  const lines: string[] = []
  let line = ""
  for (const word of text.split(/\s+/).filter(Boolean)) {
    const candidate = line === "" ? word : `${line} ${word}`
    if (candidate.length <= max) {
      line = candidate
      continue
    }
    if (line !== "") lines.push(line)
    let rest = word
    while (rest.length > max) {
      lines.push(rest.slice(0, max))
      rest = rest.slice(max)
    }
    line = rest
  }
  if (line !== "") lines.push(line)
  return lines.length > 0 ? lines : [""]
}

/** Envuelve con prefijo y conserva la sangría en las líneas de continuación. */
function wrapLines(prefix: string, text: string, width: number): string[] {
  const lines = wrap(text, Math.max(width - prefix.length, 1))
  const pad = " ".repeat(prefix.length)
  return lines.map((line, index) => (index === 0 ? `${prefix}${line}` : `${pad}${line}`))
}

function clip(text: string, width: number): string {
  const max = Math.max(width, 1)
  return text.length <= max ? text : `${text.slice(0, Math.max(max - 1, 0))}…`
}

function formatClock(at: number): string {
  return new Date(at).toLocaleTimeString("es-ES", { hour12: false })
}

function Wrapped(props: { readonly text: string; readonly width: number; readonly fg: RGBA; readonly prefix?: string }) {
  return (
    <For each={wrapLines(props.prefix ?? "", props.text, props.width)}>
      {(line) => <text fg={props.fg}>{line}</text>}
    </For>
  )
}

function StatusBody(props: { readonly data: RunnerData; readonly width: number }) {
  const context = usePlugin()
  const state = () => props.data.state
  const phase = () => props.data.phases.find((item) => item.number === state().phase)
  const total = () => props.data.phases.length

  return (
    <>
      <text fg={context.theme.text.base}>
        {clip(`Fase ${state().phase}/${total()} · ${phase()?.name ?? "fase desconocida"}`, props.width)}
      </text>
      <Show when={phase()}>
        {(current) => <Wrapped prefix="Cuándo: " text={current().when} width={props.width} fg={context.theme.text.muted} />}
      </Show>

      <text fg={context.theme.text.base}> </text>
      <text fg={context.theme.text.base}>Operativa</text>
      <Show
        when={(phase()?.steps.length ?? 0) > 0}
        fallback={<text fg={context.theme.text.faint}>Sin pasos registrados.</text>}
      >
        <For each={phase()?.steps ?? []}>
          {(step, index) => <Wrapped prefix={`${index() + 1}. `} text={step} width={props.width} fg={context.theme.text.muted} />}
        </For>
      </Show>

      <text fg={context.theme.text.base}> </text>
      <text fg={context.theme.text.base}>Advertencias</text>
      <Show
        when={(phase()?.warnings.length ?? 0) > 0}
        fallback={<text fg={context.theme.text.faint}>Ninguna.</text>}
      >
        <For each={phase()?.warnings ?? []}>
          {(warning) => (
            <Wrapped prefix="• " text={warning} width={props.width} fg={context.theme.text.feedback.warning.muted} />
          )}
        </For>
      </Show>

      <text fg={context.theme.text.base}> </text>
      <text fg={context.theme.text.base}>Historial de evidencias</text>
      <Show
        when={state().evidence.length > 0}
        fallback={<text fg={context.theme.text.faint}>Sin evidencias registradas.</text>}
      >
        <For each={state().evidence}>
          {(entry) => <Wrapped prefix={`Fase ${entry.phase} · `} text={entry.text} width={props.width} fg={context.theme.text.muted} />}
        </For>
      </Show>
    </>
  )
}

/** Dimensiones del renderer, si el host las expone; el modal se mide con ellas. */
function rendererDimensions(renderer: unknown): { width?: number | undefined; height?: number | undefined } {
  if (typeof renderer !== "object" || renderer === null) return {}
  const source = renderer as { readonly width?: unknown; readonly height?: unknown }
  const width = typeof source.width === "number" && source.width > 0 ? source.width : undefined
  const height = typeof source.height === "number" && source.height > 0 ? source.height : undefined
  return { width, height }
}

/** Ancho del contenido: el host fija el marco (máximo 116 columnas, capado a la terminal). */
const DIALOG_COLUMNS = 116
function contentWidth(termWidth: number | undefined): number {
  return Math.max(40, Math.min(DIALOG_COLUMNS, (termWidth ?? 120) - 2) - 4)
}

/** Alto máximo del contenido: 25 filas, sin superar el 80% de la terminal. */
function contentHeight(termHeight: number | undefined): number {
  return Math.max(8, Math.min(25, Math.floor(((termHeight ?? 30) * 4) / 5) - 4))
}

function OrchestratorDialog() {
  const context = usePlugin()
  const [snapshot, setSnapshot] = createSignal<Snapshot>()
  const [busy, setBusy] = createSignal(false)
  const [dimensions, setDimensions] = createSignal(rendererDimensions(context.renderer))
  const width = () => contentWidth(dimensions().width)
  const maxHeight = () => contentHeight(dimensions().height)

  const onResize = (nextWidth: number, nextHeight: number) => {
    setDimensions({ width: nextWidth, height: nextHeight })
  }
  context.renderer?.on("resize", onResize)
  onCleanup(() => context.renderer?.off("resize", onResize))

  const refresh = () => {
    if (busy()) return
    setBusy(true)
    const location = context.location ?? context.data.location.default()
    void queryRunner(location.directory).then((next) => {
      setSnapshot(next)
      setBusy(false)
    })
  }

  context.keymap.layer(() => ({
    commands: [
      {
        id: "orchestrator.status.close",
        title: "Cerrar estado del orquestador",
        bind: "escape",
        run: () => {
          context.ui.dialog.clear()
          return false
        },
      },
      {
        id: "orchestrator.status.refresh",
        title: "Actualizar estado del orquestador",
        bind: "r",
        run: refresh,
      },
    ],
  }))

  refresh()

  const footerText = () => {
    if (busy()) return "consultando…"
    const current = snapshot()
    if (!current) return "consultando…"
    const at = `actualizado a las ${formatClock(current.at)}`
    if (current.kind === "error") return `Error de consulta · ${at}`
    const data = current.data
    if (!data.state.valid) return `Estado corrupto: se muestra la fase 1 · ${at}`
    if (!data.state.evidence_valid) return `Historial inválido: el runner no registra evidencias · ${at}`
    if (!data.state.exists) return `Sin estado guardado: fase 1 por defecto · ${at}`
    return at
  }

  const footerFg = () => {
    const current = snapshot()
    if (current?.kind === "error") return context.theme.text.feedback.error.base
    if (current?.kind === "ok" && !current.data.state.valid) return context.theme.text.feedback.warning.base
    return context.theme.text.faint
  }

  const data = () => {
    const current = snapshot()
    return current?.kind === "ok" ? current.data : undefined
  }

  const error = () => {
    const current = snapshot()
    return current?.kind === "error" ? current.message : undefined
  }

  return (
    <scrollbox focused stickyScroll={false} maxHeight={maxHeight()}>
      <box flexDirection="column" paddingLeft={1} paddingRight={1}>
        <Show when={data()}>{(current) => <StatusBody data={current()} width={width()} />}</Show>
        <Show when={!data() && !error()}>
          <text fg={context.theme.text.muted}>Consultando el runner del orquestador…</text>
        </Show>
        <Show when={error()}>{(message) => <text fg={context.theme.text.feedback.error.base}>{message()}</text>}</Show>

        <text fg={context.theme.text.base}> </text>
        <Wrapped text={`${footerText()} · r actualiza · esc cierra`} width={width()} fg={footerFg()} />
      </box>
    </scrollbox>
  )
}

export default Plugin.define({
  id: "orchestrator-status",
  setup(context) {
    context.keymap.layer(() => ({
      mode: "global",
      commands: [
        {
          id: "orchestrator.status",
          title: "Estado del orquestador",
          group: "Orquestación",
          palette: true,
          slash: { name: "orquestador" },
          run: () => {
            context.ui.dialog.set({ size: "xlarge", centered: true })
            context.ui.dialog.show(() => <OrchestratorDialog />)
          },
        },
        {
          id: "orchestrator.status.close",
          title: "Cerrar estado del orquestador",
          group: "Orquestación",
          palette: true,
          run: () => {
            context.ui.dialog.clear()
          },
        },
      ],
    }))
  },
})
