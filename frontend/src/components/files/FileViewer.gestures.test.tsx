import { screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { driveApi } from '../../api/drive'
import { touch, touchSwipe } from '../../test/gesture-events'
import { FileViewer, type ViewerFile } from './FileViewer'

const pictures: ViewerFile[] = [
  { id: 'p-1', name: 'anh-hop-doi-soat.jpg', mime: 'image/jpeg', uploaderName: 'Nguyễn Thu Lan' },
  { id: 'p-2', name: 'bien-lai.jpg', mime: 'image/jpeg', uploaderName: 'Nguyễn Thu Lan' },
]

let phone = true
beforeEach(() => {
  phone = true
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: phone && query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
  Object.defineProperty(window, 'innerHeight', { value: 800, configurable: true, writable: true })
  vi.spyOn(driveApi, 'preview').mockResolvedValue({ kind: 'native', url: 'https://s.test/x.jpg', mime: 'image/jpeg' })
  vi.spyOn(driveApi, 'getDownloadUrl').mockResolvedValue({ download_url: 'https://s.test/dl' })
})
afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  resetClient()
})

async function open() {
  const onClose = vi.fn()
  renderWithClient(<FileViewer files={pictures} index={0} onClose={onClose} />)
  const dialog = screen.getByRole('dialog')
  Object.defineProperty(dialog, 'offsetHeight', { value: 800, configurable: true })
  await waitFor(() => expect(screen.getByTestId('image-stage')).toBeInTheDocument())
  return { onClose, dialog, stage: screen.getByTestId('image-stage'), scrim: screen.getByTestId('viewer-scrim') }
}

describe('FileViewer on a phone: drag down to close', () => {
  it('closes when the picture is dragged down past 25% of the screen', async () => {
    const { onClose, stage } = await open()
    touchSwipe(stage, [{ x: 200, y: 200, t: 1000 }, { x: 202, y: 260, t: 1400 }, { x: 204, y: 420, t: 1800 }])
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('settles back after a short, slow drag', async () => {
    const { onClose, stage } = await open()
    touchSwipe(stage, [{ x: 200, y: 200, t: 1000 }, { x: 200, y: 230, t: 1400 }, { x: 200, y: 260, t: 1800 }])
    expect(onClose).not.toHaveBeenCalled()
  })

  it('fades the scrim with the finger', async () => {
    const { stage, scrim } = await open()
    touch(stage, 'touchstart', { x: 200, y: 200, t: 1000 })
    touch(stage, 'touchmove', { x: 200, y: 220, t: 1100 })
    touch(stage, 'touchmove', { x: 200, y: 600, t: 1200 })
    await waitFor(() => expect(Number(scrim.style.opacity)).toBeCloseTo(0.5, 1))
  })

  it('leaves a sideways drag to the picture (it changes picture, it does not close)', async () => {
    const { onClose, stage } = await open()
    touchSwipe(stage, [{ x: 300, y: 200, t: 1000 }, { x: 250, y: 205, t: 1100 }, { x: 100, y: 210, t: 1200 }])
    expect(onClose).not.toHaveBeenCalled()
  })

  it('does not close by dragging on a wide screen', async () => {
    phone = false
    const { onClose, stage } = await open()
    touchSwipe(stage, [{ x: 200, y: 200, t: 1000 }, { x: 202, y: 260, t: 1100 }, { x: 204, y: 600, t: 1200 }])
    expect(onClose).not.toHaveBeenCalled()
  })

  it('still closes from the scrim and the ✕', async () => {
    const { onClose, scrim } = await open()
    scrim.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
    expect(onClose).toHaveBeenCalledTimes(1)
    screen.getByRole('button', { name: 'Đóng trình xem' }).click()
    expect(onClose).toHaveBeenCalledTimes(2)
  })
})
