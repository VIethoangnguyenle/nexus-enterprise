import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from '../../api/client'
import { keys } from '../../hooks/keys'
import { queryClient } from '../../lib/query-client'
import { resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { useToastStore } from '../primitives'
import { SettingsChoice } from './SettingsChoice'
import { SettingsLayoutContext } from './settings-layout'
import { CAL_TEAM, WS, renderSettings, resetSettingsServer, server, settingsAnswer, viewport } from './settings-test-server'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  logoutSession: vi.fn(),
}))
const api = vi.mocked(apiFetch)

beforeEach(() => {
  viewport(false)
  resetSettingsServer()
  api.mockReset()
  api.mockImplementation(settingsAnswer as typeof apiFetch)
  useToastStore.getState().clear()
})
afterEach(() => {
  cleanup()
  resetClient()
  vi.unstubAllGlobals()
})

const page = (name: string) => screen.findByRole('region', { name })
const putsTo = (path: string) => server.writes.filter((w) => w.method === 'PUT' && w.path.split('?')[0] === path)

describe('Chat on a wide screen', () => {
  it('the four kind switches, the default level as a drop-down; no row that only shapes the phone', async () => {
    await renderSettings(`/settings/chat?ws=${WS}`)
    const chat = await page('Chat')
    expect(within(chat).getByRole('heading', { level: 1, name: 'Chat' })).toBeInTheDocument()
    expect(within(chat).getByText('Trong NovaPay')).toBeInTheDocument()
    for (const label of ['Tin nhắn trực tiếp', 'Được nhắc tên', 'Thread đang theo dõi', 'Cuộc gọi đến']) {
      expect(within(chat).getByRole('switch', { name: label }), label).toBeInTheDocument()
    }
    expect(within(chat).getByRole('combobox', { name: 'Mức mặc định cho nhóm mới' })).toHaveValue('main')
    for (const label of ['Enter để gửi', 'Mở Chat ở', 'Sắp xếp', 'Cảm xúc khi chạm đúp']) {
      expect(within(chat).queryByText(label), label).toBeNull()
    }
    expect(screen.queryByRole('button', { name: 'Quay lại' })).toBeNull()
    expectNoIds()
  })

  it('Báo cho tôi khi: a switch saves the account\'s push preferences at once, the rest as they were', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/chat?ws=${WS}`)
    const before = structuredClone(server.push)
    const call = await screen.findByRole('switch', { name: 'Cuộc gọi đến' })
    await waitFor(() => expect(call).toBeEnabled())
    expect(call).toHaveAttribute('aria-checked', 'true')
    await user.click(call)
    await waitFor(() => expect(putsTo('/me/push-preferences')).toHaveLength(1))
    expect(putsTo('/me/push-preferences')[0]!.body).toEqual({ ...before, kinds: { ...before.kinds, incoming_call: false } })
    await waitFor(() => expect(call).toHaveAttribute('aria-checked', 'false'))
  })

  it('Báo cho tôi khi: a refused save puts the switch back and says so', async () => {
    const user = userEvent.setup()
    server.settingsDown = true
    await renderSettings(`/settings/chat?ws=${WS}`)
    const mention = await screen.findByRole('switch', { name: 'Được nhắc tên' })
    await waitFor(() => expect(mention).toBeEnabled())
    await user.click(mention)
    await waitFor(() => expect(useToastStore.getState().toasts.map((t) => t.message)).toContain('Không lưu được cài đặt thông báo'))
    await waitFor(() => expect(mention).toHaveAttribute('aria-checked', 'true'))
  })

  it('picking a level saves only that key at once', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/chat?ws=${WS}`)
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Mức mặc định cho nhóm mới' }), 'all')
    await waitFor(() => expect(putsTo(`/workspaces/${WS}/chat-settings`)).toEqual([
      { method: 'PUT', path: `/workspaces/${WS}/chat-settings`, body: { default_level: 'all' } },
    ]))
    expect(screen.getByRole('combobox', { name: 'Mức mặc định cho nhóm mới' })).toHaveValue('all')
  })

  it('Quản lý mục opens the sections of Chat', async () => {
    const user = userEvent.setup()
    const router = await renderSettings(`/settings/chat?ws=${WS}`)
    await user.click(await screen.findByRole('button', { name: /^Quản lý mục/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/channels'))
    expect(router.state.location.search).toEqual({ ws: WS, tab: 'sections' })
  })

  it('who creates groups is a locked row when an administrator set it', async () => {
    server.groupCreation = 'admins'
    await renderSettings(`/settings/chat?ws=${WS}`)
    await screen.findByText('Chỉ quản trị viên')
    const rule = screen.getByText('Ai được tạo nhóm').closest('[aria-disabled]') as HTMLElement
    expect(rule).toHaveAttribute('aria-disabled', 'true')
    expect(within(rule).getByText('Quản trị viên NovaPay đã đặt')).toBeInTheDocument()
  })

  it('a personal workspace has its own lead line and no group rule', async () => {
    server.personal = true
    await renderSettings(`/settings/chat?ws=${WS}`)
    const chat = await page('Chat')
    expect(await within(chat).findByText('Trong workspace này')).toBeInTheDocument()
    expect(within(chat).queryByText('Ai được tạo nhóm')).toBeNull()
  })
})

describe('Tài liệu on a wide screen', () => {
  it('Xem dạng and Sắp xếp inline, Thư mục trước tệp; no Tải ảnh xem trước', async () => {
    await renderSettings(`/settings/drive?ws=${WS}`)
    const drive = await page('Tài liệu')
    expect(within(drive).getByText('Trong NovaPay')).toBeInTheDocument()
    const view = within(drive).getByRole('radiogroup', { name: 'Xem dạng' })
    expect(within(view).getAllByRole('radio').map((r) => r.textContent)).toEqual(['Tự động', 'Danh sách', 'Lưới'])
    expect(within(view).getByRole('radio', { name: 'Tự động' })).toHaveAttribute('aria-checked', 'true')
    const sort = within(drive).getByRole('radiogroup', { name: 'Sắp xếp' })
    expect(within(sort).getByRole('radio', { name: 'Sửa gần nhất' })).toHaveAttribute('aria-checked', 'true')
    expect(within(drive).getByRole('switch', { name: 'Thư mục trước tệp' })).toBeInTheDocument()
    expect(within(drive).queryByText('Tải ảnh xem trước')).toBeNull()
    expect(within(drive).queryByText('Dữ liệu di động')).toBeNull()
  })

  it('picking a sort saves only that key at once', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/drive?ws=${WS}`)
    const sort = await screen.findByRole('radiogroup', { name: 'Sắp xếp' })
    await user.click(within(sort).getByRole('radio', { name: 'Tên' }))
    await waitFor(() => expect(putsTo('/drive/settings').map((w) => w.body)).toEqual([{ sort: 'name' }]))
    expect(within(sort).getByRole('radio', { name: 'Tên' })).toHaveAttribute('aria-checked', 'true')
  })

  it('arrow keys move the choice and save it', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/drive?ws=${WS}`)
    const view = await screen.findByRole('radiogroup', { name: 'Xem dạng' })
    within(view).getByRole('radio', { name: 'Tự động' }).focus()
    await user.keyboard('{ArrowRight}')
    await waitFor(() => expect(putsTo('/drive/settings').map((w) => w.body)).toEqual([{ view: 'list' }]))
    expect(within(view).getByRole('radio', { name: 'Danh sách' })).toHaveFocus()
  })

  it('a refused save puts the old choice back and says so', async () => {
    const user = userEvent.setup()
    server.settingsDown = true
    await renderSettings(`/settings/drive?ws=${WS}`)
    const sort = await screen.findByRole('radiogroup', { name: 'Sắp xếp' })
    await user.click(within(sort).getByRole('radio', { name: 'Dung lượng' }))
    await waitFor(() => expect(putsTo('/drive/settings')).toHaveLength(1))
    await waitFor(() => expect(within(sort).getByRole('radio', { name: 'Sửa gần nhất' })).toHaveAttribute('aria-checked', 'true'))
    expect(within(sort).getByRole('radio', { name: 'Dung lượng' })).toHaveAttribute('aria-checked', 'false')
    await waitFor(() => expect(useToastStore.getState().toasts.map((t) => t.message).join(' ')).toMatch(/chưa lưu cài đặt được/i))
  })
})

