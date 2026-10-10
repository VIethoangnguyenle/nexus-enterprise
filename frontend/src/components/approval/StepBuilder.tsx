import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-react'
import { newUid } from '../../lib/approval-model'
import type { PeopleDirectory } from '../../lib/people'
import { Button, IconButton, TextField } from '../primitives'
import { ApproverPicker, type Approver } from './ApproverPicker'

/** A step as the builder holds it. `uid` is only a React key; it never reaches the server. */
export interface StepDraft {
  uid: number
  name: string
  approver: Approver
  requiredCount: number
  /** Kept from the saved template so an edit does not reset it. */
  timeoutHours: number
}

export interface StepErrors { name?: string; approver?: string }

export const blankStep = (): StepDraft => ({
  uid: newUid(), name: '', approver: { type: 'specific_user', value: '', name: '' }, requiredCount: 1, timeoutHours: 0,
})

interface StepBuilderProps {
  steps: StepDraft[]
  onChange: (steps: StepDraft[]) => void
  errors: Record<number, StepErrors>
  workspaceId: string
  people: PeopleDirectory
}

/**
 * The approval chain of a template: ordered steps, each with a name and an
 * approver chosen by picker (a person, a role or a department). Steps can be
 * moved up and down; the order is the order of approval.
 */
export function StepBuilder({ steps, onChange, errors, workspaceId, people }: StepBuilderProps) {
  const update = (uid: number, patch: Partial<StepDraft>) =>
    onChange(steps.map((s) => (s.uid === uid ? { ...s, ...patch } : s)))
  const move = (index: number, by: -1 | 1) => {
    const to = index + by
    if (to < 0 || to >= steps.length) return
    const next = [...steps]
    ;[next[index], next[to]] = [next[to]!, next[index]!]
    onChange(next)
  }

  return (
    <div className="grid gap-3">
      <ol aria-label="Các bước duyệt" className="grid gap-3 m-0 p-0 list-none">
        {steps.map((s, i) => {
          const err = errors[s.uid]
          const label = `bước ${i + 1}`
          return (
            <li key={s.uid} className="grid gap-3 p-3 rounded-surface bg-sunk">
              <div className="flex items-start gap-2.5">
                <span
                  className="grid place-items-center w-6 h-6 mt-8 rounded-full bg-accent text-on-accent text-xs font-semibold shrink-0 tnum"
                  aria-hidden="true"
                >
                  {i + 1}
                </span>
                <TextField
                  className="flex-1 min-w-0"
                  label={`Tên ${label}`}
                  value={s.name}
                  placeholder="Ví dụ: Trưởng phòng duyệt"
                  error={err?.name}
                  onChange={(e) => update(s.uid, { name: e.target.value })}
                />
                <div className="flex items-center gap-0.5 mt-7 shrink-0">
                  <IconButton size="md" aria-label={`Đưa ${label} lên trước`} disabled={i === 0} onClick={() => move(i, -1)}>
                    <ArrowUp size={16} strokeWidth={1.75} />
                  </IconButton>
                  <IconButton size="md" aria-label={`Đưa ${label} xuống sau`} disabled={i === steps.length - 1} onClick={() => move(i, 1)}>
                    <ArrowDown size={16} strokeWidth={1.75} />
                  </IconButton>
                  <IconButton
                    size="md"
                    tone="danger"
                    aria-label={`Xoá ${label}`}
                    disabled={steps.length === 1}
                    onClick={() => onChange(steps.filter((x) => x.uid !== s.uid))}
                  >
                    <Trash2 size={16} strokeWidth={1.75} />
                  </IconButton>
                </div>
              </div>
              <div className="grid gap-1.5 pl-8.5">
                <span className="text-sm font-semibold text-ink">Người duyệt</span>
                <ApproverPicker
                  stepLabel={label}
                  workspaceId={workspaceId}
                  people={people}
                  value={s.approver}
                  invalid={!!err?.approver}
                  onChange={(approver) => update(s.uid, { approver })}
                />
                {err?.approver && <span role="alert" className="text-xs font-medium text-danger">{err.approver}</span>}
              </div>
              <TextField
                className="pl-8.5 w-72"
                label={`Số người cần duyệt ${label}`}
                labelHint="(tối thiểu)"
                type="number"
                min={1}
                value={String(s.requiredCount)}
                onChange={(e) => update(s.uid, { requiredCount: Math.max(1, Math.floor(Number(e.target.value)) || 1) })}
              />
            </li>
          )
        })}
      </ol>
      <Button variant="soft" size="sm" className="justify-self-start" onClick={() => onChange([...steps, blankStep()])}>
        <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
        Thêm bước
      </Button>
    </div>
  )
}
