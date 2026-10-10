import type { PermissionArea } from '../../api/admin'
import { areaLabel, opHint, opLabel, type Grants } from '../../lib/admin-model'
import { Pressable } from '../primitives'

interface PermissionSwitchesProps {
  areas: PermissionArea[]
  draft: Grants
  changed: ReadonlySet<string>
  role: string
  onToggle: (area: PermissionArea, op: string, on: boolean) => void
  disabled?: boolean
}

/**
 * The phone's form of the permission matrix (design/mockups/admin.html §9): the
 * same areas and operations, as a list of switches grouped by area. An operation
 * an area does not offer is simply not listed.
 */
export function PermissionSwitches({ areas, draft, changed, role, onToggle, disabled }: PermissionSwitchesProps) {
  return (
    <div className="grid gap-4 content-start" role="group" aria-label={`Quyền của ${role}`}>
      {areas.map((area) => {
        const held = new Set(draft[area.area] ?? [])
        return (
          <section key={area.area} aria-label={areaLabel(area.area)} className="grid">
            <h3 className="m-0 px-4 pt-2 pb-1 text-label text-ink-muted">{areaLabel(area.area)}</h3>
            {area.operations.map((op) => {
              const on = held.has(op)
              return (
                <div
                  key={op}
                  className={`flex items-center gap-3 min-h-14 px-4 border-b border-line transition-colors duration-quick
                    ${changed.has(area.area) ? 'bg-warning-wash' : ''}`}
                >
                  <span className="grid flex-1 min-w-0">
                    <span className="font-semibold">{opLabel(op)}</span>
                    <small className="text-xs text-ink-muted">{opHint(op)}</small>
                  </span>
                  <Pressable
                    role="switch"
                    aria-checked={on}
                    aria-label={`${opLabel(op)} trên ${areaLabel(area.area)}`}
                    disabled={disabled}
                    onClick={() => onToggle(area, op, !on)}
                    className="relative shrink-0 w-10 h-6 rounded-full transition-colors duration-press
                      before:absolute before:-inset-2.5 before:content-['']"
                  >
                    <span
                      aria-hidden="true"
                      className={`absolute inset-0 rounded-full transition-colors duration-press ${on ? 'bg-accent' : 'bg-sunk field-line'}`}
                    />
                    <span
                      aria-hidden="true"
                      className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-raised shadow-overlay
                        transition-transform duration-press ${on ? 'translate-x-4' : ''}`}
                    />
                  </Pressable>
                </div>
              )
            })}
          </section>
        )
      })}
    </div>
  )
}
