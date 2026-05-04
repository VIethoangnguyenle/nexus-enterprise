import { useState, useMemo, useCallback } from 'react'
import { Modal } from '../composites'
import { Button, Select, Spinner, Text } from '../primitives'
import { DynamicFormRenderer } from './DynamicFormRenderer'
import { useApprovalTemplates, useApprovalTemplate, useCreateRequest } from '../../hooks/useApproval'
import type { FormFieldDefinition } from '../../api/approval'

interface CreateRequestModalProps {
  onClose: () => void
  defaultScopeOaId: string
  defaultDepartmentId: string
}

/** Modal for creating a new approval request — template selector + dynamic form. */
export function CreateRequestModal({
  onClose,
  defaultScopeOaId,
  defaultDepartmentId,
}: CreateRequestModalProps) {
  const { data: templatesData, isLoading: loadingTemplates } = useApprovalTemplates()
  const createMut = useCreateRequest()

  const templates = templatesData?.templates || []
  const [selectedTemplateId, setSelectedTemplateId] = useState('')
  const [formValues, setFormValues] = useState<Record<string, string>>({})

  // Fetch full template details (with steps) when a template is selected
  const { data: selectedTemplate } = useApprovalTemplate(selectedTemplateId)

  const formFields: FormFieldDefinition[] = useMemo(
    () => selectedTemplate?.form_fields || [],
    [selectedTemplate],
  )

  const handleTemplateChange = useCallback((id: string) => {
    setSelectedTemplateId(id)
    setFormValues({})
  }, [])

  const handleFieldChange = useCallback((label: string, value: string) => {
    setFormValues((prev) => ({ ...prev, [label]: value }))
  }, [])

  const isValid = useMemo(() => {
    if (!selectedTemplate) return false
    return formFields
      .filter((f) => f.required)
      .every((f) => (formValues[f.label] || '').trim() !== '')
  }, [selectedTemplate, formFields, formValues])

  const handleSubmit = useCallback(() => {
    if (!selectedTemplate || !isValid) return

    createMut.mutate(
      {
        entity_type: selectedTemplate.entity_type,
        entity_id: crypto.randomUUID(),
        form_data_json: JSON.stringify(formValues),
        scope_oa_id: defaultScopeOaId,
        department_id: defaultDepartmentId,
      },
      { onSuccess: onClose },
    )
  }, [selectedTemplate, isValid, formValues, defaultScopeOaId, defaultDepartmentId, createMut, onClose])

  return (
    <Modal onClose={onClose} size="lg">
      <Modal.Header onClose={onClose}>New Approval Request</Modal.Header>
      <Modal.Body>
        <div className="space-y-5">
          {/* Template selector */}
          <div className="flex flex-col gap-1">
            <label className="text-caption-ui text-on-surface-variant">
              Template Type <span className="text-danger">*</span>
            </label>
            {loadingTemplates ? (
              <div className="flex items-center gap-2 py-2">
                <Spinner size="sm" />
                <Text variant="caption" muted>Loading templates...</Text>
              </div>
            ) : (
              <Select
                value={selectedTemplateId}
                onChange={(e) => handleTemplateChange(e.target.value)}
              >
                <option value="">Select a template...</option>
                {templates.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.name} — {t.entity_type}
                  </option>
                ))}
              </Select>
            )}
          </div>

          {/* Dynamic form */}
          {selectedTemplate && formFields.length > 0 && (
            <div className="border-t border-outline-variant pt-4">
              <Text variant="caption" muted className="uppercase tracking-wider mb-3 block">
                Request Details
              </Text>
              <DynamicFormRenderer
                fields={formFields}
                values={formValues}
                onChange={handleFieldChange}
              />
            </div>
          )}

          {/* Approval flow preview */}
          {selectedTemplate?.steps && selectedTemplate.steps.length > 0 && (
            <div className="border-t border-outline-variant pt-4">
              <Text variant="caption" muted className="uppercase tracking-wider mb-3 block">
                Approval Flow
              </Text>
              <div className="flex items-center gap-2 flex-wrap">
                {selectedTemplate.steps.map((step, i) => (
                  <div key={step.id || i} className="flex items-center gap-2">
                    <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-surface-container border border-outline-variant">
                      <span className="w-5 h-5 rounded-full bg-primary text-white flex items-center justify-center text-xs font-medium">
                        {step.step_order}
                      </span>
                      <Text variant="caption">{step.name}</Text>
                    </div>
                    {i < (selectedTemplate.steps?.length || 0) - 1 && (
                      <span className="text-outline">→</span>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </Modal.Body>
      <Modal.Actions>
        <Button variant="ghost" onClick={onClose}>Cancel</Button>
        <Button
          onClick={handleSubmit}
          disabled={!isValid || createMut.isPending}
        >
          {createMut.isPending ? <Spinner size="sm" /> : null}
          Submit Request
        </Button>
      </Modal.Actions>
    </Modal>
  )
}
