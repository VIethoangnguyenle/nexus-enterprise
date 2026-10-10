import { Fragment } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import { Avatar } from '../primitives'
import { formatDateTime } from '../../lib/format'
import { formatStamp, type AssetPersonView, type Segment } from '../../lib/asset-model'
import { useMotionPresets } from '../../lib/motion'
import { personStyle } from '../../lib/person-hue'
import { AssetPill } from './AssetPill'

export interface StepItem {
  key: string
  actor: AssetPersonView
  /** What they did, after their name. */
  segments: Segment[]
  /** The note they left. */
  comment?: string
  at: unknown
  /** The change is someone else's and new: wash the row in their hue for a moment. */
  fresh?: boolean
}

/** One sentence of a step: text, names in bold, a pill for a state. */
export function Sentence({ segments }: { segments: Segment[] }) {
  return (
    <>
      {segments.map((s, i) => (
        <Fragment key={i}>
          {typeof s === 'string' ? s : 'b' in s ? <b className="font-semibold">{s.b}</b> : <AssetPill pill={s.pill} />}
        </Fragment>
      ))}
    </>
  )
}

/**
 * Who did what, and when (mockup §1, §2): an avatar, the actor's name in bold,
 * a sentence, the note they left, and the time, with the full date on hover.
 * Used by an asset's history and by the dashboard's recent activity.
 */
export function StepList({ items, label }: { items: StepItem[]; label: string }) {
  const m = useMotionPresets()
  return (
    <ol aria-label={label} className="grid m-0 p-0 list-none">
      {/* Entries on screen at first paint just appear; one that arrives later grows in. */}
      <AnimatePresence initial={false}>
        {items.map((it) => (
          <motion.li key={it.key} {...m.row} layout={m.layoutProp} transition={m.layout} className="overflow-hidden">
            <div
              style={it.fresh ? personStyle(it.actor.hueKey) : undefined}
              className={`grid grid-cols-[24px_minmax(0,1fr)] gap-2.5 items-start rounded-md py-1.5 ${it.fresh ? 'rt-wash' : ''}`}
            >
              <Avatar name={it.actor.name} hueKey={it.actor.hueKey} src={it.actor.avatarUrl} size={24} halo={it.fresh} />
              <p className="m-0 text-sm leading-snug min-w-0 break-words">
                <b className="font-semibold">{it.actor.name}</b> <Sentence segments={it.segments} />
                {it.comment && <span className="text-ink-muted"> · “{it.comment}”</span>}
                <time title={formatDateTime(it.at)} className="block text-xs text-ink-muted tnum">
                  {formatStamp(it.at)}
                </time>
              </p>
            </div>
          </motion.li>
        ))}
      </AnimatePresence>
    </ol>
  )
}
