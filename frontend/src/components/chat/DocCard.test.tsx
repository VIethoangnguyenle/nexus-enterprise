import { screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import { documentApi } from '../../api/documents'
import { DocCard } from './DocCard'
import { renderWithClient, resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { DOC_ID, textDoc } from '../../test/chat-cards-fixtures'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/documents', async (orig) => {
  const real = await orig<typeof import('../../api/documents')>()
  return { ...real, documentApi: { ...real.documentApi, get: vi.fn() } }
})

beforeEach(() => vi.clearAllMocks())
afterEach(resetClient)

describe('DocCard', () => {
  it('shows the title, the owner by name and a way to open it', async () => {
    vi.mocked(documentApi.get).mockResolvedValue(textDoc())
    renderWithClient(<DocCard docId={DOC_ID} />)
    expect(await screen.findByText('Biên bản họp với ngân hàng 14/10')).toBeInTheDocument()
    expect(screen.getByText(/Nguyễn Thu Lan/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Mở' })).toHaveAttribute('href', `/documents/${DOC_ID}`)
    expectNoIds(document.body.querySelector('[data-card]') as HTMLElement)
  })

  it.each([403, 404])('on a %i says the document cannot be viewed and leaks no title', async (status) => {
    vi.mocked(documentApi.get).mockRejectedValue(new ApiError('nope', status))
    renderWithClient(<DocCard docId={DOC_ID} />)
    expect(await screen.findByText('Tài liệu không xem được')).toBeInTheDocument()
    expect(screen.queryByRole('link')).toBeNull()
    expect(screen.queryByText(/Biên bản/)).toBeNull()
  })

  it('says the card could not load, apart from "cannot view", on a server error', async () => {
    vi.mocked(documentApi.get).mockRejectedValue(new ApiError('boom', 500))
    renderWithClient(<DocCard docId={DOC_ID} />)
    expect(await screen.findByText('Chưa tải được tài liệu')).toBeInTheDocument()
    expect(screen.queryByText('Tài liệu không xem được')).toBeNull()
  })
})
