/**
 * Approval query keys. Approval data lives in a per-tenant schema and the
 * session is tenant-scoped, so these carry no workspace id.
 *
 * The paged lists (history, my requests, department) are infinite queries: one
 * key per list, every loaded page under it. Their `*All` prefixes exist for the
 * mutations that must refresh them whatever page the user has reached.
 */
export const approvalKeys = {
  all: () => ['approval'] as const,
  pending: () => ['approval', 'pending'] as const,
  historyAll: () => ['approval', 'history'] as const,
  history: () => ['approval', 'history', 'pages'] as const,
  myRequestsAll: () => ['approval', 'my-requests'] as const,
  myRequests: () => ['approval', 'my-requests', 'pages'] as const,
  departmentAll: () => ['approval', 'department'] as const,
  department: () => ['approval', 'department', 'pages'] as const,
  /** One request opened for reading: its chain and every approver. */
  request: (requestId: string) => ['approval', 'request', requestId] as const,
  requestsAll: () => ['approval', 'request'] as const,
  auditAll: () => ['approval', 'audit'] as const,
  audit: (requestId: string) => ['approval', 'audit', requestId] as const,
  /** What the signed-in user may do here (e.g. manage templates). */
  permissions: () => ['approval', 'permissions'] as const,
  templatesAll: () => ['approval', 'templates'] as const,
  templates: (entityType?: string, activeOnly = true) =>
    ['approval', 'templates', entityType || 'all', activeOnly] as const,
  template: (id: string) => ['approval', 'template', id] as const,
}
