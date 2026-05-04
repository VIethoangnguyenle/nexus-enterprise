import { useCallback } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { Input, Select, Button, Text } from '../primitives'
import { IconButton } from '../primitives'

export interface StepItem {
  step_order: number
  name: string
  approver_type: string
  approver_value: string
  required_count: number
  timeout_hours: number
}

interface StepBuilderProps {
  steps: StepItem[]
  onChange: (steps: StepItem[]) => void
}

const APPROVER_TYPES = [
  { value: 'specific_user', label: 'Specific User' },
  { value: 'role_in_dept', label: 'Role in Department' },
  { value: 'department', label: 'Department' },
  { value: 'creator_manager', label: "Creator's Manager" },
]

/** Builder for approval step chain — ordered step cards with type, name, required count. */
export function StepBuilder({ steps, onChange }: StepBuilderProps) {
  const addStep = useCallback(() => {
    const nextOrder = steps.length > 0 ? Math.max(...steps.map((s) => s.step_order)) + 1 : 1
    onChange([
      ...steps,
      {
        step_order: nextOrder,
        name: '',
        approver_type: 'role_in_dept',
        approver_value: '',
        required_count: 1,
        timeout_hours: 0,
      },
    ])
  }, [steps, onChange])

  const updateStep = useCallback(
    (index: number, patch: Partial<StepItem>) => {
      const next = steps.map((s, i) => (i === index ? { ...s, ...patch } : s))
      onChange(next)
    },
    [steps, onChange],
  )

  const removeStep = useCallback(
    (index: number) => {
      const next = steps.filter((_, i) => i !== index).map((s, i) => ({ ...s, step_order: i + 1 }))
      onChange(next)
    },
    [steps, onChange],
  )

  return (
    <div className="space-y-3">
      <Text variant="caption" muted className="uppercase tracking-wider">
        Approval Steps — Define approval chain
      </Text>

      {steps.map((step, i) => (
        <div
          key={i}
          className="flex items-center gap-2 p-3 bg-surface-container-lowest border border-outline-variant rounded-lg"
        >
          <span className="w-6 h-6 rounded-full bg-primary text-white flex items-center justify-center text-xs font-medium shrink-0">
            {step.step_order}
          </span>

          <Input
            type="text"
            value={step.name}
            onChange={(e) => updateStep(i, { name: e.target.value })}
            placeholder="Step name"
            className="flex-1 min-w-0"
          />

          <Select
            value={step.approver_type}
            onChange={(e) => updateStep(i, { approver_type: e.target.value })}
            className="w-40 shrink-0"
          >
            {APPROVER_TYPES.map((t) => (
              <option key={t.value} value={t.value}>{t.label}</option>
            ))}
          </Select>

          <Input
            type="text"
            value={step.approver_value}
            onChange={(e) => updateStep(i, { approver_value: e.target.value })}
            placeholder="UA / User ID"
            className="w-32 shrink-0"
          />

          <Input
            type="number"
            value={String(step.required_count)}
            onChange={(e) => updateStep(i, { required_count: Number(e.target.value) || 1 })}
            className="w-16 shrink-0"
            min={1}
          />

          <IconButton
            aria-label="Remove step"
            onClick={() => removeStep(i)}
            size="sm"
          >
            <Trash2 size={14} />
          </IconButton>
        </div>
      ))}

      <Button variant="ghost" onClick={addStep} className="w-full">
        <Plus size={14} />
        Add Step
      </Button>
    </div>
  )
}
