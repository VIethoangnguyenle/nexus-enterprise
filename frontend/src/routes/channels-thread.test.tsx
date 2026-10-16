import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { validateChannelSearch, Route } from './_workspace/channels.$channelId'
import { ChannelRoute } from './_workspace/-channel-route'

let currentSearch: { ws?: string; thread?: string; message?: string } = {}

vi.mock('@tanstack/react-router', async () => {
  const actual = await vi.importActual('@tanstack/react-router')
  return {
    ...actual,
    useNavigate: () => vi.fn(),
  }
})

vi.mock('../components/spaces/SpaceView', () => ({
  SpaceView: ({ channelId, initialThreadId, focusMessageId }: { channelId: string; initialThreadId?: string; focusMessageId?: string }) => (
    <div data-testid="space-view" data-channel-id={channelId} data-thread={initialThreadId} data-message={focusMessageId}>
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

    it('keeps a message id to focus, with or without a thread', () => {
      expect(validateChannelSearch({ message: 'msg-1' })).toEqual({ message: 'msg-1' })
      expect(validateChannelSearch({ thread: 't-1', message: 'msg-1' })).toEqual({ thread: 't-1', message: 'msg-1' })
    })

    it('drops a message that is empty, not a string, or not shaped like an id', () => {
      expect(validateChannelSearch({ message: '' })).toEqual({})
      expect(validateChannelSearch({ message: 5 as unknown as string })).toEqual({})
      expect(validateChannelSearch({ message: '"] x[y="' })).toEqual({})
      expect(validateChannelSearch({ thread: 'a b' })).toEqual({})
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

    it('hands the thread and the message to SpaceView as its focus target', () => {
      currentSearch = { thread: 'root-1', message: 'reply-9' }
      render(<ChannelRoute />)
      const sv = screen.getByTestId('space-view')
      expect(sv).toHaveAttribute('data-thread', 'root-1')
      expect(sv).toHaveAttribute('data-message', 'reply-9')
    })

    it('has no focus target when the link names none', () => {
      currentSearch = {}
      render(<ChannelRoute />)
      expect(screen.getByTestId('space-view')).not.toHaveAttribute('data-thread')
      expect(screen.getByTestId('space-view')).not.toHaveAttribute('data-message')
    })

    it('does not render a separate ThreadPanel (H3 fix)', () => {
      currentSearch = { ws: 'w-test', thread: 'msg-abc' }
      render(<ChannelRoute />)

      // There should be no separate thread panel — SpaceView owns thread display.
      expect(screen.queryByTestId('thread-panel')).not.toBeInTheDocument()
    })
  })
})
