import { useInfiniteQuery, useMutation, useQuery, queryOptions, type InfiniteData } from '@tanstack/react-query'
import {
  approvalApi,
  type ApprovalAssignment,
  type ApprovalRequest,
  type CreateRequestInput,
  type CreateTemplateInput,
  type RequestWithAssignment,
  type UpdateTemplateInput,
} from '../api/approval'
import { apiFetch } from '../api/client'
import { queryClient } from '../lib/query-client'
import { statusOf } from '../lib/errors'
import { keys } from './keys'

/**
 * A line of any request list. Pending and history rows come with the viewer's
 * own assignment; "my requests" and department rows do not have one.
 */
export interface ApprovalRow {
  request: ApprovalRequest
  assignment?: ApprovalAssignment
}

// --- Query options ---

export const approvalPendingOptions = () =>
  queryOptions({
    queryKey: keys.approval.pending(),
    queryFn: () => approvalApi.getPending(),
    select: (d): { rows: ApprovalRow[]; total: number } => ({
      rows: (d.items ?? []).map((r) => ({ request: r.request, assignment: r.assignment })),
      total: d.total ?? 0,
    }),
  })

export const approvalRequestOptions = (requestId: string) =>
  queryOptions({
    queryKey: keys.approval.request(requestId),
    queryFn: () => approvalApi.getRequest(requestId),
    enabled: !!requestId,
  })

export const approvalAuditOptions = (requestId: string) =>
  queryOptions({
    queryKey: keys.approval.audit(requestId),
    queryFn: () => approvalApi.getAuditLog(requestId),
    enabled: !!requestId,
    // 403 is an answer ("not yours to read"), not a hiccup: asking again won't change it.
    retry: (count, err) => statusOf(err) !== 403 && count < 1,
  })

export const approvalTemplatesOptions = (entityType?: string, activeOnly = true) =>
  queryOptions({
    queryKey: keys.approval.templates(entityType, activeOnly),
    queryFn: () => approvalApi.listTemplates(entityType, activeOnly),
  })

export const approvalTemplateOptions = (id: string) =>
  queryOptions({
    queryKey: keys.approval.template(id),
    queryFn: () => approvalApi.getTemplate(id),
    enabled: !!id,
  })

// --- Query hooks ---

/** Everything waiting on the signed-in user, with the count for the tab. */
export function useApprovalPending() {
  return useQuery(approvalPendingOptions())
}

type Page<T> = { items?: T[]; next_cursor?: string }

/** The same list read page by page: every loaded page is kept, newest request first. */
function usePagedRows<T>(
  queryKey: readonly unknown[],
  fetchPage: (cursor?: string) => Promise<Page<T>>,
  toRow: (item: T) => ApprovalRow,
  enabled: boolean,
) {
  const q = useInfiniteQuery({
    queryKey,
    queryFn: ({ pageParam }) => fetchPage(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor || undefined,
    enabled,
    select: (d: InfiniteData<Page<T>>) => d.pages.flatMap((p) => (p.items ?? []).map(toRow)),
  })
  return q
}

const wrapped = (r: RequestWithAssignment): ApprovalRow => ({ request: r.request, assignment: r.assignment })
const flat = (r: ApprovalRequest): ApprovalRow => ({ request: r })

/** Requests the user has acted on. Off until the tab is opened. */
export function useApprovalHistory(enabled = true) {
  return usePagedRows(keys.approval.history(), (c) => approvalApi.getHistory(c), wrapped, enabled)
}

/** Requests the user submitted. */
export function useApprovalMyRequests(enabled = true) {
  return usePagedRows(keys.approval.myRequests(), (c) => approvalApi.getMyRequests(c), flat, enabled)
}

/** Requests inside the scopes the user can read. */
export function useApprovalDepartment(enabled = true) {
  return usePagedRows(keys.approval.department(), (c) => approvalApi.getDepartmentRequests(c), flat, enabled)
}

/** One request with its chain; the caller must be allowed to see it (403 otherwise). */
export function useApprovalRequest(requestId: string) {
  return useQuery(approvalRequestOptions(requestId))
}

/** The audit trail of a request. 403 when the caller cannot read this request's trail. */
export function useApprovalAudit(requestId: string) {
  return useQuery(approvalAuditOptions(requestId))
}

/** What the user may do here. Unknown until it loads; callers treat unknown as no. */
export function useApprovalPermissions() {
  return useQuery({
    queryKey: keys.approval.permissions(),
    queryFn: () => approvalApi.getPermissions(),
    staleTime: 60_000,
  })
}

/** Templates, optionally of one entity type; `activeOnly = false` includes switched-off ones. */
export function useApprovalTemplates(entityType?: string, activeOnly = true, enabled = true) {
  return useQuery({ ...approvalTemplatesOptions(entityType, activeOnly), enabled })
}

export function useApprovalTemplate(id: string) {
  return useQuery(approvalTemplateOptions(id))
}

export interface WorkspaceRole {
  id: string
  name: string
  ngac_node_id: string
}

/**
 * The workspace's roles, for choosing who approves a step. Shares its cache
 * entry (and response shape) with the admin roles page.
 */
export function useWorkspaceRoles(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: keys.admin.roles(workspaceId),
    queryFn: () => apiFetch<{ roles?: WorkspaceRole[] }>(`/workspaces/${workspaceId}/roles`),
    enabled: !!workspaceId && enabled,
    select: (d) => d.roles ?? [],
  })
}