describe('Lịch on a wide screen', () => {
  const putsOfSettings = () => putsTo(`/workspaces/${WS}/calendar/settings`).map((w) => w.body)

  it('every option of the phone inline, the working hours on one row; no default view or duration', async () => {
    await renderSettings(`/settings/calendar?ws=${WS}`)
    const cal = await page('Lịch')
    expect(within(cal).getByRole('heading', { level: 1, name: 'Lịch' })).toBeInTheDocument()
    expect(within(cal).getByText('Trong NovaPay')).toBeInTheDocument()
    const week = within(cal).getByRole('radiogroup', { name: 'Tuần bắt đầu vào' })
    expect(within(week).getByRole('radio', { name: 'Thứ Hai' })).toHaveAttribute('aria-checked', 'true')
    await waitFor(() => expect(within(cal).getByRole('combobox', { name: 'Múi giờ' })).toHaveValue('Asia/Ho_Chi_Minh'))
    // The zone reads as an offset and a place, never as its IANA id.
    expect(within(cal).getByRole('option', { name: '(GMT+7) Hà Nội' })).toHaveProperty('selected', true)
    expect(within(cal).queryByRole('option', { name: /Asia\/|Ho Chi Minh/ })).toBeNull()
    expect(within(cal).getByRole('combobox', { name: 'Bắt đầu làm việc' })).toHaveValue('08:00')
    expect(within(cal).getByRole('combobox', { name: 'Kết thúc làm việc' })).toHaveValue('17:30')
    expect(await within(cal).findByRole('checkbox', { name: 'Lê Thị Hoa' })).toBeChecked()
    expect(within(cal).getByRole('checkbox', { name: 'NovaPay' })).toBeChecked()
    expect(within(cal).getByRole('combobox', { name: 'Nhắc trước' })).toHaveValue('10')
    for (const label of ['Lời mời mới', 'Thay đổi sự kiện', 'Nhắc sự kiện']) {
      expect(within(cal).getByRole('switch', { name: label }), label).toBeInTheDocument()
    }
    for (const label of ['Chế độ xem mặc định', 'Thời lượng mặc định']) expect(within(cal).queryByText(label), label).toBeNull()
    expect(screen.queryByRole('dialog')).toBeNull()
    expectNoIds()
  })

  it('a later end of the working day saves only the end', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/calendar?ws=${WS}`)
    const end = await screen.findByRole('combobox', { name: 'Kết thúc làm việc' })
    await waitFor(() => expect(end).toHaveValue('17:30'))
    await user.selectOptions(end, '18:00')
    await waitFor(() => expect(putsOfSettings()).toEqual([{ workEnd: '18:00' }]))
    expect(end).toHaveValue('18:00')
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('an end not after the start is refused here and nothing is sent', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/calendar?ws=${WS}`)
    const end = await screen.findByRole('combobox', { name: 'Kết thúc làm việc' })
    await waitFor(() => expect(end).toHaveValue('17:30'))
    await user.selectOptions(end, '07:30')
    expect(await screen.findByRole('alert')).toHaveTextContent('Giờ kết thúc phải sau giờ bắt đầu.')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Bắt đầu làm việc' }), '07:30')
    expect(screen.getByRole('alert')).toHaveTextContent('Giờ kết thúc phải sau giờ bắt đầu.')
    expect(putsOfSettings()).toEqual([])

    // Moving the start back makes the pair good: both changed fields go in one save.
    await user.selectOptions(screen.getByRole('combobox', { name: 'Bắt đầu làm việc' }), '07:00')
    await waitFor(() => expect(putsOfSettings()).toEqual([{ workStart: '07:00', workEnd: '07:30' }]))
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('the reminder and the zone each save only their key', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/calendar?ws=${WS}`)
    const reminder = await screen.findByRole('combobox', { name: 'Nhắc trước' })
    await waitFor(() => expect(reminder).toHaveValue('10'))
    await user.selectOptions(reminder, '30')
    await waitFor(() => expect(putsOfSettings()).toEqual([{ defaultReminder: 30 }]))
    await user.selectOptions(screen.getByRole('combobox', { name: 'Múi giờ' }), 'Asia/Tokyo')
    await waitFor(() => expect(putsOfSettings()).toEqual([{ defaultReminder: 30 }, { tzid: 'Asia/Tokyo' }]))
  })

  it('a new zone refreshes Tìm giờ, as a new pair of working hours does', async () => {
    const user = userEvent.setup()
    const busy = [...keys.calendar.freeBusyAll(WS), 'u-hoa']
    queryClient.setQueryData(busy, { people: [] })
    await renderSettings(`/settings/calendar?ws=${WS}`)
    const zone = await screen.findByRole('combobox', { name: 'Múi giờ' })
    await waitFor(() => expect(zone).toHaveValue('Asia/Ho_Chi_Minh'))
    expect(queryClient.getQueryState(busy)?.isInvalidated).toBe(false)
    await user.selectOptions(zone, 'Asia/Tokyo')
    await waitFor(() => expect(putsOfSettings()).toEqual([{ tzid: 'Asia/Tokyo' }]))
    await waitFor(() => expect(queryClient.getQueryState(busy)?.isInvalidated).toBe(true))
  })

  it('Lịch đang hiện unticks the same checkbox as the calendar column', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/calendar?ws=${WS}`)
    const team = await screen.findByRole('checkbox', { name: 'NovaPay' })
    await user.click(team)
    await waitFor(() => expect(putsTo(`/workspaces/${WS}/calendar/calendars/${CAL_TEAM}/subscription`).map((w) => w.body))
      .toEqual([{ visible: false }]))
    await waitFor(() => expect(team).not.toBeChecked())
    expect(server.calendars.find((c) => c.id === CAL_TEAM)!.visible).toBe(false)
    expect(screen.getByRole('checkbox', { name: 'Lê Thị Hoa' })).toBeChecked()
    expect(putsOfSettings()).toEqual([])
  })
})

