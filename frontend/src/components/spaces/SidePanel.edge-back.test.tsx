import { screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient } from '../../test/render'
import { touchSwipe } from '../../test/gesture-events'
import { SidePanel } from './SidePanel'

let standalone = true
let sheet = true
vi.mock('../../lib/push', () => ({ isStandalone: () => standalone }))

beforeEach(() => {
  standalone = true
  sheet = true
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: sheet && query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
  Object.defineProperty(window, 'innerWidth', { value: 390, configurable: true, writable: true })
})
afterEach(() => vi.unstubAllGlobals())

function thread(props: { onBack?: () => void; edgeBack?: boolean } = {}) {
  const onClose = vi.fn()
  renderWithClient(
    <SidePanel label="Thread" title="Thread" closeLabel="thread" sheetBelow="lg" backOnSheet edgeBack onClose={onClose} {...props}>
      <p>Các câu trả lời</p>
    </SidePanel>,
  )
  return { onClose, panel: screen.getByRole('dialog', { name: 'Thread' }) }
}

const fromEdge = (x0: number) => [{ x: x0, t: 1000 }, { x: x0 + 15, t: 1016 }, { x: x0 + 260, t: 1800 }]

describe('SidePanel as a sheet in the installed app: edge back', () => {
  it('leaves the thread on a swipe from the left edge, as ← does', async () => {
    const { onClose, panel } = thread()
    touchSwipe(panel, fromEdge(5))
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1))
  })

  it('goes back to the thread list when it was opened from there', async () => {
    const onBack = vi.fn()
    const { onClose, panel } = thread({ onBack })
    touchSwipe(panel, fromEdge(5))
    await waitFor(() => expect(onBack).toHaveBeenCalledTimes(1))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('does nothing for the same swipe starting 40px in (that is a message swipe)', async () => {
    const { onClose, panel } = thread()
    touchSwipe(panel, fromEdge(40))
    await new Promise((r) => setTimeout(r, 320))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('stays when released short of 35% of the width, slowly', async () => {
    const { onClose, panel } = thread()
    touchSwipe(panel, [{ x: 5, t: 1000 }, { x: 20, t: 1100 }, { x: 80, t: 1900 }])
    await new Promise((r) => setTimeout(r, 320))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('leaves the edge to Safari in a browser tab, where ← is the way back', async () => {
    standalone = false
    const { onClose, panel } = thread()
    expect(screen.queryByTestId('panel-edge-back')).toBeNull()
    touchSwipe(panel, fromEdge(5))
    await new Promise((r) => setTimeout(r, 320))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('has no edge swipe as a side panel on a wide screen', () => {
    sheet = false
    renderWithClient(
      <SidePanel label="Thread" title="Thread" closeLabel="thread" sheetBelow="lg" edgeBack onClose={vi.fn()}>
        <p>Các câu trả lời</p>
      </SidePanel>,
    )
    expect(screen.queryByTestId('panel-edge-back')).toBeNull()
  })
})
