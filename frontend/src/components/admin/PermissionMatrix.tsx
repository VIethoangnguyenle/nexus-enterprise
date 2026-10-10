import { useRef, type KeyboardEvent } from 'react'
import { Check, FileText, MessagesSquare, Package, ShieldCheck } from 'lucide-react'
import type { PermissionArea } from '../../api/admin'
import { areaHint, areaLabel, matrixColumns, opHint, opLabel, type Grants } from '../../lib/admin-model'
import { Pressable } from '../primitives'

const AREA_ICON: Record<string, typeof FileText> = {
  documents: FileText,
  channels: MessagesSquare,
  assets: Package,
  management: ShieldCheck,
}

interface PermissionMatrixProps {
  /** What the server says can be granted where. Drives the rows and the columns. */
  areas: PermissionArea[]
  /** What is drafted now. */
  draft: Grants
  /** Areas whose draft differs from what is saved: their rows are tinted until saved. */
  changed: ReadonlySet<string>
  /** Name of the role, for the cells' accessible names. */
  role: string
  onToggle: (area: PermissionArea, op: string, on: boolean) => void
  disabled?: boolean
}

/**
 * Vùng tài nguyên × thao tác (design/mockups/admin.html §5): rows are the areas
 * and columns the operations the server offers on any of them. A cell is a
 * toggle button, 44×44, where the operation applies to the area; where it does
 * not there is nothing to press. Arrow keys move between cells.
 */
export function PermissionMatrix({ areas, draft, changed, role, onToggle, disabled }: PermissionMatrixProps) {
  const cols = matrixColumns(areas)
  const gridRef = useRef<HTMLDivElement>(null)

  // The column count comes from the server's answer, so it cannot be a static class.
  const template = { gridTemplateColumns: `minmax(11rem,1.6fr) repeat(${cols.length}, minmax(3.25rem,1fr))` }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const step: Record<string, [number, number]> = {
      ArrowRight: [0, 1], ArrowLeft: [0, -1], ArrowDown: [1, 0], ArrowUp: [-1, 0],
    }
    const d = step[e.key]
    const here = (e.target as HTMLElement).closest<HTMLElement>('[data-cell]')
    if (!d || !here) return
    let r = Number(here.dataset.r) + d[0]
    let c = Number(here.dataset.c) + d[1]
    // Skip cells where the operation does not apply, in the direction of travel.
    while (r >= 0 && r < areas.length && c >= 0 && c < cols.length) {
      const next = gridRef.current?.querySelector<HTMLElement>(`[data-cell][data-r="${r}"][data-c="${c}"]`)
      if (next) {
        e.preventDefault()
        next.focus()
        return
      }
      r += d[0]
      c += d[1]
    }
  }

  return (
    <div role="grid" aria-label={`Quyền của ${role}`} ref={gridRef} onKeyDown={onKeyDown} className="grid content-start">
      <div role="row" style={template} className="grid items-end min-h-11 px-2.5 text-xs font-semibold text-ink-muted border-b border-line">
        <span role="columnheader" className="pb-2">Vùng tài nguyên</span>
        {cols.map((op) => (
          <span key={op} role="columnheader" title={opHint(op)} className="pb-2 text-center leading-tight">{opLabel(op)}</span>
        ))}
      </div>
      {areas.map((area, r) => {
        const Icon = AREA_ICON[area.area] ?? FileText
        const held = new Set(draft[area.area] ?? [])
        return (
          <div
            key={area.area}
            role="row"
            style={template}
            className={`grid items-center min-h-14 px-2.5 border-b border-line transition-colors duration-quick
              ${changed.has(area.area) ? 'bg-warning-wash' : ''}`}
          >
            <span role="rowheader" className="flex items-center gap-2.5 min-w-0 py-1.5">
              <Icon size={18} strokeWidth={1.75} className="text-ink-muted shrink-0" aria-hidden="true" />
              <span className="grid min-w-0">
                <span className="font-semibold truncate">{areaLabel(area.area)}</span>
                <small className="text-xs text-ink-muted truncate">{areaHint(area.area)}</small>
              </span>
            </span>
            {cols.map((op, c) => {
              if (!area.operations.includes(op)) {
                return (
                  <span key={op} role="gridcell" className="grid place-items-center h-11">
                    <span className="block w-2 h-0.5 rounded-full bg-ink-subtle" aria-hidden="true" />
                    <span className="sr-only">Không áp dụng</span>
                  </span>
                )
              }
              const on = held.has(op)
              return (
                <span key={op} role="gridcell" className="grid place-items-center">
                  <Pressable
                    data-cell
                    data-r={r}
                    data-c={c}
                    aria-pressed={on}
                    aria-label={`${opLabel(op)} trên ${areaLabel(area.area)}`}
                    title={opHint(op)}
                    disabled={disabled}
                    onClick={() => onToggle(area, op, !on)}
                    className="grid place-items-center w-11 h-11 rounded-md press"
                  >
                    <span
                      aria-hidden="true"
                      className={`grid place-items-center w-5.5 h-5.5 rounded-sm transition-colors duration-press
                        ${on ? 'bg-accent text-on-accent' : 'bg-raised field-line'}`}
                    >
                      {on && <Check size={14} strokeWidth={2.5} />}
                    </span>
                  </Pressable>
                </span>
              )
            })}
          </div>
        )
      })}
    </div>
  )
}
