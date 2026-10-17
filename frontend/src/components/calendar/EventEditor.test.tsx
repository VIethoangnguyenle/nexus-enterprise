import { cleanup, configure, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter, useParams, useSearch,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { E, NOW, PEOPLE, UUID_RE, calendarFixtureApi, calls, mode, resetFixtures } from '../../test/calendar-fixtures'
import { validateEditEventSearch, validateNewEventSearch, type EditEventSearch, type NewEventSearch } from '../../lib/calendar-search'
import { apiFetch } from '../../api/client'
import { useToastStore } from '../primitives'
import { EditEventEditor, NewEventEditor } from './EventEditorRoutes'

vi.mock('../../api/client', async (orig) => {
  const { calendarFixtureApi } = await import('../../test/calendar-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(calendarFixtureApi) }
})

const api = vi.mocked(apiFetch)
configure({ asyncUtilTimeout: 4000 })
vi.setConfig({ testTimeout: 15_000 })

function NewPage() {
  return <NewEventEditor search={useSearch({ strict: false }) as NewEventSearch} />
}
function EditPage() {
  const { eventId } = useParams({ strict: false }) as { eventId: string }
  return <EditEventEditor eventId={eventId} search={useSearch({ strict: false }) as EditEventSearch} />
}

async function renderEditor(url: string) {
  const root = createRootRoute()
  const grid = createRoute({ getParentRoute: () => root, path: '/calendar', component: () => <p>Lưới lịch</p> })
  const added = createRoute({ getParentRoute: () => root, path: '/calendar/new', validateSearch: validateNewEventSearch, component: NewPage })
  const edit = createRoute({ getParentRoute: () => root, path: '/calendar/event/$eventId', validateSearch: validateEditEventSearch, component: EditPage })
  const router = createRouter({ routeTree: root.addChildren([grid, added, edit]), history: createMemoryHistory({ initialEntries: [url] }) })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

const NEW = '/calendar/new?date=2026-10-14&start=09:00&end=10:00'
const writes = (method: string) => calls.filter((c) => c.method === method)
const save = (user: ReturnType<typeof userEvent.setup>) => user.click(screen.getByRole('button', { name: 'Lưu' }))
const setup = () => userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
const ready = () => screen.findByRole('textbox', { name: 'Tiêu đề' })

beforeEach(() => {
  localStorage.clear()
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(NOW)
  resetFixtures()
  api.mockImplementation(calendarFixtureApi)
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.restoreAllMocks()
  resetClient()
  useToastStore.getState().clear()
})

describe('the times', () => {
  it('starts on the span it was sent, with the length beside the end', async () => {
    await renderEditor(NEW)
    await ready()
    expect(screen.getByLabelText('Ngày bắt đầu')).toHaveValue('2026-10-14')
    expect(screen.getByLabelText('Giờ bắt đầu')).toHaveValue('09:00')
    expect(screen.getByLabelText('Giờ kết thúc')).toHaveValue('10:00')
    expect(screen.getByText('1 giờ')).toBeInTheDocument()
  })

  it('keeps the length when the start moves', async () => {
    await renderEditor(NEW)
    await ready()
    fireEvent.change(screen.getByLabelText('Giờ bắt đầu'), { target: { value: '13:30' } })
    expect(screen.getByLabelText('Giờ kết thúc')).toHaveValue('14:30')
  })

  it('says the end must come after the start once the field is left, and does not save', async () => {
    const user = setup()
    await renderEditor(NEW)
    await ready()
    const end = screen.getByLabelText('Giờ kết thúc')
    fireEvent.change(end, { target: { value: '08:00' } })
    expect(screen.queryByRole('alert')).toBeNull()
    fireEvent.blur(end)
    expect(await screen.findByRole('alert')).toHaveTextContent('Giờ kết thúc phải sau giờ bắt đầu')
    await save(user)
    expect(writes('POST')).toHaveLength(0)
  })

  it('names the error on Lưu even when no field was left', async () => {
    const user = setup()
    await renderEditor(NEW)
    await ready()
    fireEvent.change(screen.getByLabelText('Giờ kết thúc'), { target: { value: '09:00' } })
    await save(user)
    expect(await screen.findByRole('alert')).toHaveTextContent('Giờ kết thúc phải sau giờ bắt đầu')
    expect(writes('POST')).toHaveLength(0)
  })
})

describe('the repeat', () => {
  it('lists the choices worked out from the day', async () => {
    await renderEditor(NEW)
    await ready()
    const select = screen.getByLabelText('Lặp lại')
    expect(within(select).getAllByRole('option').map((o) => o.textContent)).toEqual([
      'Không lặp lại', 'Hằng ngày', 'Hằng tuần vào thứ Tư', 'Hằng tháng vào thứ Tư thứ hai', 'Hằng năm vào ngày 14 tháng 10',
      'Mọi ngày trong tuần (thứ Hai đến thứ Sáu)', 'Tùy chọn…',
    ])
  })

  it.each([
    ['daily', 'FREQ=DAILY'],
    ['weekly', 'FREQ=WEEKLY;BYDAY=WE'],
    ['monthly', 'FREQ=MONTHLY;BYDAY=2WE'],
    ['yearly', 'FREQ=YEARLY'],
    ['weekdays', 'FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR'],
  ])('sends the rule of %s', async (id, rrule) => {
    const user = setup()
    await renderEditor(NEW)
    await ready()
    await user.selectOptions(screen.getByLabelText('Lặp lại'), id)
    await save(user)
    await waitFor(() => expect(writes('POST')).toHaveLength(1))
    expect(writes('POST')[0]!.body).toMatchObject({ rrule })
  })

  it('sends no rule for Không lặp lại', async () => {
    const user = setup()
    await renderEditor(NEW)
    await ready()
    await save(user)
    await waitFor(() => expect(writes('POST')).toHaveLength(1))
    expect(writes('POST')[0]!.body).not.toHaveProperty('rrule')
  })

  it('makes a custom rule in a dialog and says it in words, not as a rule', async () => {
    const user = setup()
    await renderEditor(NEW)
    await ready()
    await user.selectOptions(screen.getByLabelText('Lặp lại'), 'custom')
    const dialog = await screen.findByRole('dialog', { name: 'Lặp lại tùy chỉnh' })
    fireEvent.change(within(dialog).getByLabelText('Số lần'), { target: { value: '2' } })
    await user.click(within(dialog).getByRole('button', { name: 'T6' }))
    await user.click(within(dialog).getByRole('radio', { name: 'Sau' }))
    fireEvent.change(within(dialog).getByLabelText('Số lần lặp'), { target: { value: '5' } })
    expect(within(dialog).getByText('Mỗi 2 tuần vào thứ Tư và thứ Sáu, 5 lần')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Xong' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Lặp lại tùy chỉnh' })).toBeNull())
    expect(screen.queryByText(/FREQ/)).toBeNull()
    await save(user)
    await waitFor(() => expect(writes('POST')).toHaveLength(1))
    expect(writes('POST')[0]!.body).toMatchObject({ rrule: 'FREQ=WEEKLY;INTERVAL=2;BYDAY=WE,FR;COUNT=5' })
  })

  it('ends a custom rule on a day', async () => {
    const user = setup()
    await renderEditor(NEW)
    await ready()
    await user.selectOptions(screen.getByLabelText('Lặp lại'), 'custom')
    const dialog = await screen.findByRole('dialog', { name: 'Lặp lại tùy chỉnh' })
    await user.click(within(dialog).getByRole('radio', { name: 'Vào ngày' }))
    fireEvent.change(within(dialog).getByLabelText('Ngày kết thúc'), { target: { value: '2026-12-31' } })
    expect(within(dialog).getByText('Hằng tuần vào thứ Tư, đến hết 31/12/2026')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Xong' }))
    await save(user)
    await waitFor(() => expect(writes('POST')).toHaveLength(1))
    expect(String(writes('POST')[0]!.body!.rrule)).toMatch(/^FREQ=WEEKLY;BYDAY=WE;UNTIL=\d{8}T\d{6}Z$/)
  })
})

describe('saving', () => {
  it('saves a new event with Ctrl+S and goes back to the grid on that day', async () => {
    const user = setup()
    const router = await renderEditor(NEW)
    const title = await ready()
    await user.type(title, 'Duyệt ngân sách')
    await user.keyboard('{Control>}s{/Control}')
    await waitFor(() => expect(writes('POST')).toHaveLength(1))
    expect(writes('POST')[0]!.body).toMatchObject({ title: 'Duyệt ngân sách', allDay: false, hasCall: false })
    await waitFor(() => expect(router.state.location.pathname).toBe('/calendar'))
    expect(router.state.location.search).toMatchObject({ date: '2026-10-14' })
  })

  it('uses the default reminder until the person chooses their own', async () => {
    const user = setup()
    await renderEditor(NEW)
    await ready()
    await waitFor(() => expect(screen.getAllByLabelText('Số')).toHaveLength(1))
    expect(screen.getByLabelText('Số')).toHaveValue(10)
    await user.click(screen.getByRole('button', { name: 'Bỏ lời nhắc' }))
    await save(user)
    await waitFor(() => expect(writes('POST')).toHaveLength(1))
    expect(writes('POST')[0]!.body).toMatchObject({ reminders: [] })
  })

  it('asks "Bỏ thay đổi?" when closed with changes, and goes on editing when told to', async () => {
    const user = setup()
    const router = await renderEditor(NEW)
    await user.type(await ready(), 'x')
    await user.click(screen.getByRole('button', { name: 'Đóng trình soạn' }))
    const ask = await screen.findByRole('dialog', { name: 'Bỏ thay đổi?' })
    await user.click(within(ask).getByRole('button', { name: 'Tiếp tục sửa' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Bỏ thay đổi?' })).toBeNull())
    expect(router.state.location.pathname).toBe('/calendar/new')
    await user.click(screen.getByRole('button', { name: 'Đóng trình soạn' }))
    await user.click(within(await screen.findByRole('dialog', { name: 'Bỏ thay đổi?' })).getByRole('button', { name: 'Bỏ' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/calendar'))
  })

  it('closes at once when nothing changed', async () => {
    const user = setup()
    const router = await renderEditor(NEW)
    await ready()
    await user.click(screen.getByRole('button', { name: 'Đóng trình soạn' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/calendar'))
  })
})

describe('guests', () => {
  it('adds a person from the picker, shows when they are busy, and sends their node', async () => {
    const user = setup()
    await renderEditor(NEW)
    await ready()
    await user.type(screen.getByRole('combobox', { name: 'Thêm khách' }), 'Yến')
    await user.click(await screen.findByRole('option', { name: /Phạm Hải Yến/ }))
    const list = screen.getByRole('region', { name: 'Khách' })
    expect(await within(list).findByText('Bận lúc này · Gặp ngân hàng')).toBeInTheDocument()
    await save(user)
    await waitFor(() => expect(writes('POST')).toHaveLength(1))
    expect(writes('POST')[0]!.body).toMatchObject({ attendees: [PEOPLE.yen.nodeId] })
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('shows a guest as free when nothing is in the way, and lets them be taken off', async () => {
    const user = setup()
    await renderEditor('/calendar/new?date=2026-10-14&start=13:00&end=14:00')
    await ready()
    await user.type(screen.getByRole('combobox', { name: 'Thêm khách' }), 'Yến')
    await user.click(await screen.findByRole('option', { name: /Phạm Hải Yến/ }))
    expect(await screen.findByText('Rảnh')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Bỏ Phạm Hải Yến' }))
    expect(screen.queryByText('Rảnh')).toBeNull()
  })

  it('invites a whole group chat by name', async () => {
    const user = setup()
    await renderEditor(NEW)
    await ready()
    await user.type(screen.getByRole('combobox', { name: 'Mời cả nhóm' }), 'Vận hành')
    await user.click(await screen.findByRole('option', { name: /Vận hành/ }))
    await save(user)
    await waitFor(() => expect(writes('POST')).toHaveLength(1))
    expect(writes('POST')[0]!.body).toHaveProperty('inviteChannelId')
  })

  it('has no field that takes an email or an id', async () => {
    await renderEditor(NEW)
    await ready()
    expect(screen.queryByPlaceholderText(/email|id/i)).toBeNull()
  })
})

describe('Tìm giờ', () => {
  async function openFind(user: ReturnType<typeof userEvent.setup>, url = NEW) {
    await renderEditor(url)
    await ready()
    await user.type(screen.getByRole('combobox', { name: 'Thêm khách' }), 'Yến')
    await user.click(await screen.findByRole('option', { name: /Phạm Hải Yến/ }))
    await user.click(screen.getByRole('tab', { name: 'Tìm giờ' }))
    return screen.findByTestId('find-time')
  }

  it('draws a column for each person with the clash flagged, and the chosen time as a band', async () => {
    const user = setup()
    const find = await openFind(user)
    await within(find).findByText('Trùng giờ')
    expect(within(find).getByText('Phạm Hải Yến')).toBeInTheDocument()
    expect(within(find).getByText('Gặp ngân hàng')).toBeInTheDocument()
    expect(within(find).getByRole('slider', { name: 'Khoảng giờ đã chọn, 09:00 đến 10:00' })).toBeInTheDocument()
  })

  it('moves the band a quarter of an hour with the arrow keys, and the form follows', async () => {
    const user = setup()
    const find = await openFind(user)
    const band = within(find).getByRole('slider')
    band.focus()
    await user.keyboard('{ArrowDown}')
    expect(within(find).getByRole('slider', { name: 'Khoảng giờ đã chọn, 09:15 đến 10:15' })).toBeInTheDocument()
    await user.click(screen.getByRole('tab', { name: 'Chi tiết sự kiện' }))
    expect(screen.getByLabelText('Giờ bắt đầu')).toHaveValue('09:15')
  })

  it('jumps the band to a suggested time', async () => {
    const user = setup()
    const find = await openFind(user)
    await user.click(await within(find).findByRole('button', { name: '11:30 – 12:30' }))
    expect(within(find).getByRole('slider', { name: 'Khoảng giờ đã chọn, 11:30 đến 12:30' })).toBeInTheDocument()
  })

  it('says so when nobody shares a free hour, and offers tomorrow', async () => {
    mode.noSlots = true
    const user = setup()
    const find = await openFind(user)
    expect(await within(find).findByText(/Không có giờ nào mọi người cùng rảnh hôm nay/)).toBeInTheDocument()
    await user.click(within(find).getByRole('button', { name: 'Xem ngày mai' }))
    expect(within(find).getByText('Thứ Năm, 15 tháng 10')).toBeInTheDocument()
  })

  it('takes a guest off the grid without taking them off the event', async () => {
    const user = setup()
    const find = await openFind(user)
    await user.click(within(find).getByRole('button', { name: 'Ẩn Phạm Hải Yến khỏi tìm giờ' }))
    expect(within(find).queryByText('Phạm Hải Yến')).toBeNull()
    await user.click(screen.getByRole('tab', { name: 'Chi tiết sự kiện' }))
    expect(screen.getByRole('button', { name: 'Bỏ Phạm Hải Yến' })).toBeInTheDocument()
  })
})

describe('editing an event', () => {
  const standup = `/calendar/event/${E.standup}?occurrence=2026-10-16T09:00:00%2B07:00&start=2026-10-16T09:00:00%2B07:00&end=2026-10-16T09:15:00%2B07:00`

  it('opens on the instance that was clicked and sends only what changed', async () => {
    const user = setup()
    await renderEditor(`/calendar/event/${E.report}`)
    const title = await ready()
    expect(title).toHaveValue('Báo cáo tuần')
    await user.clear(title)
    await user.type(title, 'Báo cáo tháng')
    await save(user)
    await waitFor(() => expect(writes('PATCH')).toHaveLength(1))
    expect(writes('PATCH')[0]!.body).toEqual({ version: 3, title: 'Báo cáo tháng' })
    expect(writes('PATCH')[0]!.query.get('scope')).toBeNull()
  })

  it('asks which occurrences a recurring change covers, then sends the scope and the instance', async () => {
    const user = setup()
    await renderEditor(standup)
    const title = await ready()
    expect(screen.getByLabelText('Ngày bắt đầu')).toHaveValue('2026-10-16')
    await user.clear(title)
    await user.type(title, 'Standup mới')
    await save(user)
    const ask = await screen.findByRole('dialog', { name: 'Sửa sự kiện lặp lại' })
    expect(writes('PATCH')).toHaveLength(0)
    await user.click(within(ask).getByRole('radio', { name: 'Tất cả sự kiện' }))
    await user.click(within(ask).getByRole('button', { name: 'OK' }))
    await waitFor(() => expect(writes('PATCH')).toHaveLength(1))
    const call = writes('PATCH')[0]!
    expect(call.query.get('scope')).toBe('all')
    expect(call.query.get('occurrence')).toBe('2026-10-16T09:00:00+07:00')
    expect(call.body).toMatchObject({ title: 'Standup mới' })
    expect(call.body).not.toHaveProperty('start')
  })

  it('does not ask when nothing changed', async () => {
    const user = setup()
    const router = await renderEditor(standup)
    await ready()
    await save(user)
    await waitFor(() => expect(router.state.location.pathname).toBe('/calendar'))
    expect(writes('PATCH')).toHaveLength(0)
  })

  it('says in words that an event is gone', async () => {
    await renderEditor('/calendar/event/e0000000-aaaa-4bbb-8ccc-0000000000ee')
    expect(await screen.findByText('Sự kiện này không còn nữa.')).toBeInTheDocument()
  })

  it('does not offer the editor on an event that cannot be changed', async () => {
    await renderEditor(`/calendar/event/${E.interview}`)
    expect(await screen.findByText('Bạn không có quyền sửa sự kiện này.')).toBeInTheDocument()
  })

  it('copies an event into a new one without its guests', async () => {
    await renderEditor(`/calendar/new?copy=${E.giaoBan}&date=2026-10-21&start=09:00&end=10:00`)
    expect(await ready()).toHaveValue('Họp giao ban')
    expect(screen.getByLabelText('Ngày bắt đầu')).toHaveValue('2026-10-21')
    expect(screen.queryByRole('button', { name: /^Bỏ Phạm/ })).toBeNull()
  })
})
