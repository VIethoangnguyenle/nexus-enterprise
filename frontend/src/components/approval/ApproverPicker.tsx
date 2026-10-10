import { useMemo } from 'react'
import { Building2, ShieldCheck } from 'lucide-react'
import { useDepartments } from '../../hooks/useAdmin'
import { useWorkspaceRoles } from '../../hooks/useApproval'
import { APPROVER_KINDS, type ApproverType } from '../../lib/approval-model'
import { UNKNOWN_PERSON, type PeopleDirectory, type Person } from '../../lib/people'
import { ChoicePicker, PeoplePicker, Pressable } from '../primitives'

/** Who approves a step, as the builder holds it. `value` is the key the server resolves; `name` is what is shown. */
export interface Approver {
  type: ApproverType
  value: string
  name: string
}

interface ApproverPickerProps {
  /** Distinguishes this step's controls from the others on the page ("bước 2"). */
  stepLabel: string
  workspaceId: string
  people: PeopleDirectory
  value: Approver
  onChange: (next: Approver) => void
  invalid?: boolean
}

/**
 * Chooses who approves a step: a person, a role or a department, each picked
 * from a list by name. Switching the kind clears the pick (a person's key means
 * nothing as a role's).
 */
export function ApproverPicker({ stepLabel, workspaceId, people, value, onChange, invalid }: ApproverPickerProps) {
  const roles = useWorkspaceRoles(workspaceId, value.type === 'role_in_dept')
  const departments = useDepartments(value.type === 'department' ? workspaceId : '')

  const roleChoices = useMemo(
    () => (roles.data ?? []).map((r) => ({ id: r.ngac_node_id, name: r.name })),
    [roles.data],
  )
  const deptChoices = useMemo(
    () => (departments.data?.flat ?? []).map((d) => ({ id: d.id, name: d.name })),
    [departments.data],
  )

  // The person already chosen, from the directory, or by the name the server gave.
  const person: Person[] = useMemo(() => {
    if (value.type !== 'specific_user' || !value.value) return []
    return [
      people.byNodeId.get(value.value) ?? {
        userId: value.value, nodeId: value.value, username: '', name: value.name || UNKNOWN_PERSON, role: '', avatarUrl: '',
      },
    ]
  }, [people, value])

  const setKind = (type: ApproverType) => {
    if (type !== value.type) onChange({ type, value: '', name: '' })
  }

  return (
    <div className="grid gap-2">
      <div role="radiogroup" aria-label={`Người duyệt ${stepLabel}`} className="flex flex-wrap gap-2">
        {APPROVER_KINDS.map((k) => {
          const on = k.type === value.type
          return (
            <Pressable
              key={k.type}
              role="radio"
              aria-checked={on}
              title={k.hint}
              onClick={() => setKind(k.type)}
              className={`h-8 px-3 rounded-md text-small-ui
                ${on ? 'bg-accent-wash text-ink font-semibold' : 'bg-raised text-ink hover:bg-hover'}`}
            >
              {k.label}
            </Pressable>
          )
        })}
      </div>

      {value.type === 'specific_user' && (
        <PeoplePicker
          label={`Chọn người duyệt ${stepLabel}`}
          people={people.list}
          value={person}
          max={1}
          placeholder="Tìm theo tên hoặc phòng ban"
          onChange={(next) => {
            const p = next[next.length - 1]
            onChange(p ? { type: 'specific_user', value: p.nodeId, name: p.name } : { type: 'specific_user', value: '', name: '' })
          }}
        />
      )}
      {value.type === 'role_in_dept' && (
        <ChoicePicker
          label={`Chọn vai trò duyệt ${stepLabel}`}
          choices={roleChoices}
          value={value.value ? { id: value.value, name: value.name || 'Vai trò đã chọn' } : null}
          icon={<ShieldCheck size={16} strokeWidth={1.75} />}
          placeholder="Tìm vai trò"
          emptyText="Không có vai trò nào khớp."
          loading={roles.isLoading}
          invalid={invalid}
          onChange={(c) => onChange({ type: 'role_in_dept', value: c?.id ?? '', name: c?.name ?? '' })}
        />
      )}
      {value.type === 'department' && (
        <ChoicePicker
          label={`Chọn phòng ban duyệt ${stepLabel}`}
          choices={deptChoices}
          value={value.value ? { id: value.value, name: value.name || 'Phòng ban đã chọn' } : null}
          icon={<Building2 size={16} strokeWidth={1.75} />}
          placeholder="Tìm phòng ban"
          emptyText="Không có phòng ban nào khớp."
          loading={departments.isLoading}
          invalid={invalid}
          onChange={(c) => onChange({ type: 'department', value: c?.id ?? '', name: c?.name ?? '' })}
        />
      )}
    </div>
  )
}
