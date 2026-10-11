import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { validateChannelSearch, ChannelRoute, Route } from './_workspace/channels.$channelId'

let currentSearch: { ws?: string; thread?: string } = {}

vi.mock('@tanstack/react-router', async () => {
  const actual = await vi.importActual('@tanstack/react-router')
  return {
    ...actual,
    useNavigate: () => vi.fn(),
  }
})

vi.mock('../components/spaces/SpaceView', () => ({
  SpaceView: ({ channelId }: { channelId: string }) => (
    <div data-testid="space-view" data-channel-id={channelId}>
      SpaceView Content
    </div>
  ),
}))

vi.mock('../lib/workspace', () => ({
  validateWorkspaceSearch: (search: Record<string, unknown>) => {
    const out: Record<string, unknown> = {}
    if (typeof search.ws === 'string' && search.ws) out.ws = search.ws
    return out
  },
}))

describe('channels.$channelId route & ?thread= handling', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    currentSearch = {}
    vi.spyOn(Route, 'useParams').mockReturnValue({ channelId: 'ch-1' })
    vi.spyOn(Route, 'useSearch').mockImplementation(() => currentSearch)
  })

  describe('validateChannelSearch', () => {
    it('validates empty search', () => {
      expect(validateChannelSearch({})).toEqual({})
    })

    it('keeps valid ws and thread params', () => {
      expect(validateChannelSearch({ ws: 'w-1', thread: 'msg-99' })).toEqual({
        ws: 'w-1',
        thread: 'msg-99',
      })
    })

    it('ignores invalid types for thread and ws', () => {
      expect(validateChannelSearch({ ws: '', thread: 123 as unknown as string })).toEqual({})
    })
  })

  describe('ChannelRoute component', () => {
    it('renders SpaceView when no thread param is present', () => {
      currentSearch = { ws: 'w-test' }
      render(<ChannelRoute />)

      expect(screen.getByTestId('space-view')).toBeInTheDocument()
      expect(screen.getByTestId('space-view')).toHaveAttribute('data-channel-id', 'ch-1')
    })

    it('renders SpaceView even when ?thread= is provided (H3: no separate panel)', () => {
      currentSearch = { ws: 'w-test', thread: 'msg-abc' }
      render(<ChannelRoute />)

      const sv = screen.getByTestId('space-view')
      expect(sv).toBeInTheDocument()
      expect(sv).toHaveAttribute('data-channel-id', 'ch-1')
    })

    it('does not render a separate ThreadPanel (H3 fix)', () => {
      currentSearch = { ws: 'w-test', thread: 'msg-abc' }
      render(<ChannelRoute />)

      // There should be no separate thread panel — SpaceView owns thread display.
      expect(screen.queryByTestId('thread-panel')).not.toBeInTheDocument()
    })
  })
})
