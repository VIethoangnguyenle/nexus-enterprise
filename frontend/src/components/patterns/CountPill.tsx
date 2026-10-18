import { formatCount } from '../../lib/format'

/** The number of something new, 99+ above 99. Decoration: whoever reads the control gets the figure from its label. */
export function CountPill({ n }: { n: number }) {
  return (
    <span
      aria-hidden="true"
      className="inline-flex items-center justify-center h-5 min-w-5 px-1.5 rounded-full bg-accent text-on-accent text-2xs font-semibold tnum"
    >
      {formatCount(n)}
    </span>
  )
}
