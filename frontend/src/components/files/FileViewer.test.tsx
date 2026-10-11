import { useRef } from 'react'
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { driveApi, type FilePreview } from '../../api/drive'
import { ApiError } from '../../api/client'
import { Pressable } from '../primitives'
import { FileViewer, type ViewerFile } from './FileViewer'

const sheetBook = vi.hoisted(() => ({
  names: ['Tháng 10', 'Tháng 11'],
  rows: { 'Tháng 10': [] as unknown[][], 'Tháng 11': [['Mã', 'Ghi chú'], ['Z1', 'sang tháng']] as unknown[][] },
  full: 'A1:B2',
}))
vi.mock('xlsx', () => ({
  read: () => ({
    SheetNames: sheetBook.names,
    Sheets: Object.fromEntries(sheetBook.names.map((n) => [n, { sheet: n, '!ref': 'A1:B2', '!fullref': sheetBook.full }])),
  }),
  utils: {
    decode_range: (ref: string) => ({ e: { r: Number(ref.split(':')[1]!.replace(/\D/g, '')) - 1 } }),
    sheet_to_json: (ws: { sheet: keyof typeof sheetBook.rows }) => sheetBook.rows[ws.sheet],
  },
}))

const file = (over: Partial<ViewerFile> = {}): ViewerFile => ({
  id: 'f-1',
  name: 'Biên bản.pdf',
  mime: 'application/pdf',
  size: 640 * 1024,
  uploaderName: 'Phạm Hải Yến',
  uploaderId: 'u-yen',
  time: '2026-10-10T09:36:00',
  ...over,
})

const native = (mime: string, url = 'https://s.test/x'): FilePreview => ({ kind: 'native', url, mime })

function mockPreview(by: Record<string, FilePreview | Error | Promise<FilePreview>>) {
  return vi.spyOn(driveApi, 'preview').mockImplementation(async (id) => {
    const r = await by[id]
    if (r instanceof Error) throw r
    return r as FilePreview
  })
}

/** The first element matching, waited for (waitFor itself only retries a throw). */
const appears = <T extends Element>(selector: string) =>
  waitFor(() => {
    const el = document.querySelector<T>(selector)
    if (!el) throw new Error(`no ${selector}`)
    return el
  })

const open = (files: ViewerFile[], props: Partial<Parameters<typeof FileViewer>[0]> = {}) =>
  renderWithClient(<FileViewer files={files} index={0} onClose={() => undefined} {...props} />)

beforeEach(() => {
  vi.spyOn(driveApi, 'getDownloadUrl').mockResolvedValue({ download_url: 'https://s.test/dl' })
  sheetBook.rows['Tháng 10'] = [['Ngày', 'Số tiền'], ['01/10', '1.200'], ['02/10', '350']]
  sheetBook.full = 'A1:B3'
})
afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  resetClient()
})

