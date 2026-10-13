import type { ReactNode } from 'react'
import { ChevronDown } from 'lucide-react'
import type { DeviceOption } from '../../lib/call-devices'
import { Select } from '../primitives'
import './call-stage.css'

/** A device choice (mockup §2): the kind's icon, the name the operating system gave it, a chevron. Never an id. */
export function DeviceSelect({ id, icon, value, options, disabled, onChange }: {
  id: string
  icon: ReactNode
  /** The chosen device id; empty is "Mặc định". */
  value: string
  options: DeviceOption[]
  disabled?: boolean
  onChange: (id: string) => void
}) {
  return (
    <div className="cs-selwrap">
      <span className="cs-selicon" aria-hidden="true">{icon}</span>
      <Select id={id} className="pl-10 pr-9 h-11" value={value} disabled={disabled} onChange={(e) => onChange(e.target.value)}>
        {!value && <option value="">Mặc định</option>}
        {options.map((o) => <option key={o.id} value={o.id}>{o.label}</option>)}
      </Select>
      <ChevronDown className="cs-selchev cs-icon-s" aria-hidden="true" />
    </div>
  )
}