describe('Tài sản on a wide screen', () => {
  it('Tab mở đầu inline, saved at once', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/assets?ws=${WS}`)
    const assets = await page('Tài sản')
    expect(within(assets).getByText('Trong NovaPay')).toBeInTheDocument()
    const tab = within(assets).getByRole('radiogroup', { name: 'Tab mở đầu' })
    await waitFor(() => expect(within(tab).getByRole('radio', { name: 'Tổng quan' })).toHaveAttribute('aria-checked', 'true'))
    await user.click(within(tab).getByRole('radio', { name: 'Yêu cầu' }))
    await waitFor(() => expect(putsTo(`/workspaces/${WS}/asset-settings`).map((w) => w.body)).toEqual([{ default_tab: 'requests' }]))
    expect(within(tab).getByRole('radio', { name: 'Yêu cầu' })).toHaveAttribute('aria-checked', 'true')
  })
})

describe('Phê duyệt on a wide screen', () => {
  it('the two switches and no Sắp xếp', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/approval?ws=${WS}`)
    const approval = await page('Phê duyệt')
    expect(within(approval).getByText('Trong NovaPay')).toBeInTheDocument()
    const pending = within(approval).getByRole('switch', { name: 'Đề nghị chờ bạn duyệt' })
    expect(within(approval).getByRole('switch', { name: 'Kết quả đề nghị của bạn' })).toBeInTheDocument()
    expect(within(approval).queryByText('Sắp xếp')).toBeNull()
    await waitFor(() => expect(pending).toHaveAttribute('aria-checked', 'true'))
    await user.click(pending)
    await waitFor(() => expect(putsTo('/approval/settings').map((w) => w.body)).toEqual([{ notify_pending: false }]))
  })
})

