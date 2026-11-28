/**
 * Declaraciones ambientales mínimas del plugin orchestrator-status.
 *
 * El plugin no se instala con npm: se distribuye copiando sus fuentes (tk-40) y
 * OpenCode resuelve `@opencode/plugin`, OpenTUI y Solid en runtime. Por eso aquí
 * se declara SOLO el subconjunto que usa `tui.tsx`, suficiente para que
 * `tsc --noEmit` funcione sin `node_modules`. Cada firma sigue la API real
 * publicada (`@opencode/plugin` 2.x, `@opentui/*` 0.9, `solid-js` 1.9); si el
 * plugin empieza a usar más API, se amplía este fichero o se instalan las
 * devDependencies de package.json.
 */

type OrchestratorColor = {
  readonly r: number
  readonly g: number
  readonly b: number
  readonly a: number
}

/** Global de Node que usa el plugin para leer variables de entorno. */
declare const process: { readonly env: Record<string, string | undefined> }

/** Temporizador del host TUI, para el refresco en segundo plano; el asa solo se devuelve a clearInterval. */
declare function setInterval(handler: () => void, timeout: number): unknown
declare function clearInterval(handle: unknown): void

/** Resultado de componer JSX; el host lo materializa. */
type OrchestratorElement = { readonly __orchestratorElement: never }

declare module "node:child_process" {
  export interface ExecFileError extends Error {
    readonly code?: string | number
  }

  export function execFile(
    file: string,
    args: readonly string[],
    options: { readonly cwd?: string; readonly timeout?: number; readonly env?: Record<string, string | undefined> },
    callback: (error: ExecFileError | null, stdout: string, stderr: string) => void,
  ): void
}

declare module "node:fs" {
  export function existsSync(path: string): boolean
}

declare module "node:os" {
  export function homedir(): string
}

declare module "node:path" {
  export function join(...parts: readonly string[]): string
}

declare module "solid-js" {
  export type Accessor<T> = () => T

  /** Solo la forma usada: crear con un valor inicial (obligatorio u opcional) y escribir con uno. */
  export function createSignal<T>(value: T): [Accessor<T>, (next: T) => void]
  export function createSignal<T>(value?: T): [Accessor<T | undefined>, (next: T | undefined) => void]

  /** Limpieza al desmontarse el componente que la llamó. */
  export function onCleanup(fn: () => void): void

  export interface ForProps<T> {
    readonly each: readonly T[] | undefined | null
    readonly fallback?: unknown
    readonly children: (item: T, index: Accessor<number>) => OrchestratorElement
  }

  export function For<T>(props: ForProps<T>): OrchestratorElement

  export interface ShowProps<T> {
    readonly when: T | undefined | null | false
    readonly fallback?: unknown
    readonly children: OrchestratorElement | ((item: Accessor<NonNullable<T>>) => OrchestratorElement)
  }

  export function Show<T>(props: ShowProps<T>): OrchestratorElement
}

declare module "@opentui/core" {
  export type RGBA = OrchestratorColor
}

declare module "@opentui/solid/jsx-runtime" {
  export namespace JSX {
    /** El host decide qué es un elemento; el plugin solo compone JSX. */
    type Element = OrchestratorElement

    interface TextProps {
      readonly children?: unknown
      readonly fg?: OrchestratorColor | string
      readonly bg?: OrchestratorColor | string
    }

    interface BoxProps {
      readonly children?: unknown
      readonly flexDirection?: "row" | "column"
      readonly width?: number | string
      readonly height?: number | string
      readonly flexGrow?: number
      readonly paddingLeft?: number
      readonly paddingRight?: number
    }

    /** El host hace scroll con teclado y rueda cuando el box tiene el foco; maxHeight lo acota. */
    interface ScrollBoxProps {
      readonly children?: unknown
      readonly focused?: boolean
      readonly stickyScroll?: boolean
      readonly flexGrow?: number
      readonly maxHeight?: number
      readonly width?: number | string
      readonly height?: number | string
    }

    interface IntrinsicElements {
      readonly box: BoxProps
      readonly text: TextProps
      readonly scrollbox: ScrollBoxProps
    }

    /** Sin esto TypeScript no entrega el hijo JSX como prop `children`. */
    interface ElementChildrenAttribute {
      children: {}
    }
  }
}

declare module "@opencode/plugin/tui" {
  export interface Theme {
    readonly text: {
      readonly base: OrchestratorColor
      readonly muted: OrchestratorColor
      readonly faint: OrchestratorColor
      readonly feedback: Readonly<
        Record<"error" | "warning" | "success" | "info", { readonly base: OrchestratorColor; readonly muted: OrchestratorColor }>
      >
    }
  }

  export interface LocationRef {
    readonly directory: string
  }

  /** El renderer real de OpenTUI: dimensiones de terminal y su evento de cambio de tamaño. */
  export interface RendererLike {
    readonly width?: number
    readonly height?: number
    on(event: string, listener: (width: number, height: number) => void): void
    off(event: string, listener: (width: number, height: number) => void): void
  }

  export interface KeymapCommand {
    readonly id?: string
    readonly title?: string
    readonly group?: string
    readonly palette?: true
    readonly slash?: { readonly name: string; readonly aliases?: readonly string[]; readonly arguments?: true }
    readonly bind?: false | string
    readonly run: (input?: string, event?: unknown) => void | false | Promise<void>
  }

  export interface KeymapLayer {
    readonly mode?: string
    readonly priority?: number
    readonly commands?: readonly KeymapCommand[]
    readonly bindings?: readonly string[]
  }

  export interface Keymap {
    layer(factory: () => KeymapLayer): void
  }

  /** Diálogo propio del host: los tamaños son los del host; xlarge es el más ancho (116 columnas). */
  export interface Dialog {
    set(options: { readonly size?: "small" | "medium" | "large" | "xlarge"; readonly centered?: boolean }): void
    show(render: () => unknown, onClosed?: () => void): void
    clear(): void
  }

  export interface UI {
    readonly dialog: Dialog
    readonly router: { current(): { readonly type: "session"; readonly sessionID: string } | { readonly type: "home" | "plugin" } }
  }

  export interface Context {
    readonly location: LocationRef | undefined
    readonly data: {
      readonly location: { default(): LocationRef }
    }
    readonly theme: Theme
    readonly ui: UI
    readonly keymap: Keymap
    readonly renderer: RendererLike | undefined
  }

  export interface Definition {
    readonly id: string
    readonly setup: (context: Context) => Promise<(() => void) | void> | (() => void) | void
  }

  export const Plugin: {
    define(definition: Definition): Definition
  }

  export function usePlugin(): Context
}
