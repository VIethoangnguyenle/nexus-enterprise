import { useReducedMotion, type Transition, type TargetAndTransition } from 'motion/react'

/**
 * Shared motion presets, one per row of the DESIGN.md §7 table.
 *
 * `motion` needs numbers, not CSS variables, so the durations and curves are
 * mirrored here from the `--duration-*` / `--ease-*` tokens in index.css. Keep
 * the two in step: a change to one is a change to both.
 *
 * Only transform and opacity animate (plus height for row insert/remove, which
 * motion measures and drives as a layout property). Exits run at ~75% of the
 * entrance.
 */
export const DURATION = {
  press: 0.1, // --duration-press
  quick: 0.16, // --duration-quick
  base: 0.22, // --duration-base
  layout: 0.28, // --duration-layout
  exit: 0.21, // --duration-exit (75% of layout)
  reducedFade: 0.12, // reduced motion: fades only, at most 120ms
} as const

type Bezier = [number, number, number, number]
export const EASE: Record<'out' | 'outExpo' | 'in' | 'spring', Bezier> = {
  out: [0.25, 1, 0.5, 1], // --ease-out
  outExpo: [0.16, 1, 0.3, 1], // --ease-out-expo
  in: [0.5, 0, 0.75, 0], // --ease-in
  spring: [0.175, 0.885, 0.32, 1.275], // --ease-spring (reaction pop only)
}

/** Props spread onto a `motion.*` element inside `AnimatePresence`. */
export interface Preset {
  initial: TargetAndTransition
  animate: TargetAndTransition
  exit: TargetAndTransition
}

const fade = (inDur: number, outDur: number): Preset => ({
  initial: { opacity: 0 },
  animate: { opacity: 1, transition: { duration: inDur, ease: EASE.out } },
  exit: { opacity: 0, transition: { duration: outDur, ease: EASE.in } },
})

function slide(
  from: TargetAndTransition,
  inDur: number,
  inEase: Bezier,
  outDur: number,
): Preset {
  const rest: TargetAndTransition = { opacity: 1, x: 0, y: 0, scale: 1 }
  return {
    initial: { opacity: 0, ...from },
    animate: { ...rest, transition: { duration: inDur, ease: inEase } },
    exit: { opacity: 0, ...from, transition: { duration: outDur, ease: EASE.in } },
  }
}

export interface MotionPresets {
  reduced: boolean
  /** Detail panel: translateX(16px) scale(.985) → 0, 280ms expo; out 210ms. */
  panel: Preset
  /** Modal surface: translateY(8px) scale(.98) → 0, 280ms expo; out 210ms. */
  modal: Preset
  /** Scrim behind a modal: fade 220ms; out 210ms. */
  scrim: Preset
  /** Popover / menu: translateY(-4px) + fade, 160ms; out 120ms. */
  popover: Preset
  /** Toast: translateY(12px) + fade, 220ms; out 160ms. */
  toast: Preset
  /** Row insert / remove: height + fade, 220ms in, 160ms out. */
  row: Preset
  /** Route content: fade 160ms, no slide. */
  route: Preset
  /** Transition for `layout` re-ordering (FLIP), 280ms expo. */
  layout: Transition
  /** `layout` prop value: off entirely under reduced motion. */
  layoutProp: boolean | 'position'
}

export function presets(reduced: boolean): MotionPresets {
  if (reduced) {
    const f = fade(DURATION.reducedFade, DURATION.reducedFade)
    return {
      reduced,
      panel: f,
      modal: f,
      scrim: f,
      popover: f,
      toast: f,
      row: f,
      route: f,
      layout: { duration: 0 },
      layoutProp: false,
    }
  }
  return {
    reduced,
    panel: slide({ x: 16, scale: 0.985 }, DURATION.layout, EASE.outExpo, DURATION.exit),
    modal: slide({ y: 8, scale: 0.98 }, DURATION.layout, EASE.outExpo, DURATION.exit),
    scrim: fade(DURATION.base, DURATION.exit),
    popover: slide({ y: -4 }, DURATION.quick, EASE.out, 0.12),
    toast: slide({ y: 12 }, DURATION.base, EASE.out, DURATION.quick),
    row: {
      initial: { opacity: 0, height: 0 },
      animate: { opacity: 1, height: 'auto', transition: { duration: DURATION.base, ease: EASE.outExpo } },
      exit: { opacity: 0, height: 0, transition: { duration: DURATION.quick, ease: EASE.in } },
    },
    route: fade(DURATION.quick, DURATION.quick),
    layout: { duration: DURATION.layout, ease: EASE.outExpo },
    layoutProp: 'position',
  }
}

/** Gap between consecutive rows of a list that has just appeared. */
export const STAGGER_STEP = 0.03
/** Rows past this one enter together, so a long list never keeps its tail waiting. */
export const STAGGER_ROWS = 8

/**
 * Entrance delay (seconds) for row `index` of a list that has just appeared:
 * 30ms apart over the first eight rows. None under reduced motion.
 */
export function staggerDelay(index: number, reduced: boolean): number {
  if (reduced) return 0
  return Math.min(Math.max(index, 0), STAGGER_ROWS) * STAGGER_STEP
}

/** The same preset with its entrance delayed. Exits are left alone. */
export function withDelay(preset: Preset, delay: number): Preset {
  if (delay <= 0) return preset
  const base = preset.animate.transition
  const transition = { ...(base as object), delay } as Transition
  return { ...preset, animate: { ...preset.animate, transition } }
}

/** Presets for the current user's motion preference. */
export function useMotionPresets(): MotionPresets {
  return presets(!!useReducedMotion())
}
