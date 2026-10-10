import { beforeEach, describe, expect, it, vi } from 'vitest'

import { approvalApi } from './approval'

/**
 * The approval REST handlers bind JSON into structs and ignore any field they
 * do not know, so a misspelt body is not an error: it is a request that does
 * something else. These pin the field names to the handler's request structs
 * (backend/services/approval/internal/rest/handler.go).
 */
const apiFetch = vi.fn()
vi.mock('./client', () => ({ apiFetch: (...args: unknown[]) => apiFetch(...args) }))

beforeEach(() => apiFetch.mockReset().mockResolvedValue({}))

const lastCall = () => {
  const [url, init] = apiFetch.mock.calls[apiFetch.mock.calls.length - 1] as [string, RequestInit | undefined]
  return { url: `/api${url}`, method: init?.method ?? 'GET', body: init?.body ? JSON.parse(String(init.body)) : undefined }
}

describe('approval request bodies', () => {
  it('approve and reject send request_id and comment', async () => {
    await approvalApi.approve('r1')
    expect(lastCall()).toMatchObject({ url: '/api/approval/approve', method: 'POST', body: { request_id: 'r1', comment: '' } })
    await approvalApi.reject('r1', 'Thiếu hoá đơn')
    expect(lastCall()).toMatchObject({ url: '/api/approval/reject', method: 'POST', body: { request_id: 'r1', comment: 'Thiếu hoá đơn' } })
  })

  it('batch approve sends request_ids', async () => {
    await approvalApi.batchApprove(['a', 'b'])
    expect(lastCall()).toMatchObject({ url: '/api/approval/batch-approve', body: { request_ids: ['a', 'b'], comment: '' } })
  })

  it('create request names the chosen template and sends no entity identity of its own', async () => {
    await approvalApi.createRequest({ template_id: 't1', form_data_json: '{"a":"b"}' })
    const { url, method, body } = lastCall()
    expect([url, method]).toEqual(['/api/approval/requests', 'POST'])
    expect(body).toEqual({ template_id: 't1', form_data_json: '{"a":"b"}' })
  })

  it('update template carries is_active and priority, which the server writes as given', async () => {
    await approvalApi.updateTemplate('t1', { name: 'N', is_active: false, priority: 4, steps: [], expected_updated_at: '2026-10-01T08:00:00Z' })
    expect(lastCall()).toMatchObject({
      url: '/api/approval/templates/t1', method: 'PUT', body: { name: 'N', is_active: false, priority: 4, steps: [], expected_updated_at: '2026-10-01T08:00:00Z' },
    })
  })

  it('create template sends steps with the field names the handler binds', async () => {
    await approvalApi.createTemplate({
      name: 'N', entity_type: 'expense', priority: 0,
      form_fields: [{ label: 'Số tiền', field_type: 'currency', required: true, options: '', placeholder: '' }],
      steps: [{ step_order: 1, name: 'S', approver_type: 'specific_user', approver_value: 'n', required_count: 1, timeout_hours: 0 }],
    })
    const { body } = lastCall()
    expect(Object.keys(body.steps[0]).sort()).toEqual(
      ['approver_type', 'approver_value', 'name', 'required_count', 'step_order', 'timeout_hours'],
    )
    expect(Object.keys(body.form_fields[0]).sort()).toEqual(['field_type', 'label', 'options', 'placeholder', 'required'])
  })

  it('reads one request and its trail from their own paths', async () => {
    await approvalApi.getRequest('r1')
    expect(lastCall().url).toBe('/api/approval/requests/r1')
    await approvalApi.getAuditLog('r1')
    expect(lastCall().url).toBe('/api/approval/requests/r1/audit')
  })

  it('pages with cursor and limit, and lists inactive templates only when asked', async () => {
    await approvalApi.getHistory('2026-10-05T10:00:00Z')
    expect(lastCall().url).toBe('/api/approval/history?limit=20&cursor=2026-10-05T10%3A00%3A00Z')
    await approvalApi.listTemplates(undefined, false)
    expect(lastCall().url).toBe('/api/approval/templates?active_only=false')
    await approvalApi.listTemplates()
    expect(lastCall().url).toBe('/api/approval/templates?')
  })
})

describe('approval permissions', () => {
  it('asks the server what the user may do', async () => {
    await approvalApi.getPermissions()
    expect(lastCall()).toMatchObject({ url: '/api/approval/permissions', method: 'GET' })
  })
})