// --- Mutations ---

type PendingCache = { items: RequestWithAssignment[]; total: number }

/** Takes request(s) out of the pending list at once; returns what to put back on failure. */
function removeFromPending(requestIds: string[]) {
  const key = keys.approval.pending()
  const prev = queryClient.getQueryData<PendingCache>(key)
  if (prev) {
    const gone = new Set(requestIds)
    const items = (prev.items ?? []).filter((r) => !gone.has(r.request.id))
    queryClient.setQueryData<PendingCache>(key, { ...prev, items, total: Math.max(0, (prev.total ?? 0) - ((prev.items ?? []).length - items.length)) })
  }
  return prev
}

/** What a decision changes: lists, the opened request and its trail. Templates are untouched. */
function refreshAfterDecision() {
  for (const queryKey of [
    keys.approval.pending(),
    keys.approval.historyAll(),
    keys.approval.myRequestsAll(),
    keys.approval.departmentAll(),
    keys.approval.requestsAll(),
    keys.approval.auditAll(),
  ]) {
    void queryClient.invalidateQueries({ queryKey })
  }
}

/** Optimistic removal from the pending list, undone if the server refuses. */
function decision<V>(idsOf: (vars: V) => string[]) {
  return {
    onMutate: async (vars: V) => {
      await queryClient.cancelQueries({ queryKey: keys.approval.pending() })
      return { prev: removeFromPending(idsOf(vars)) }
    },
    onError: (_e: unknown, _v: V, ctx: { prev?: PendingCache } | undefined) => {
      if (ctx?.prev) queryClient.setQueryData(keys.approval.pending(), ctx.prev)
    },
    onSettled: refreshAfterDecision,
  }
}

/** Approve one request. It leaves the pending list at once and comes back if the server refuses. */
export function useApprove() {
  return useMutation({
    meta: { action: 'duyệt đề nghị' },
    mutationFn: ({ requestId, comment }: { requestId: string; comment?: string }) =>
      approvalApi.approve(requestId, comment),
    ...decision<{ requestId: string; comment?: string }>((v) => [v.requestId]),
  })
}

/** Return one request with the reason. The reason is required by the screen; the server stores it. */
export function useReject() {
  return useMutation({
    meta: { action: 'trả lại đề nghị' },
    mutationFn: ({ requestId, comment }: { requestId: string; comment: string }) =>
      approvalApi.reject(requestId, comment),
    ...decision<{ requestId: string; comment: string }>((v) => [v.requestId]),
  })
}

/** Approve several at once. The server skips any it cannot approve and says which it did. */
export function useBatchApprove() {
  return useMutation({
    meta: { action: 'duyệt các đề nghị đã chọn' },
    mutationFn: ({ requestIds, comment }: { requestIds: string[]; comment?: string }) =>
      approvalApi.batchApprove(requestIds, comment),
    ...decision<{ requestIds: string[]; comment?: string }>((v) => v.requestIds),
  })
}

export function useCreateTemplate() {
  return useMutation({
    meta: { action: 'lưu mẫu phê duyệt' },
    mutationFn: (input: CreateTemplateInput) => approvalApi.createTemplate(input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.approval.templatesAll() }),
  })
}

export function useUpdateTemplate() {
  return useMutation({
    meta: { action: 'lưu mẫu phê duyệt' },
    mutationFn: ({ id, input }: { id: string; input: UpdateTemplateInput }) => approvalApi.updateTemplate(id, input),
    onSuccess: (_, vars) => {
      void queryClient.invalidateQueries({ queryKey: keys.approval.templatesAll() })
      void queryClient.invalidateQueries({ queryKey: keys.approval.template(vars.id) })
    },
  })
}

/** Send a new request. The dialog reports a failure beside the form, so no second toast. */
export function useCreateRequest() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (input: CreateRequestInput) => approvalApi.createRequest(input),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: keys.approval.myRequestsAll() })
      void queryClient.invalidateQueries({ queryKey: keys.approval.departmentAll() })
    },
  })
}