describe('FileViewer header', () => {
  it('draws the header from what the opener knows, before the preview answers', () => {
    mockPreview({ 'f-1': new Promise<FilePreview>(() => undefined) })
    open([file()])
    const dialog = screen.getByRole('dialog', { name: 'Biên bản.pdf' })
    expect(within(dialog).getByText('PDF · 640 KB')).toBeInTheDocument()
    expect(within(dialog).getByText('Phạm Hải Yến')).toBeInTheDocument()
    expect(within(dialog).getByText('10/10/2026 09:36')).toBeInTheDocument()
    expect(dialog.textContent).not.toContain('f-1')
    expect(dialog.textContent).not.toContain('u-yen')
  })

  it('moves focus to Close on open and gives it back to the opener on close', async () => {
    mockPreview({ 'f-1': native('application/pdf') })
    function Host() {
      const ref = useRef<HTMLButtonElement>(null)
      return (
        <>
          <Pressable ref={ref}>opener</Pressable>
          <FileViewer files={[file()]} index={0} onClose={() => undefined} returnFocusRef={ref} />
        </>
      )
    }
    const { unmount } = renderWithClient(<Host />)
    await waitFor(() => expect(screen.getByRole('button', { name: 'Đóng trình xem' })).toHaveFocus())
    unmount()
  })

  it('closes on Escape and on the close button', async () => {
    mockPreview({ 'f-1': native('application/pdf') })
    const onClose = vi.fn()
    open([file()], { onClose })
    await userEvent.keyboard('{Escape}')
    await userEvent.click(screen.getByRole('button', { name: 'Đóng trình xem' }))
    expect(onClose).toHaveBeenCalledTimes(2)
  })

  it('offers "Mở trong Tài liệu" only with a folder, a handler and not from Tài liệu', () => {
    mockPreview({ 'f-1': native('application/pdf') })
    const onOpenInDrive = vi.fn()
    const f = file({ parentFolderId: 'fo-1' })
    const first = open([f], { onOpenInDrive })
    expect(screen.getByRole('button', { name: 'Mở trong Tài liệu' })).toBeInTheDocument()
    first.unmount()

    open([f], { onOpenInDrive, origin: 'drive' })
    expect(screen.queryByRole('button', { name: 'Mở trong Tài liệu' })).not.toBeInTheDocument()
  })

  it('hides it when the file has no folder or no handler', () => {
    mockPreview({ 'f-1': native('application/pdf') })
    open([file()], { onOpenInDrive: vi.fn() })
    expect(screen.queryByRole('button', { name: 'Mở trong Tài liệu' })).not.toBeInTheDocument()
  })

  it('downloads through a fresh URL', async () => {
    mockPreview({ 'f-1': native('application/pdf') })
    open([file()])
    await userEvent.click(screen.getByRole('button', { name: 'Tải xuống' }))
    await waitFor(() => expect(driveApi.getDownloadUrl).toHaveBeenCalledWith('f-1'))
  })

  it('opens the phone sheet with the actions, and Esc closes the sheet before the viewer', async () => {
    mockPreview({ 'f-1': native('application/pdf') })
    const onClose = vi.fn()
    open([file({ parentFolderId: 'fo-1' })], { onClose, onOpenInDrive: vi.fn() })
    await userEvent.click(screen.getByRole('button', { name: 'Thao tác với tệp' }))
    const sheet = screen.getByRole('dialog', { name: 'Thao tác với tệp' })
    expect(within(sheet).getByRole('button', { name: 'Tải xuống' })).toBeInTheDocument()
    expect(within(sheet).getByRole('button', { name: 'Mở trong Tài liệu' })).toBeInTheDocument()
    await userEvent.keyboard('{Escape}')
    expect(screen.queryByRole('dialog', { name: 'Thao tác với tệp' })).not.toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()
    await userEvent.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledTimes(1)
  })
})

