import { useEffect, useState, type ReactNode } from 'react'
import { Mic, Video, Volume2 } from 'lucide-react'
import type { CallRoom } from '../../lib/call-room'
import { listDevices, type DeviceKind, type DeviceOption } from '../../lib/call-devices'
import { loadCallPrefs, saveCallPrefs } from '../../lib/call-prefs'
import { Button } from '../primitives'
import { DeviceSelect } from './DeviceSelect'
import { Dialog } from '../composites/Dialog'

const FIELDS: { kind: DeviceKind; label: string; pref: 'micId' | 'camId' | 'speakerId'; icon: ReactNode }[] = [
  { kind: 'audioinput', label: 'Micro', pref: 'micId', icon: <Mic className="cs-icon-s" /> },
  { kind: 'videoinput', label: 'Camera', pref: 'camId', icon: <Video className="cs-icon-s" /> },
  { kind: 'audiooutput', label: 'Loa', pref: 'speakerId', icon: <Volume2 className="cs-icon-s" /> },
]

/** "Micro, camera, loa" during a call: the same three choices as the check before joining. */
export function DeviceSettings({ open, onClose, callRoom, userId }: {
  open: boolean
  onClose: () => void
  callRoom: CallRoom | null
  userId: string | undefined
}) {
  const [lists, setLists] = useState<Partial<Record<DeviceKind, DeviceOption[]>>>({})
  const [chosen, setChosen] = useState(() => loadCallPrefs(userId))

  useEffect(() => {
    if (!open) return
    let live = true
    setChosen(loadCallPrefs(userId))
    for (const f of FIELDS) {
      void listDevices(f.kind, false).then((opts) => { if (live) setLists((l) => ({ ...l, [f.kind]: opts })) })
    }
    return () => { live = false }
  }, [open, userId])

  const pick = (kind: DeviceKind, pref: 'micId' | 'camId' | 'speakerId', id: string) => {
    const next = { ...chosen, [pref]: id }
    setChosen(next)
    saveCallPrefs(userId, next)
    void callRoom?.useDevice(kind, id)
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Micro, camera, loa"
      width="sm"
      footer={<Button variant="primary" onClick={onClose}>Xong</Button>}
    >
      <div className="grid gap-3">
        {FIELDS.map((f) => {
          const opts = lists[f.kind] ?? []
          const supported = f.kind !== 'audiooutput' || (typeof HTMLMediaElement !== 'undefined' && 'setSinkId' in HTMLMediaElement.prototype)
          return (
            <div key={f.kind} className="cs-field">
              <label htmlFor={`dev-${f.kind}`}>{f.label}</label>
              <DeviceSelect
                id={`dev-${f.kind}`}
                icon={f.icon}
                value={chosen[f.pref] ?? ''}
                options={opts}
                disabled={!supported || opts.length === 0}
                onChange={(id) => pick(f.kind, f.pref, id)}
              />
            </div>
          )
        })}
      </div>
    </Dialog>
  )
}
