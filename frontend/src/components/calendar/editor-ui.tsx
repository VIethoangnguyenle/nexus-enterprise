import type { ReactNode } from 'react'

/** A line of the editor: an icon in a 40px column and the control beside it. */
export function EditorLine({ icon, children, top }: { icon: ReactNode; children: ReactNode; top?: boolean }) {
  return (
    <div className={`grid grid-cols-[2.5rem_minmax(0,1fr)] gap-x-1 ${top ? 'items-start' : 'items-center'}`}>
      <span className={`grid place-items-center size-10 text-ink-muted`} aria-hidden="true">{icon}</span>
      <div className="min-w-0">{children}</div>
    </div>
  )
}

/** Names of the zones the browser knows; the one in use is always among them. */
export function zoneNames(current: string): string[] {
  const known = (Intl as { supportedValuesOf?: (k: string) => string[] }).supportedValuesOf?.('timeZone')
    ?? ['Asia/Ho_Chi_Minh', 'Asia/Bangkok', 'Asia/Singapore', 'Asia/Tokyo', 'Europe/London', 'America/New_York', 'UTC']
  return known.includes(current) ? known : [current, ...known]
}

export const zoneLabel = (tz: string) => tz.replace(/_/g, ' ')
