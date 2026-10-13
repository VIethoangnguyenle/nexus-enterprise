import '@testing-library/jest-dom/vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { resetClient } from '../../test/render'
import { apiFetch } from '../../api/client'
import { answer, dialogOf, renderShell, resetServer, viewport } from '../patterns/phone-shell-harness'
import { NewChatButton } from './NewChatButton'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

beforeEach(() => {
  viewport(true)
  resetServer()
  vi.mocked(apiFetch).mockReset()
  vi.mocked(apiFetch).mockImplementation(answer as typeof apiFetch)
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
})

async function openSheet() {
  await renderShell(<NewChatButton />)
  const user = userEvent.setup()
  const plus = await screen.findByRole('button', { name: 'Trò chuyện mới' })
  await user.click(plus)
  return { user, plus, sheet: await dialogOf('Trò chuyện mới') }
}

describe('NewChatButton', () => {
  it('is a 64px button that opens a sheet', async () => {
    await renderShell(<NewChatButton />)
    const plus = await screen.findByRole('button', { name: 'Trò chuyện mới' })
    expect(plus).toHaveAttribute('aria-haspopup', 'dialog')
    expect(plus).toHaveClass('w-16', 'h-16')
  })

  it('offers Nhắn tin trực tiếp, Tạo nhóm and Tạo mục, and no Duyệt nhóm while there is nothing to browse', async () => {
    const { sheet } = await openSheet()
    expect(sheet).toHaveAttribute('aria-modal', 'true')
    expect(within(sheet).getAllByRole('button').map((b) => b.textContent)).toEqual([
      'Nhắn tin trực tiếpChọn một người',
      'Tạo nhómNơi cả nhóm trò chuyện theo chủ đề',
      'Tạo mụcGom các cuộc trò chuyện của bạn',
    ])
    expect(within(sheet).queryByText('Duyệt nhóm')).toBeNull()
  })

  it('opens the direct message dialog and closes the sheet', async () => {
    const { user, sheet } = await openSheet()
    await user.click(within(sheet).getByRole('button', { name: /Nhắn tin trực tiếp/ }))
    expect(await dialogOf('Nhắn tin trực tiếp')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Trò chuyện mới' })).toBeNull())
  })

  it('opens the group dialog', async () => {
    const { user, sheet } = await openSheet()
    await user.click(within(sheet).getByRole('button', { name: /Tạo nhóm/ }))
    expect(await dialogOf('Tạo nhóm')).toBeInTheDocument()
  })

  it('opens the name sheet for a new section', async () => {
    const { user, sheet } = await openSheet()
    await user.click(within(sheet).getByRole('button', { name: /Tạo mục/ }))
    const naming = await dialogOf('Tên mục mới')
    expect(within(naming).getByLabelText('Tên mục mới')).toBeInTheDocument()
  })

  it('gives focus back to the button when the sheet or a dialog closes', async () => {
    const { user, plus } = await openSheet()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(plus).toHaveFocus()
    await user.click(plus)
    await user.click(within(await dialogOf('Trò chuyện mới')).getByRole('button', { name: /Tạo nhóm/ }))
    await dialogOf('Tạo nhóm')
    await user.click(screen.getByRole('button', { name: 'Huỷ' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(plus).toHaveFocus()
  })
})
