import { describe, expect, it } from 'vitest'
import type { ApprovalRequest } from '../../api/approval'
import type { PeopleDirectory } from '../people'
import { matchesRequest } from './approval'

const req = (over: Partial<ApprovalRequest>): ApprovalRequest => ({
  id: 'r1', entity_type: 'expense', template_name: 'Tạm ứng công tác phí tháng 10', created_by: 'n1', created_by_name: 'Phạm Hải Yến',
  ...over,
} as ApprovalRequest)

const NO_ONE = { byNodeId: new Map(), byUserId: new Map(), list: [] } as unknown as PeopleDirectory

describe('matchesRequest', () => {
  it('finds a request by its title, ignoring case and accents', () => {
    expect(matchesRequest(req({}), 'cong tac', NO_ONE)).toBe(true)
    expect(matchesRequest(req({}), 'CÔNG TÁC', NO_ONE)).toBe(true)
  })

  it('finds it by who sent it and by its kind', () => {
    expect(matchesRequest(req({}), 'hai yen', NO_ONE)).toBe(true)
    expect(matchesRequest(req({}), 'chi phí', NO_ONE)).toBe(true)
  })

  it('needs every word, in any field', () => {
    expect(matchesRequest(req({}), 'yen tạm ứng', NO_ONE)).toBe(true)
    expect(matchesRequest(req({}), 'yen hợp đồng', NO_ONE)).toBe(false)
  })

  it('finds nothing for a blank search', () => {
    expect(matchesRequest(req({}), '  ', NO_ONE)).toBe(false)
  })
})
