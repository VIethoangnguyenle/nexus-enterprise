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
  /** An internal key (person node, role or department). Never shown; see `approver_name`. */
  approver_value: string
  /** Who `approver_value` stands for, named by the server. Absent when it cannot be resolved. */
  approver_name?: string
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
  created_by_name?: string
  created_at: string
  updated_at: string
}

export interface ApprovalAssignment {
  id: string
  step_order: number
  step_name: string
  user_node_id: string
  /** Display name of the person (or role) the assignment is for. */
  user_name?: string
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
  department_name?: string
  created_by: string
  /** Display name of the requester, named by the server. */
  created_by_name?: string
  created_at: string
  completed_at: string | null
  /** The template as it was when the request was made. Lists carry it; the detail unpacks it. */
  template_snapshot?: string
  /** Set client-side on pending/history rows: the viewer's own assignment. */
  assignments?: ApprovalAssignment[]
}

/** One request opened for reading: its chain and every approver's assignment. */
export interface ApprovalRequestDetail {
  request: ApprovalRequest
  steps: ApprovalStep[] | null
  form_fields: FormFieldDefinition[] | null
  assignments: ApprovalAssignment[] | null
  /** It is the caller's turn: directly, or through a role or department they belong to. */
  can_act: boolean
}

/** What the signed-in user may do in the approval module. */
export interface ApprovalPermissions {
  can_manage_templates: boolean
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
  /** Display name of the actor; absent for system entries. */
  actor_name?: string
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

/**
 * The server writes `is_active` and `priority` from the body as given, so an
 * update that leaves them out switches the template off and resets its
 * priority. Both are therefore required: an edit must say what it keeps.
 */
export interface UpdateTemplateInput {
  name?: string
  is_active: boolean
  priority: number
  /**
   * The template's `updated_at` as it was read. The server refuses the edit (409)
   * if the template has changed since, instead of overwriting someone else's.
   */
  expected_updated_at: string
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
  /** The template the submitter chose; the server follows it instead of matching one. */
  template_id: string
  form_data_json?: string
  /** Only for requests that must be matched on conditions rather than a chosen template. */
  entity_fields?: Record<string, string>
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

  getPermissions: () => apiFetch<ApprovalPermissions>('/approval/permissions'),

  getRequest: (requestId: string) =>
    apiFetch<ApprovalRequestDetail>(`/approval/requests/${requestId}`),

  // Audit
  getAuditLog: (requestId: string) =>
    apiFetch<{ entries: AuditEntry[] }>(
      `/approval/requests/${requestId}/audit`,
    ),
}
