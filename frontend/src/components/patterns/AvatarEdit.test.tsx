import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AvatarEdit } from './AvatarEdit'

const PICTURE = '/api/auth/avatars/0123456789abcdef0123456789abcdef'

function viewport(phone: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: phone && query.includes('max-width'),
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }))
}

type Props = Partial<React.ComponentProps<typeof AvatarEdit>>

function setup(props: Props = {}) {
  const onUpload = vi.fn()
  const onRemove = vi.fn()
  const view = render(
    <AvatarEdit
      name="Trần Thu Lan"
      hueKey="u1"
      label="Đổi ảnh đại diện"
      canEdit
      onUpload={onUpload}
      onRemove={onRemove}
      {...props}
    />,
  )
  return { onUpload, onRemove, user: userEvent.setup({ applyAccept: false }), ...view }
}

const png = () => new File([new Uint8Array(2048)], 'anh.png', { type: 'image/png' })
const trigger = () => screen.getByRole('button', { name: 'Đổi ảnh đại diện' })

beforeEach(() => viewport(false))
afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe('AvatarEdit', () => {
  it('is only the avatar when the person cannot edit', () => {
    const { container } = setup({ canEdit: false })
    expect(screen.queryByRole('button')).toBeNull()
    expect(container.querySelector('input')).toBeNull()
    expect(container.textContent).toContain('TL')
  })

  it('is one button around the picture that announces a menu', () => {
    setup({ src: PICTURE })
    const b = trigger()
    expect(b).toHaveAttribute('aria-haspopup', 'menu')
    expect(b).toHaveAttribute('aria-expanded', 'false')
    expect(screen.getAllByRole('button')).toHaveLength(1)
  })

  it('opens a menu with Đổi ảnh and Bỏ ảnh when there is a picture', async () => {
    const { user } = setup({ src: PICTURE })
    await user.click(trigger())
    const menu = await screen.findByRole('menu')
    expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual(['Đổi ảnh', 'Bỏ ảnh'])
    expect(trigger()).toHaveAttribute('aria-expanded', 'true')
  })

  it('offers only Tải ảnh lên when there is no picture yet', async () => {
    const { user } = setup()
    await user.click(trigger())
    const menu = await screen.findByRole('menu')
    expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual(['Tải ảnh lên'])
  })

  it.each([['{Enter}'], [' ']])('opens from the keyboard with %j and Esc closes it, returning focus', async (key) => {
    const { user } = setup({ src: PICTURE })
    await user.tab()
    expect(trigger()).toHaveFocus()
    await user.keyboard(key)
    await screen.findByRole('menu')
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('menu')).toBeNull())
    expect(trigger()).toHaveFocus()
  })

  it('moves between the menu items with the arrow keys', async () => {
    const { user } = setup({ src: PICTURE })
    await user.click(trigger())
    const [change, remove] = await screen.findAllByRole('menuitem')
    await waitFor(() => expect(change).toHaveFocus())
    await user.keyboard('{ArrowDown}')
    expect(remove).toHaveFocus()
    await user.keyboard('{ArrowUp}')
    expect(change).toHaveFocus()
  })

  it('uploads the chosen picture', async () => {
    const { user, onUpload } = setup({ src: PICTURE })
    await user.upload(screen.getByLabelText('Tệp ảnh từ máy'), png())
    expect(onUpload).toHaveBeenCalledTimes(1)
    expect(onUpload.mock.calls[0]?.[0]).toBeInstanceOf(File)
  })

  it('Đổi ảnh opens the file picker', async () => {
    const { user } = setup({ src: PICTURE })
    const input = screen.getByLabelText('Tệp ảnh từ máy') as HTMLInputElement
    const click = vi.spyOn(input, 'click')
    await user.click(trigger())
    await user.click(await screen.findByRole('menuitem', { name: 'Đổi ảnh' }))
    expect(click).toHaveBeenCalled()
    await waitFor(() => expect(screen.queryByRole('menu')).toBeNull())
  })

  it('refuses a file that is not a JPG or PNG without calling onUpload', async () => {
    const { user, onUpload } = setup()
    await user.upload(screen.getByLabelText('Tệp ảnh từ máy'), new File(['GIF89a'], 'a.gif', { type: 'image/gif' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(/JPG hoặc PNG/)
    expect(onUpload).not.toHaveBeenCalled()
    expect(trigger()).toHaveAccessibleDescription(/JPG hoặc PNG/)
  })

  it('refuses a file over 5 MB', async () => {
    const { user, onUpload } = setup()
    await user.upload(screen.getByLabelText('Tệp ảnh từ máy'), new File([new Uint8Array(5 * 1024 * 1024 + 1)], 'lon.png', { type: 'image/png' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(/5 MB/)
    expect(onUpload).not.toHaveBeenCalled()
  })

  it('clears its own refusal when a good picture follows', async () => {
    const { user, onUpload } = setup()
    const input = screen.getByLabelText('Tệp ảnh từ máy')
    await user.upload(input, new File(['x'], 'a.gif', { type: 'image/gif' }))
    await screen.findByRole('alert')
    await user.upload(input, png())
    expect(onUpload).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('shows the error the caller passes', () => {
    setup({ error: 'Ảnh quá lớn về kích thước.' })
    expect(screen.getByRole('alert')).toHaveTextContent('Ảnh quá lớn về kích thước.')
  })

  it('Bỏ ảnh calls onRemove', async () => {
    const { user, onRemove } = setup({ src: PICTURE })
    await user.click(trigger())
    await user.click(await screen.findByRole('menuitem', { name: 'Bỏ ảnh' }))
    expect(onRemove).toHaveBeenCalledTimes(1)
  })

  it('says it is busy, with the progress, and does not open', async () => {
    const { user } = setup({ src: PICTURE, busy: true, progress: 0.64 })
    const b = screen.getByRole('button', { name: 'Đang tải ảnh, 64%' })
    expect(b).toHaveAttribute('aria-busy', 'true')
    await user.click(b)
    expect(screen.queryByRole('menu')).toBeNull()
  })

  it('says it is busy without a number when progress is unknown', () => {
    setup({ busy: true })
    expect(screen.getByRole('button', { name: 'Đang cập nhật ảnh' })).toHaveAttribute('aria-busy', 'true')
  })

  it('marks a camera badge that shows on touch screens', () => {
    setup({ src: PICTURE })
    expect(trigger().querySelector('[data-badge]')?.className).toContain('pointer-coarse:grid')
  })

  it('draws a group as a rounded square', () => {
    setup({ shape: 'square', label: 'Đổi ảnh nhóm' })
    expect(screen.getByRole('button', { name: 'Đổi ảnh nhóm' })).toHaveAttribute('data-shape', 'square')
  })

  describe('on a phone', () => {
    beforeEach(() => viewport(true))

    it('opens a sheet with Chụp ảnh, Chọn ảnh, Bỏ ảnh and Huỷ', async () => {
      const { user } = setup({ src: PICTURE })
      await user.click(trigger())
      const sheet = await screen.findByRole('dialog', { name: 'Đổi ảnh đại diện' })
      expect(screen.queryByRole('menu')).toBeNull()
      expect(within(sheet).getAllByRole('button').map((b) => b.textContent)).toEqual(['Chụp ảnh', 'Chọn ảnh', 'Bỏ ảnh', 'Huỷ'])
    })

    it('leaves out Bỏ ảnh when there is no picture', async () => {
      const { user } = setup()
      await user.click(trigger())
      const sheet = await screen.findByRole('dialog')
      expect(within(sheet).queryByRole('button', { name: 'Bỏ ảnh' })).toBeNull()
    })

    it('opens the camera input, which asks for the front camera', async () => {
      const { user, onUpload } = setup({ src: PICTURE })
      const camera = screen.getByLabelText('Ảnh chụp từ máy ảnh') as HTMLInputElement
      expect(camera).toHaveAttribute('capture', 'user')
      const click = vi.spyOn(camera, 'click')
      await user.click(trigger())
      await user.click(await screen.findByRole('button', { name: 'Chụp ảnh' }))
      expect(click).toHaveBeenCalled()
      await user.upload(camera, png())
      expect(onUpload).toHaveBeenCalledTimes(1)
    })

    it('Huỷ closes the sheet and changes nothing', async () => {
      const { user, onUpload, onRemove } = setup({ src: PICTURE })
      await user.click(trigger())
      await user.click(await screen.findByRole('button', { name: 'Huỷ' }))
      await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
      expect(onUpload).not.toHaveBeenCalled()
      expect(onRemove).not.toHaveBeenCalled()
    })
  })
})
