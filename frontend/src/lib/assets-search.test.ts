import { describe, expect, it } from 'vitest'
import {
  assetSearch, composeSearch, filterSearch, legacyAssetsRedirect, pageSearch, requestSearch, showSearch, tabOf, tabSearch,
  typeSearch, validateAssetsSearch,
} from './assets-search'

const A = '77777777-aaaa-4bbb-8ccc-000000000001'

describe('validateAssetsSearch', () => {
  it('keeps what the screen reads', () => {
    expect(validateAssetsSearch({ ws: 'w1', section: 'list', asset: A, state: 'available', kind: 't1', q: 'dell', page: 3 })).toEqual({
      ws: 'w1', section: 'list', asset: A, state: 'available', kind: 't1', q: 'dell', page: 3,
    })
    expect(validateAssetsSearch({ section: 'requests', request: 'r1', show: 'approved', compose: 'request' })).toEqual({
      section: 'requests', request: 'r1', show: 'approved', compose: 'request',
    })
    expect(validateAssetsSearch({ section: 'types', type: 't1' })).toEqual({ section: 'types', type: 't1' })
  })

  it('drops anything unknown, so a hand-edited link opens the overview', () => {
    expect(validateAssetsSearch({ section: 'overview', state: 'in_use', show: 'whatever', page: -2, compose: 'x', asset: 9 })).toEqual({})
    expect(validateAssetsSearch({ page: 1 })).toEqual({})
    expect(validateAssetsSearch({ page: '4' })).toEqual({ page: 4 })
    expect(validateAssetsSearch({ q: '   ' })).toEqual({})
  })

  it('reads the first tab when the link names none', () => {
    expect(tabOf({})).toBe('overview')
    expect(tabOf({ section: 'types' })).toBe('types')
  })
})

describe('search builders', () => {
  it('a tab starts clean: no open panel, no filters, workspace kept', () => {
    expect(tabSearch({ ws: 'w', section: 'list', asset: A, state: 'available', q: 'x', page: 2 }, 'requests')).toEqual({ ws: 'w', section: 'requests' })
    expect(tabSearch({ ws: 'w', section: 'list' }, 'overview')).toEqual({ ws: 'w' })
  })

  it('opening an asset goes to the list and keeps its filters; closing keeps them too', () => {
    expect(assetSearch({ ws: 'w', state: 'available' }, A)).toEqual({ ws: 'w', section: 'list', state: 'available', asset: A })
    expect(assetSearch({ ws: 'w', section: 'list', asset: A, state: 'available' })).toEqual({ ws: 'w', section: 'list', state: 'available' })
  })

  it('a request, a type and the new-request form each close the others', () => {
    expect(requestSearch({ section: 'requests', compose: 'request' }, 'r1')).toEqual({ section: 'requests', request: 'r1' })
    expect(typeSearch({ section: 'types', type: 'a' }, 'b')).toEqual({ section: 'types', type: 'b' })
    expect(composeSearch({ section: 'requests', request: 'r1', show: 'approved' })).toEqual({ section: 'requests', show: 'approved', compose: 'request' })
    expect(composeSearch({ section: 'requests', compose: 'request' }, false)).toEqual({ section: 'requests' })
  })

  it('changing a filter goes back to the first page and keeps the panel', () => {
    expect(filterSearch({ section: 'list', page: 4, asset: A }, { state: 'assigned' })).toEqual({ section: 'list', asset: A, state: 'assigned' })
    expect(filterSearch({ section: 'list', state: 'assigned', kind: 't', q: 'a' }, { state: undefined, kind: undefined, q: '' })).toEqual({ section: 'list' })
  })

  it('page 1 is the absence of a page', () => {
    expect(pageSearch({ section: 'list' }, 3)).toEqual({ section: 'list', page: 3 })
    expect(pageSearch({ section: 'list', page: 3 }, 1)).toEqual({ section: 'list' })
  })

  it('the request filter "Đang chờ" is the default and is not written', () => {
    expect(showSearch({ section: 'requests', request: 'r1' }, 'approved')).toEqual({ section: 'requests', show: 'approved' })
    expect(showSearch({ section: 'requests', show: 'approved' }, 'pending')).toEqual({ section: 'requests' })
  })
})

describe('old /assets URLs', () => {
  it('map to the tab that replaced them', () => {
    expect(legacyAssetsRedirect('dashboard', {})).toEqual({})
    expect(legacyAssetsRedirect('list', { ws: 'w' })).toEqual({ ws: 'w', section: 'list' })
    expect(legacyAssetsRedirect('requests', {})).toEqual({ section: 'requests' })
    expect(legacyAssetsRedirect('types', {})).toEqual({ section: 'types' })
    expect(legacyAssetsRedirect('new-request', { ws: 'w' })).toEqual({ ws: 'w', section: 'requests', compose: 'request' })
    expect(legacyAssetsRedirect({ asset: A }, {})).toEqual({ section: 'list', asset: A })
  })
})
