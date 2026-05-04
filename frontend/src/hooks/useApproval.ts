import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { approvalApi, type RequestWithAssignment } from '../api/approval'
import { queryClient } from '../lib/query-client'
import type { CreateTemplateInput, UpdateTemplateInput, CreateRequestInput } from '../api/approval'

// --- Query Options ---

export const approvalPendingOptions = () =>
  queryOptions({
    queryKey: ['approval', 'pending'],
    queryFn: () => approvalApi.getPending(),
  })

export const approvalHistoryOptions = (cursor?: string) =>
  queryOptions({
    queryKey: ['approval', 'history', cursor || 'initial'],
    queryFn: () => approvalApi.getHistory(cursor),
  })

export const approvalMyRequestsOptions = (cursor?: string) =>
  queryOptions({
    queryKey: ['approval', 'my-requests', cursor || 'initial'],
    queryFn: () => approvalApi.getMyRequests(cursor),
  })

export const approvalDeptOptions = (cursor?: string) =>
  queryOptions({
    queryKey: ['approval', 'department', cursor || 'initial'],
    queryFn: () => approvalApi.getDepartmentRequests(cursor),
  })

export const approvalAuditOptions = (requestId: string) =>
  queryOptions({
    queryKey: ['approval', 'audit', requestId],
    queryFn: () => approvalApi.getAuditLog(requestId),
    enabled: !!requestId,
  })

export const approvalTemplatesOptions = (entityType?: string, activeOnly = true) =>
  queryOptions({
    queryKey: ['approval', 'templates', entityType || 'all', activeOnly],
    queryFn: () => approvalApi.listTemplates(entityType, activeOnly),
  })

export const approvalTemplateOptions = (id: string) =>
  queryOptions({
    queryKey: ['approval', 'template', id],
    queryFn: () => approvalApi.getTemplate(id),
    enabled: !!id,
  })

// --- Query Hooks ---

/** Loads all pending approvals assigned to current user. */
export function useApprovalPending() {
  return useQuery(approvalPendingOptions())
}

/** Loads approval history with cursor-based pagination. */
export function useApprovalHistory(cursor?: string) {
  return useQuery(approvalHistoryOptions(cursor))
}

/** Loads requests created by current user. */
export function useApprovalMyRequests(cursor?: string) {
  return useQuery(approvalMyRequestsOptions(cursor))
}

/** Loads department-scoped requests. */
export function useApprovalDepartment(cursor?: string) {
  return useQuery(approvalDeptOptions(cursor))
}

/** Loads audit log for a specific request. */
export function useApprovalAudit(requestId: string) {
  return useQuery(approvalAuditOptions(requestId))
}

/** Loads all approval templates (optionally filtered by entity type). */
export function useApprovalTemplates(entityType?: string, activeOnly = true) {
  return useQuery(approvalTemplatesOptions(entityType, activeOnly))
}

/** Loads a single approval template by ID. */
export function useApprovalTemplate(id: string) {
  return useQuery(approvalTemplateOptions(id))
}

// --- Mutation Hooks ---

/** Optimistic helper: removes request(s) from pending list cache. */
function optimisticRemoveFromPending(requestIds: string[]) {
  const key = ['approval', 'pending']
  const prev = queryClient.getQueryData<{ items: RequestWithAssignment[]; total: number }>(key)
  if (prev) {
    const idSet = new Set(requestIds)
    queryClient.setQueryData(key, {
      ...prev,
      items: prev.items.filter((r) => !idSet.has(r.request.id)),
      total: Math.max(0, prev.total - requestIds.length),
    })
  }
  return prev
}

/** Approve a single pending request with optimistic removal. */
export function useApprove() {
  return useMutation({
    mutationFn: ({ requestId, comment }: { requestId: string; comment?: string }) =>
      approvalApi.approve(requestId, comment),
    onMutate: async ({ requestId }) => {
      await queryClient.cancelQueries({ queryKey: ['approval', 'pending'] })
      return { prev: optimisticRemoveFromPending([requestId]) }
    },
    onError: (_err, _vars, context) => {
      if (context?.prev) queryClient.setQueryData(['approval', 'pending'], context.prev)
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['approval', 'history'] })
    },
  })
}

/** Reject a single pending request with optimistic removal. */
export function useReject() {
  return useMutation({
    mutationFn: ({ requestId, comment }: { requestId: string; comment: string }) =>
      approvalApi.reject(requestId, comment),
    onMutate: async ({ requestId }) => {
      await queryClient.cancelQueries({ queryKey: ['approval', 'pending'] })
      return { prev: optimisticRemoveFromPending([requestId]) }
    },
    onError: (_err, _vars, context) => {
      if (context?.prev) queryClient.setQueryData(['approval', 'pending'], context.prev)
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['approval', 'history'] })
    },
  })
}

/** Batch approve multiple requests with optimistic removal. */
export function useBatchApprove() {
  return useMutation({
    mutationFn: ({ requestIds, comment }: { requestIds: string[]; comment?: string }) =>
      approvalApi.batchApprove(requestIds, comment),
    onMutate: async ({ requestIds }) => {
      await queryClient.cancelQueries({ queryKey: ['approval', 'pending'] })
      return { prev: optimisticRemoveFromPending(requestIds) }
    },
    onError: (_err, _vars, context) => {
      if (context?.prev) queryClient.setQueryData(['approval', 'pending'], context.prev)
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['approval', 'history'] })
    },
  })
}

/** Create a new approval template (admin-only, invalidation pattern). */
export function useCreateTemplate() {
  return useMutation({
    mutationFn: (input: CreateTemplateInput) =>
      approvalApi.createTemplate(input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['approval', 'templates'] }),
  })
}

/** Update an existing approval template (admin-only, invalidation pattern). */
export function useUpdateTemplate() {
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateTemplateInput }) =>
      approvalApi.updateTemplate(id, input),
    onSuccess: (_, vars) => {
      queryClient.invalidateQueries({ queryKey: ['approval', 'templates'] })
      queryClient.invalidateQueries({ queryKey: ['approval', 'template', vars.id] })
    },
  })
}

/** Create a new approval request with optimistic insert into my-requests. */
export function useCreateRequest() {
  return useMutation({
    mutationFn: (input: CreateRequestInput) =>
      approvalApi.createRequest(input),
    onSuccess: (newRequest) => {
      // Insert newly created request into my-requests cache
      const key = ['approval', 'my-requests', 'initial']
      const prev = queryClient.getQueryData<{ items: ApprovalRequest[]; next_cursor: string }>(key)
      if (prev) {
        queryClient.setQueryData(key, {
          ...prev,
          items: [newRequest, ...prev.items],
        })
      } else {
        queryClient.invalidateQueries({ queryKey: ['approval', 'my-requests'] })
      }
    },
  })
}
