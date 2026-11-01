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

/** Vietnamese names for the zones people here pick most; any other zone reads as the place in its id. */
const ZONE_PLACES: Record<string, string> = {
  'Asia/Ho_Chi_Minh': 'Hà Nội',
  'Asia/Saigon': 'Hà Nội',
  'Asia/Bangkok': 'Bangkok',
  'Asia/Singapore': 'Singapore',
  'Asia/Tokyo': 'Tokyo',
  'Asia/Seoul': 'Seoul',
  'Asia/Shanghai': 'Thượng Hải',
  'Asia/Hong_Kong': 'Hồng Kông',
  'Asia/Kolkata': 'Kolkata',
  'Asia/Calcutta': 'Kolkata',
  'Australia/Sydney': 'Sydney',
  'Europe/London': 'London',
  'Europe/Paris': 'Paris',
  'America/New_York': 'New York',
  'America/Los_Angeles': 'Los Angeles',
  UTC: 'Giờ quốc tế (UTC)',
}

const zonePlace = (tz: string) => ZONE_PLACES[tz] ?? (tz.split('/').pop() ?? tz).replace(/_/g, ' ')

/**
 * The zone's offset from GMT right now, in minutes east, or null when the browser does not know the zone.
 * Read now, so a zone on summer time shows its summer offset.
 */
function zoneOffset(tz: string): number | null {
  let name: string | undefined
  try {
    name = new Intl.DateTimeFormat('en-US', { timeZone: tz, timeZoneName: 'shortOffset' })
      .formatToParts(new Date()).find((p) => p.type === 'timeZoneName')?.value
  } catch {
    return null
  }
  if (name === 'GMT' || name === 'UTC') return 0
  const m = name ? /^GMT([+-])(\d{1,2})(?::(\d{2}))?$/.exec(name) : null
  if (!m) return null
  const mins = Number(m[2]) * 60 + Number(m[3] ?? 0)
  return m[1] === '-' ? -mins : mins
}

function offsetText(mins: number): string {
  const abs = Math.abs(mins)
  const rest = abs % 60
  return `GMT${mins < 0 ? '-' : '+'}${Math.floor(abs / 60)}${rest ? `:${String(rest).padStart(2, '0')}` : ''}`
}

/** How a zone reads on screen: "(GMT+7) Hà Nội", "(GMT+5:30) Kolkata"; just the place when the offset is unknown. */
export function zoneLabel(tz: string): string {
  const offset = zoneOffset(tz)
  return offset === null ? zonePlace(tz) : `(${offsetText(offset)}) ${zonePlace(tz)}`
}

/**
 * Zones the browser knows, ordered by offset and then by name; the one in use is always among them. Zones that
 * read the same (an old id and its new one, e.g. Asia/Saigon and Asia/Ho_Chi_Minh) appear once, as the one in use
 * when it is either.
 */
export function zoneNames(current: string): string[] {
  const known = (Intl as { supportedValuesOf?: (k: string) => string[] }).supportedValuesOf?.('timeZone')
    ?? Object.keys(ZONE_PLACES)
  const seen = new Set<string>()
  const zones: { tz: string; label: string; offset: number }[] = []
  for (const tz of current ? [current, ...known] : known) {
    const label = zoneLabel(tz)
    if (seen.has(label)) continue
    seen.add(label)
    zones.push({ tz, label, offset: zoneOffset(tz) ?? Number.POSITIVE_INFINITY })
  }
  return zones
    .sort((a, b) => a.offset - b.offset || a.label.localeCompare(b.label, 'vi'))
    .map((z) => z.tz)
}
