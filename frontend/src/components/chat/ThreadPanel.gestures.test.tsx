import { screen, waitFor } from '@testing-library/react'
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { buildDirectory } from '../../lib/people'
import { CONTACTS, MESSAGES, MSG, CH, ME, fixtureApi } from '../../test/chat-fixtures'
import { apiFetch } from '../../api/client'
import { queryClient } from '../../lib/query-client'
import { keys } from '../../hooks/keys'
import { swipe } from '../../test/gesture-events'
import { threadScope, useDraftsStore } from '../../stores/drafts.store'
import { ThreadPanel } from './ThreadPanel'

vi.mock('../../api/client', async (orig) => {
  const { fixtureApi } = await import('../../test/chat-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(fixtureApi) }
})
vi.mock('../../hooks/useActiveWorkspace', () => ({ useActiveWorkspace: () => ({ workspaceId: 'ws-test', workspaces: [], isLoading: false }) }))

const people = buildDirectory(CONTACTS)

beforeAll(() => {
  const none = () => ({ length: 0, item: () => null, [Symbol.iterator]: function* () {} }) as unknown as DOMRectList
  Range.prototype.getClientRects = none
  Range.prototype.getBoundingClientRect = () => new DOMRect()
  window.scrollTo = () => {}
})
beforeEach(() => {
  useDraftsStore.getState().reset()
  vi.mocked(apiFetch).mockImplementation(fixtureApi as never)
  queryClient.setQueryData(keys.messaging.messages(CH.doisoat), { messages: MESSAGES, has_more: false })
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
})
afterEach(() => {
  vi.unstubAllGlobals()
  resetClient()
})

const right = [{ x: 120, t: 1000 }, { x: 135, t: 1020 }, { x: 210, t: 1080 }]
const left = [{ x: 300, t: 1000 }, { x: 285, t: 1020 }, { x: 200, t: 1080 }]
const quoteHere = () => useDraftsStore.getState().drafts[threadScope(MSG.t1)]?.quote

function open() {
  renderWithClient(
    <ThreadPanel channelId={CH.doisoat} workspaceId="ws-test" messageId={MSG.t1} spaceName="Đối soát giao dịch" people={people} me={ME.id} onClose={vi.fn()} />,
  )
}

describe('ThreadPanel on a phone: swipes', () => {
  it('quotes a reply into the thread’s own reply box on a swipe right', async () => {
    open()
    const reply = await screen.findByText(/1\.284\.500/)
    swipe(reply, right)
    await waitFor(() => expect(quoteHere()?.id).toBe(MSG.r1))
  })

  it('quotes the root into the thread too, since it is answered there', async () => {
    open()
    const root = await screen.findByText(/lệch 3 giao dịch/)
    swipe(root, right)
    await waitFor(() => expect(quoteHere()?.id).toBe(MSG.t1))
    expect(useDraftsStore.getState().drafts[CH.doisoat]?.quote).toBeUndefined()
  })

  it('has no thread to open from inside a thread: a swipe left does nothing', async () => {
    open()
    const reply = await screen.findByText(/1\.284\.500/)
    swipe(reply, left)
    expect(quoteHere()).toBeUndefined()
    expect(screen.getByRole('dialog', { name: 'Thread' })).toBeInTheDocument()
  })
})
