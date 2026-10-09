/**
 * Approval query keys. Approval data lives in a per-tenant schema and the
 * session is tenant-scoped, so these carry no workspace id.
 */
export const approvalKeys = {
  all: () => ['approval'] as const,
  pending: () => ['approval', 'pending'] as const,
  historyAll: () => ['approval', 'history'] as const,
  history: (cursor?: string) => ['approval', 'history', cursor || 'initial'] as const,
  myRequestsAll: () => ['approval', 'my-requests'] as const,
  myRequests: (cursor?: string) => ['approval', 'my-requests', cursor || 'initial'] as const,
  department: (cursor?: string) => ['approval', 'department', cursor || 'initial'] as const,
  audit: (requestId: string) => ['approval', 'audit', requestId] as const,
  templatesAll: () => ['approval', 'templates'] as const,
  templates: (entityType?: string, activeOnly = true) =>
    ['approval', 'templates', entityType || 'all', activeOnly] as const,
  template: (id: string) => ['approval', 'template', id] as const,
}
