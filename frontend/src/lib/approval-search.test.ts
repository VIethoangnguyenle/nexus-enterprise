import { describe, expect, it } from 'vitest'
import { querySearch, tabSearch, validateApprovalSearch } from './approval-search'

describe('the search of Phê duyệt in the URL', () => {
  it('keeps the words looked for, and drops a blank search', () => {
    expect(validateApprovalSearch({ q: 'công tác' }).q).toBe('công tác')
    expect(validateApprovalSearch({ q: '   ' }).q).toBeUndefined()
    expect(validateApprovalSearch({ q: 3 }).q).toBeUndefined()
  })

  it('sets and clears the words, keeping the rest', () => {
    expect(querySearch({ ws: 'w', tab: 'mine' }, 'vpp')).toEqual({ ws: 'w', tab: 'mine', q: 'vpp' })
    expect(querySearch({ ws: 'w', q: 'vpp' }, '  ')).toEqual({ ws: 'w' })
  })

  it('is cleared by moving to another tab', () => {
    expect(tabSearch({ ws: 'w', q: 'công tác' }, 'history')).toEqual({ ws: 'w', tab: 'history' })
  })
})