describe('SettingsChoice', () => {
  const FOUR = [
    { value: 'a', label: 'Một' }, { value: 'b', label: 'Hai' }, { value: 'c', label: 'Ba' }, { value: 'd', label: 'Bốn' },
  ]

  it('wide: four or more choices are a drop-down named by the row', async () => {
    const onPick = vi.fn()
    render(
      <SettingsLayoutContext value="wide">
        <SettingsChoice label="Thứ tự" value="b" choices={FOUR} onPick={onPick} onOpen={() => {}} />
      </SettingsLayoutContext>,
    )
    const box = screen.getByRole('combobox', { name: 'Thứ tự' })
    expect(box).toHaveValue('b')
    await userEvent.setup().selectOptions(box, 'd')
    expect(onPick).toHaveBeenCalledWith('d')
  })

  it('wide: numbers keep their type through the drop-down', async () => {
    const onPick = vi.fn()
    const nums = [5, 10, 15, 30].map((n) => ({ value: n, label: `${n} phút` }))
    render(
      <SettingsLayoutContext value="wide">
        <SettingsChoice label="Nhắc trước" value={10} choices={nums} onPick={onPick} onOpen={() => {}} />
      </SettingsLayoutContext>,
    )
    await userEvent.setup().selectOptions(screen.getByRole('combobox', { name: 'Nhắc trước' }), '30')
    expect(onPick).toHaveBeenCalledWith(30)
  })

  it('wide: Home and End jump to the first and the last choice', async () => {
    const onPick = vi.fn()
    const three = FOUR.slice(0, 3)
    render(
      <SettingsLayoutContext value="wide">
        <SettingsChoice label="Thứ tự" value="b" choices={three} onPick={onPick} onOpen={() => {}} />
      </SettingsLayoutContext>,
    )
    const user = userEvent.setup()
    screen.getByRole('radio', { name: 'Hai' }).focus()
    await user.keyboard('{End}')
    expect(onPick).toHaveBeenLastCalledWith('c')
    expect(screen.getByRole('radio', { name: 'Ba' })).toHaveFocus()
    await user.keyboard('{Home}')
    expect(onPick).toHaveBeenLastCalledWith('a')
    expect(screen.getByRole('radio', { name: 'Một' })).toHaveFocus()
  })

  it('wide and disabled: neither control can pick', async () => {
    const onPick = vi.fn()
    const { rerender } = render(
      <SettingsLayoutContext value="wide">
        <SettingsChoice label="Thứ tự" value="b" choices={FOUR} onPick={onPick} onOpen={() => {}} disabled />
      </SettingsLayoutContext>,
    )
    expect(screen.getByRole('combobox', { name: 'Thứ tự' })).toBeDisabled()
    rerender(
      <SettingsLayoutContext value="wide">
        <SettingsChoice label="Thứ tự" value="b" choices={FOUR.slice(0, 3)} onPick={onPick} onOpen={() => {}} disabled />
      </SettingsLayoutContext>,
    )
    for (const radio of screen.getAllByRole('radio')) expect(radio).toBeDisabled()
    await userEvent.setup().click(screen.getByRole('radio', { name: 'Ba' }))
    expect(onPick).not.toHaveBeenCalled()
  })

  it('wide and locked: the value and who set it, no control', () => {
    render(
      <SettingsLayoutContext value="wide">
        <SettingsChoice label="Thứ tự" value="b" choices={FOUR} onPick={() => {}} onOpen={() => {}} locked="Quản trị viên NovaPay đã đặt" />
      </SettingsLayoutContext>,
    )
    expect(screen.queryByRole('combobox')).toBeNull()
    expect(screen.getByText('Hai')).toBeInTheDocument()
    expect(screen.getByText('Quản trị viên NovaPay đã đặt')).toBeInTheDocument()
  })

  it('phone: the row with the current value opens the choice page', async () => {
    const onOpen = vi.fn()
    const onPick = vi.fn()
    render(<SettingsChoice label="Thứ tự" value="b" choices={FOUR} onPick={onPick} onOpen={onOpen} />)
    expect(screen.queryByRole('combobox')).toBeNull()
    expect(screen.queryByRole('radiogroup')).toBeNull()
    await userEvent.setup().click(screen.getByRole('button', { name: /^Thứ tự\s*Hai/ }))
    expect(onOpen).toHaveBeenCalledOnce()
    expect(onPick).not.toHaveBeenCalled()
  })
})
