import { apiFetch } from './client'

// --- Types matching backend REST responses ---

export interface FormFieldDefinition {
  label: string
  field_type: 'text' | 'number' | 'currency' | 'date' | 'select' | 'textarea'
  required: boolean
  options?: string
  field_order: number
  placeholder?: string
}

export interface ApprovalStep {
  id: string
  step_order: number
  name: string
  approver_type: string
  approver_value: string
  required_count: number
  timeout_hours: number
}

export interface ApprovalCondition {
  id: string
  field: string
  operator: string
  value: string
}

export interface ApprovalTemplate {
  id: string
  name: string
  entity_type: string
  is_active: boolean
  priority: number
  form_fields: FormFieldDefinition[] | null
  conditions: ApprovalCondition[] | null
  steps: ApprovalStep[] | null
  step_count?: number
  condition_count?: number
  created_by: string
  created_at: string
  updated_at: string
}

export interface ApprovalAssignment {
  id: string
  step_order: number
  step_name: string
  user_node_id: string
  grant_source: string
  status: 'pending' | 'approved' | 'rejected' | 'skipped' | 'revoked'
  acted_at: string | null
  comment: string
}

export interface ApprovalRequest {
  id: string
  entity_type: string
  entity_id: string
  template_id: string
  template_name: string
  form_data_json: string | null
  current_step: number
  status: 'pending' | 'approved' | 'rejected' | 'cancelled'
  scope_oa_id: string
  department_id: string
  created_by: string
  created_at: string
  completed_at: string | null
  assignments?: ApprovalAssignment[]
}

/** Pending/History items pair a request with the viewer's specific assignment. */
export interface RequestWithAssignment {
  request: ApprovalRequest
  assignment: ApprovalAssignment
}

export interface AuditEntry {
  id: string
  request_id: string
  action: string
  actor_node_id: string
  step_order: number
  detail_json: string
  ip_address: string
  created_at: string
}

// --- Input types for mutations ---

export interface CreateTemplateInput {
  name: string
  entity_type: string
  priority?: number
  form_fields?: Omit<FormFieldDefinition, 'field_order'>[]
  steps: {
    step_order: number
    name: string
    approver_type: string
    approver_value: string
    required_count?: number
    timeout_hours?: number
  }[]
  conditions?: {
    field: string
    operator: string
    value: string
  }[]
}

export interface UpdateTemplateInput {
  name?: string
  is_active?: boolean
  priority?: number
  form_fields?: Omit<FormFieldDefinition, 'field_order'>[]
  steps?: {
    step_order: number
    name: string
    approver_type: string
    approver_value: string
    required_count?: number
    timeout_hours?: number
  }[]
  conditions?: {
    field: string
    operator: string
    value: string
  }[]
}

export interface CreateRequestInput {
  entity_type: string
  entity_id: string
  entity_fields?: Record<string, string>
  form_data_json?: string
  scope_oa_id: string
  department_id: string
}

// --- API ---

export const approvalApi = {
  // Templates
  listTemplates: (entityType?: string, activeOnly = true) => {
    const params = new URLSearchParams()
    if (entityType) params.set('entity_type', entityType)
    if (!activeOnly) params.set('active_only', 'false')
    return apiFetch<{ templates: ApprovalTemplate[] }>(
      `/approval/templates?${params}`,
    )
  },

  getTemplate: (id: string) =>
    apiFetch<ApprovalTemplate>(`/approval/templates/${id}`),

  createTemplate: (input: CreateTemplateInput) =>
    apiFetch<ApprovalTemplate>('/approval/templates', {
      method: 'POST',
      body: JSON.stringify(input),
    }),

  updateTemplate: (id: string, input: UpdateTemplateInput) =>
    apiFetch<ApprovalTemplate>(`/approval/templates/${id}`, {
      method: 'PUT',
      body: JSON.stringify(input),
    }),

  // Query tabs
  getPending: () =>
    apiFetch<{ items: RequestWithAssignment[]; total: number }>('/approval/pending'),

  getHistory: (cursor?: string, limit = 20) => {
    const params = new URLSearchParams({ limit: String(limit) })
    if (cursor) params.set('cursor', cursor)
    return apiFetch<{ items: RequestWithAssignment[]; next_cursor: string }>(
      `/approval/history?${params}`,
    )
  },

  getMyRequests: (cursor?: string, limit = 20) => {
    const params = new URLSearchParams({ limit: String(limit) })
    if (cursor) params.set('cursor', cursor)
    return apiFetch<{ items: ApprovalRequest[]; next_cursor: string }>(
      `/approval/my-requests?${params}`,
    )
  },

  getDepartmentRequests: (cursor?: string, limit = 20) => {
    const params = new URLSearchParams({ limit: String(limit) })
    if (cursor) params.set('cursor', cursor)
    return apiFetch<{ items: ApprovalRequest[]; next_cursor: string }>(
      `/approval/department-requests?${params}`,
    )
  },

  // Lifecycle
  createRequest: (input: CreateRequestInput) =>
    apiFetch<ApprovalRequest>('/approval/requests', {
      method: 'POST',
      body: JSON.stringify(input),
    }),

  approve: (requestId: string, comment?: string) =>
    apiFetch<{ status: string }>('/approval/approve', {
      method: 'POST',
      body: JSON.stringify({ request_id: requestId, comment: comment || '' }),
    }),

  reject: (requestId: string, comment: string) =>
    apiFetch<{ status: string }>('/approval/reject', {
      method: 'POST',
      body: JSON.stringify({ request_id: requestId, comment }),
    }),

  batchApprove: (requestIds: string[], comment?: string) =>
    apiFetch<{ approved_count: number; approved_ids: string[] }>(
      '/approval/batch-approve',
      {
        method: 'POST',
        body: JSON.stringify({ request_ids: requestIds, comment: comment || '' }),
      },
    ),

  // Audit
  getAuditLog: (requestId: string) =>
    apiFetch<{ entries: AuditEntry[] }>(
      `/approval/requests/${requestId}/audit`,
    ),
}
