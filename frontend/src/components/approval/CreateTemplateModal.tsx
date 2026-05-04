import { useState, useCallback, useMemo } from 'react'
import { Modal } from '../composites'
import { Button, Input, Select, Spinner, Text } from '../primitives'
import { FormFieldBuilder, type FormFieldItem } from './FormFieldBuilder'
import { StepBuilder, type StepItem } from './StepBuilder'
import { useCreateTemplate, useUpdateTemplate } from '../../hooks/useApproval'
import type { ApprovalTemplate } from '../../api/approval'

interface CreateTemplateModalProps {
  onClose: () => void
  editTemplate?: ApprovalTemplate | null
}

const ENTITY_TYPES = ['purchase', 'expense', 'leave', 'hiring', 'contract', 'custom']

/** Modal for creating or editing an approval template — basic info + form fields + steps. */
export function CreateTemplateModal({ onClose, editTemplate }: CreateTemplateModalProps) {
  const isEdit = !!editTemplate
  const createMut = useCreateTemplate()
  const updateMut = useUpdateTemplate()
  const isMutating = createMut.isPending || updateMut.isPending

  // Basic info
  const [name, setName] = useState(editTemplate?.name || '')
  const [entityType, setEntityType] = useState(editTemplate?.entity_type || 'purchase')

  // Form fields
  const [formFields, setFormFields] = useState<FormFieldItem[]>(
    editTemplate?.form_fields?.map((f) => ({
      label: f.label,
      field_type: f.field_type,
      required: f.required,
      options: f.options || '',
      placeholder: f.placeholder || '',
    })) || [],
  )

  // Steps
  const [steps, setSteps] = useState<StepItem[]>(
    editTemplate?.steps?.map((s) => ({
      step_order: s.step_order,
      name: s.name,
      approver_type: s.approver_type,
      approver_value: s.approver_value,
      required_count: s.required_count,
      timeout_hours: s.timeout_hours,
    })) || [
      { step_order: 1, name: '', approver_type: 'role_in_dept', approver_value: '', required_count: 1, timeout_hours: 0 },
    ],
  )

  const isValid = useMemo(() => {
    if (!name.trim()) return false
    if (steps.length === 0) return false
    return steps.every((s) => s.name.trim() !== '')
  }, [name, steps])

  const handleSubmit = useCallback(() => {
    if (!isValid) return

    const payload = {
      name,
      entity_type: entityType,
      priority: 0,
      form_fields: formFields.filter((f) => f.label.trim()),
      steps: steps.map((s) => ({
        step_order: s.step_order,
        name: s.name,
        approver_type: s.approver_type,
        approver_value: s.approver_value,
        required_count: s.required_count,
        timeout_hours: s.timeout_hours,
      })),
    }

    if (isEdit && editTemplate) {
      updateMut.mutate(
        { id: editTemplate.id, input: payload },
        { onSuccess: onClose },
      )
    } else {
      createMut.mutate(payload, { onSuccess: onClose })
    }
  }, [isValid, name, entityType, formFields, steps, isEdit, editTemplate, createMut, updateMut, onClose])

  return (
    <Modal onClose={onClose} size="lg">
      <Modal.Header onClose={onClose}>{isEdit ? 'Edit Template' : 'Create Approval Template'}</Modal.Header>
      <Modal.Body className="max-h-[70vh] overflow-y-auto">
        <div className="space-y-6">
          {/* Basic info */}
          <div className="space-y-3">
            <Text variant="caption" muted className="uppercase tracking-wider">
              Basic Information
            </Text>
            <div className="flex flex-col gap-1">
              <label className="text-caption-ui text-on-surface-variant">
                Template Name <span className="text-danger">*</span>
              </label>
              <Input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g., Purchase Request Approval"
              />
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-caption-ui text-on-surface-variant">Entity Type</label>
              <Select
                value={entityType}
                onChange={(e) => setEntityType(e.target.value)}
              >
                {ENTITY_TYPES.map((t) => (
                  <option key={t} value={t}>
                    {t.charAt(0).toUpperCase() + t.slice(1)}
                  </option>
                ))}
              </Select>
            </div>
          </div>

          {/* Form fields */}
          <div className="border-t border-outline-variant pt-4">
            <FormFieldBuilder fields={formFields} onChange={setFormFields} />
          </div>

          {/* Steps */}
          <div className="border-t border-outline-variant pt-4">
            <StepBuilder steps={steps} onChange={setSteps} />
          </div>
        </div>
      </Modal.Body>
      <Modal.Actions>
        <Button variant="ghost" onClick={onClose}>Cancel</Button>
        <Button onClick={handleSubmit} disabled={!isValid || isMutating}>
          {isMutating ? <Spinner size="sm" /> : null}
          {isEdit ? 'Save Template' : 'Create Template'}
        </Button>
      </Modal.Actions>
    </Modal>
  )
}