describe('FileViewer navigation', () => {
  const list = [file({ id: 'a', name: 'a.png', mime: 'image/png' }), file({ id: 'b', name: 'b.png', mime: 'image/png' }), file({ id: 'c', name: 'c.png', mime: 'image/png' })]
  const answers = { a: native('image/png', 'https://s.test/a'), b: native('image/png', 'https://s.test/b'), c: native('image/png', 'https://s.test/c') }

  it('walks the list with the arrow keys and stops at the ends', async () => {
    mockPreview(answers)
    open(list)
    expect(screen.getByText('1 / 3')).toBeInTheDocument()
    await userEvent.keyboard('{ArrowLeft}')
    expect(screen.getByText('1 / 3')).toBeInTheDocument()
    await userEvent.keyboard('{ArrowRight}{ArrowRight}{ArrowRight}')
    expect(screen.getByText('3 / 3')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Tệp sau' })[0]).toBeDisabled()
  })

  it('walks with the buttons and announces the file', async () => {
    mockPreview(answers)
    open(list)
    await userEvent.click(screen.getAllByRole('button', { name: 'Tệp sau' })[0]!)
    expect(screen.getByRole('dialog', { name: 'b.png' })).toBeInTheDocument()
    expect(screen.getByText('Tệp 2 / 3, b.png')).toBeInTheDocument()
  })

  it('starts at the given index', () => {
    mockPreview(answers)
    open(list, { index: 2 })
    expect(screen.getByRole('dialog', { name: 'c.png' })).toBeInTheDocument()
  })

  it('swipes horizontally at fit to navigate between files', async () => {
    mockPreview(answers)
    open(list)
    const stage = await screen.findByTestId('image-stage')
    // Swipe left to go to next file
    fireEvent.pointerDown(stage, { pointerId: 1, clientX: 200, clientY: 100 })
    fireEvent.pointerMove(stage, { pointerId: 1, clientX: 120, clientY: 100 })
    fireEvent.pointerUp(stage, { pointerId: 1, clientX: 120, clientY: 100 })
    expect(await screen.findByRole('dialog', { name: 'b.png' })).toBeInTheDocument()

    // Swipe right to go back to previous file
    const stage2 = await screen.findByTestId('image-stage')
    fireEvent.pointerDown(stage2, { pointerId: 1, clientX: 100, clientY: 100 })
    fireEvent.pointerMove(stage2, { pointerId: 1, clientX: 180, clientY: 100 })
    fireEvent.pointerUp(stage2, { pointerId: 1, clientX: 180, clientY: 100 })
    expect(await screen.findByRole('dialog', { name: 'a.png' })).toBeInTheDocument()
  })

  it('has no previous/next for a single file', () => {
    mockPreview({ 'f-1': native('application/pdf') })
    open([file()])
    expect(screen.queryByRole('button', { name: 'Tệp sau' })).not.toBeInTheDocument()
    expect(screen.queryByText(/\d \/ \d/)).not.toBeInTheDocument()
  })

  it('leaves the arrows to a video or audio control', async () => {
    mockPreview({ 'f-1': native('video/mp4'), 'f-2': native('video/mp4') })
    open([file({ id: 'f-1', name: 'v.mp4', mime: 'video/mp4' }), file({ id: 'f-2', name: 'w.mp4' })])
    const video = await appears('video')
    fireEvent.keyDown(video, { key: 'ArrowRight' })
    expect(screen.getByText('1 / 2')).toBeInTheDocument()
  })
})

describe('FileViewer bodies', () => {
  it('shows an image fitted, switches to 100% by button and by key, and back with 0', async () => {
    mockPreview({ 'f-1': native('image/png', 'https://s.test/p.png') })
    open([file({ name: 'p.png', mime: 'image/png' })])
    const img = await screen.findByRole('img', { name: 'p.png' })
    expect(img).toHaveAttribute('src', 'https://s.test/p.png')
    const fit = screen.getByRole('button', { name: 'Vừa khung' })
    const one = screen.getByRole('button', { name: '100%' })
    expect(fit).toHaveAttribute('aria-pressed', 'true')
    await userEvent.click(one)
    expect(one).toHaveAttribute('aria-pressed', 'true')
    await userEvent.keyboard('0')
    expect(fit).toHaveAttribute('aria-pressed', 'true')
    await userEvent.keyboard('+')
    expect(one).toHaveAttribute('aria-pressed', 'true')
  })

  it('shows a PDF in the browser viewer', async () => {
    mockPreview({ 'f-1': native('application/pdf', 'https://s.test/x.pdf') })
    open([file()])
    const frame = await screen.findByTitle('Bản xem trước Biên bản.pdf')
    expect(frame).toHaveAttribute('src', 'https://s.test/x.pdf')
  })

  it('shows a converted Word file as a PDF with the "may differ" note', async () => {
    mockPreview({ 'f-1': { kind: 'pdf', url: 'https://s.test/c.pdf', source_mime: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' } })
    open([file({ name: 'Báo cáo.docx', mime: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' })])
    expect(await screen.findByText('Bản xem trước từ tệp Word, có thể khác bản gốc đôi chút.')).toBeInTheDocument()
    expect(screen.getByTitle('Bản xem trước Báo cáo.docx')).toHaveAttribute('src', 'https://s.test/c.pdf')
  })

  it('uses native controls for video and audio', async () => {
    mockPreview({ 'f-1': native('video/mp4'), 'f-2': native('audio/mpeg') })
    const first = open([file({ name: 'v.mp4', mime: 'video/mp4' })])
    expect(await appears('video')).toHaveAttribute('controls')
    first.unmount()
    open([file({ id: 'f-2', name: 'a.mp3', mime: 'audio/mpeg' })])
    const audio = await appears('audio')
    expect(audio).toHaveAttribute('controls')
  })

  it('says a zip cannot be previewed and offers the download', async () => {
    mockPreview({ 'f-1': { kind: 'unavailable', reason: 'unsupported' } })
    open([file({ name: 'bundle.zip', mime: 'application/zip', size: 2 * 1024 * 1024 })])
    expect(await screen.findByText('Không xem trước được loại tệp này')).toBeInTheDocument()
    expect(screen.getByText(/^Tệp nén · 2[.,]?0? MB\. Tải xuống để mở bằng ứng dụng trên máy\.$/)).toBeInTheDocument()
  })

  it.each([
    ['too_large', 'Tệp quá lớn để xem trước'],
    ['conversion_failed', 'Không tạo được bản xem trước'],
    ['conversion_unavailable', 'Không tạo được bản xem trước'],
    ['conversion_busy', 'Không tạo được bản xem trước'],
  ] as const)('explains %s and keeps the download', async (reason, title) => {
    mockPreview({ 'f-1': { kind: 'unavailable', reason } })
    open([file({ name: 'a.docx' })])
    expect(await screen.findByText(title)).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: /Tải xuống/ }).length).toBeGreaterThan(0)
  })

  it('offers a retry for a busy converter but not for a document it cannot read', async () => {
    mockPreview({ 'f-1': { kind: 'unavailable', reason: 'conversion_busy' }, 'f-2': { kind: 'unavailable', reason: 'conversion_failed' } })
    const first = open([file({ name: 'a.docx' })])
    expect(await screen.findByRole('button', { name: 'Thử lại' })).toBeInTheDocument()
    first.unmount()
    open([file({ id: 'f-2', name: 'b.docx' })])
    await screen.findByText('Không tạo được bản xem trước')
    expect(screen.queryByRole('button', { name: 'Thử lại' })).not.toBeInTheDocument()
  })

  it('says "Đang chuẩn bị bản xem trước…" while an Office file converts, then announces it is ready', async () => {
    let finish!: (p: FilePreview) => void
    mockPreview({ 'f-1': new Promise<FilePreview>((r) => (finish = r)) })
    open([file({ name: 'a.docx' })])
    expect(screen.getByText('Đang chuẩn bị bản xem trước…')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).not.toHaveAttribute('aria-valuenow')
    expect(screen.getByRole('button', { name: 'Tải xuống thay vì chờ' })).toBeInTheDocument()
    await act(async () => finish({ kind: 'pdf', url: 'https://s.test/c.pdf', source_mime: 'x' }))
    expect(await screen.findByText('Bản xem trước đã sẵn sàng')).toBeInTheDocument()
    expect(screen.queryByText('Đang chuẩn bị bản xem trước…')).not.toBeInTheDocument()
  })

  it('says it is a long wait after 15 seconds', async () => {
    vi.useFakeTimers()
    try {
      mockPreview({ 'f-1': new Promise<FilePreview>(() => undefined) })
      open([file({ name: 'a.docx' })])
      expect(screen.getByText(/Lần đầu mở tệp Word cần vài giây/)).toBeInTheDocument()
      await act(async () => { await vi.advanceTimersByTimeAsync(15_000) })
      expect(screen.getByText('Tệp lớn cần lâu hơn một chút. Bạn có thể tải xuống thay vì chờ.')).toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })
})

describe('FileViewer refusals', () => {
  it('403: no name, type, size or sender, and no actions', async () => {
    mockPreview({ 'f-1': new ApiError('access denied', 403) })
    open([file({ parentFolderId: 'fo' })], { onOpenInDrive: vi.fn() })
    expect(await screen.findByText('Bạn không có quyền xem tệp này')).toBeInTheDocument()
    const dialog = screen.getByRole('dialog', { name: 'Trình xem tệp' })
    expect(dialog.textContent).not.toContain('Biên bản')
    expect(dialog.textContent).not.toContain('640')
    expect(dialog.textContent).not.toContain('Phạm Hải Yến')
    expect(screen.queryByRole('button', { name: /Tải xuống/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Mở trong Tài liệu' })).not.toBeInTheDocument()
  })

  it('403 still lets the arrows go on to the next file', async () => {
    mockPreview({ 'f-1': new ApiError('access denied', 403), 'f-2': native('image/png') })
    open([file(), file({ id: 'f-2', name: 'ok.png', mime: 'image/png' })])
    await screen.findByText('Bạn không có quyền xem tệp này')
    await userEvent.keyboard('{ArrowRight}')
    expect(await screen.findByRole('dialog', { name: 'ok.png' })).toBeInTheDocument()
  })

  it('404: keeps the name the opener knew, dimmed, and offers nothing', async () => {
    mockPreview({ 'f-1': new ApiError('file not found', 404) })
    open([file({ parentFolderId: 'fo' })], { onOpenInDrive: vi.fn() })
    expect(await screen.findByText('Tệp này đã bị xoá')).toBeInTheDocument()
    expect(screen.getByRole('dialog', { name: 'Biên bản.pdf' })).toBeInTheDocument()
    expect(screen.getByText('Đã xoá')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Tải xuống/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Mở trong Tài liệu' })).not.toBeInTheDocument()
  })

  it('a network failure says so and retries', async () => {
    const spy = vi.spyOn(driveApi, 'preview').mockRejectedValueOnce(new Error('offline')).mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(native('application/pdf'))
    open([file()])
    await userEvent.click(await screen.findByRole('button', { name: 'Thử lại' }, { timeout: 4000 }))
    expect(screen.queryByText('Không tải được tệp')).not.toBeInTheDocument()
    await screen.findByTitle('Bản xem trước Biên bản.pdf')
    expect(spy).toHaveBeenCalledTimes(3)
  })
})

describe('FileViewer text bodies', () => {
  const stubFetch = (body: string) => vi.stubGlobal('fetch', vi.fn(async () => new Response(body)))

  it('shows text as text, never as markup', async () => {
    stubFetch('<script>alert(1)</script><b>x</b>')
    mockPreview({ 'f-1': native('text/plain') })
    open([file({ name: 'page.html', mime: 'text/html' })])
    const region = await screen.findByRole('region', { name: 'Nội dung tệp' })
    expect(region.textContent).toContain('<script>alert(1)</script><b>x</b>')
    expect(region.querySelector('script, b')).toBeNull()
  })

  it('numbers the lines of code and cuts at 500 with the strip', async () => {
    stubFetch(Array.from({ length: 620 }, (_, i) => `line ${i + 1}`).join('\n'))
    mockPreview({ 'f-1': native('text/plain') })
    open([file({ name: 'run.py', mime: 'text/x-python' })])
    expect(await screen.findByText('line 500')).toBeInTheDocument()
    expect(screen.queryByText('line 501')).not.toBeInTheDocument()
    expect(screen.getByText('Đang xem 500 dòng đầu trên 620 dòng.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Tải xuống để xem đủ' })).toBeInTheDocument()
  })

  it('shows no strip when the whole text fits', async () => {
    stubFetch('một\nhai')
    mockPreview({ 'f-1': native('text/plain') })
    open([file({ name: 'ghi-chu.txt', mime: 'text/plain' })])
    await screen.findByRole('region', { name: 'Nội dung tệp' })
    expect(screen.queryByText(/Đang xem/)).not.toBeInTheDocument()
  })

  it('shows CSV as a table with a header row', async () => {
    stubFetch('Ngày,Số tiền\n01/10,"1,200"\n02/10,350')
    mockPreview({ 'f-1': native('text/csv') })
    open([file({ name: 'gd.csv', mime: 'text/csv' })])
    expect(await screen.findByRole('columnheader', { name: 'Số tiền' })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: '1,200' })).toBeInTheDocument()
    expect(screen.getAllByRole('row')).toHaveLength(3)
  })

  it('says so when the text cannot be fetched', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('', { status: 500 })))
    mockPreview({ 'f-1': native('text/plain') })
    open([file({ name: 'a.txt', mime: 'text/plain' })])
    expect(await screen.findByText('Không tải được tệp')).toBeInTheDocument()
  })
})

describe('FileViewer spreadsheets', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(new ArrayBuffer(8))))
    mockPreview({ 'f-1': { kind: 'sheet', url: 'https://s.test/s.xlsx', mime: 'x' } })
  })

  it('shows a tab per sheet and the values of the active one', async () => {
    open([file({ name: 'so-lieu.xlsx', mime: 'application/vnd.ms-excel' })])
    expect(await screen.findByRole('tab', { name: 'Tháng 10' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('cell', { name: '1.200' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('tab', { name: 'Tháng 11' }))
    expect(screen.getByRole('cell', { name: 'sang tháng' })).toBeInTheDocument()
    expect(screen.queryByRole('cell', { name: '1.200' })).not.toBeInTheDocument()
  })

  it('says how many rows are shown of the sheet when it is longer', async () => {
    sheetBook.full = 'A1:B1204'
    sheetBook.rows['Tháng 10'] = [['Ngày'], ...Array.from({ length: 500 }, (_, i) => [String(i)])]
    open([file({ name: 'so-lieu.xlsx' })])
    expect(await screen.findByText('Đang xem 500 dòng đầu trên 1.203 dòng.')).toBeInTheDocument()
  })
})

it('does not crash with no files', () => {
  open([])
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})
