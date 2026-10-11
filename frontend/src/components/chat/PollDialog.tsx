import { useState } from 'react'
import { Plus, X } from 'lucide-react'
import { useCreatePoll } from '../../hooks/usePolls'
import { explain } from '../../lib/errors'
import { Button, Checkbox, Pressable, Select, TextField } from '../primitives'
import { AlertBanner } from '../composites/AlertBanner'
import { Dialog } from '../composites/Dialog'

const MIN_OPTIONS = 2
const MAX_OPTIONS = 10
const QUESTION_MAX = 200
const HOUR = 3_600_000

export interface PollDraft {
  question: string
  options: string[]
  endsAt: Date | undefined
}

export interface PollErrors {
  question?: string
  options?: string
  /** Rows (0-based) that are empty while the poll asks for filled choices. */
  blank?: number[]
  endsAt?: string
}

/** What is wrong with a poll draft; an empty object means it can be sent. Nothing leaves the client until it is. */
export function validatePoll(d: PollDraft, now: Date = new Date()): PollErrors {
  const errors: PollErrors = {}
  if (!d.question.trim()) errors.question = 'Nhập câu hỏi.'
  const filled = d.options.filter((o) => o.trim()).length
  if (d.options.length > MAX_OPTIONS) errors.options = `Tối đa ${MAX_OPTIONS} lựa chọn.`
  else if (filled < MIN_OPTIONS) errors.options = `Cần ít nhất ${MIN_OPTIONS} lựa chọn.`
  const blank = d.options.flatMap((o, i) => (o.trim() ? [] : [i]))
  if (blank.length > 0 && filled >= MIN_OPTIONS) errors.blank = blank
  if (d.endsAt && !(d.endsAt.getTime() > now.getTime())) errors.endsAt = 'Hạn đóng phải ở tương lai.'
  return errors
}

type EndMode = 'none' | 'hour' | 'day' | 'custom'

function endOfToday(now: Date) {
  const d = new Date(now)
  d.setHours(23, 59, 0, 0)
  return d
}

interface PollDialogProps {
  open: boolean
  onClose: () => void
  channelId: string
  /** Made inside a thread: the poll is posted as a reply to this root. */
  parentMessageId?: string
}

/**
 * "Tạo bình chọn" (DESIGN.md "Bình chọn"): question, 2-10 choices, optional
 * multiple answers, anonymity and a closing time. The poll reaches the
 * conversation as a message once the server accepts it.
 */
export function PollDialog({ open, onClose, channelId, parentMessageId }: PollDialogProps) {
  const create = useCreatePoll(channelId, parentMessageId)
  return (
    <Dialog open={open} onClose={create.isPending ? () => {} : onClose} title="Tạo bình chọn">
      {/* Mounted only while open, so every opening starts blank. */}
      <PollForm onClose={onClose} create={create} />
    </Dialog>
  )
}

function PollForm({ onClose, create }: { onClose: () => void; create: ReturnType<typeof useCreatePoll> }) {
  const [question, setQuestion] = useState('')
  const [options, setOptions] = useState(['', ''])
  const [multi, setMulti] = useState(false)
  const [anonymous, setAnonymous] = useState(false)
  const [endMode, setEndMode] = useState<EndMode>('none')
  const [custom, setCustom] = useState('')
  const [errors, setErrors] = useState<PollErrors>({})
  const [failure, setFailure] = useState<string>()

  const endsAtFor = (now: Date): Date | undefined => {
    if (endMode === 'hour') return new Date(now.getTime() + HOUR)
    if (endMode === 'day') return endOfToday(now)
    if (endMode === 'custom') return custom ? new Date(custom) : new Date(NaN)
    return undefined
  }

  const submit = async () => {
    const now = new Date()
    const endsAt = endsAtFor(now)
    const found = validatePoll({ question, options, endsAt }, now)
    if (endMode === 'custom' && !custom) found.endsAt = 'Chọn ngày giờ đóng.'
    setErrors(found)
    if (Object.keys(found).length > 0) return
    setFailure(undefined)
    try {
      await create.mutateAsync({
        question: question.trim(),
        options: options.map((o) => o.trim()),
        is_multi: multi,
        is_anonymous: anonymous,
        ...(endsAt ? { ends_at: endsAt.toISOString() } : {}),
      })
    } catch (err) {
      setFailure(explain(err, 'tạo bình chọn'))
      return
    }
    onClose()
  }

  const setOption = (i: number, v: string) => setOptions((prev) => prev.map((o, j) => (j === i ? v : o)))

  return (
    <form
      className="grid gap-3.5"
      onSubmit={(e) => {
        e.preventDefault()
        void submit()
      }}
    >
      <TextField
        label="Câu hỏi"
        value={question}
        maxLength={QUESTION_MAX}
        counter
        autoFocus
        error={errors.question}
        onChange={(e) => setQuestion(e.target.value)}
      />

      <fieldset className="grid gap-1.5 m-0 p-0 border-none">
        <legend className="mb-1.5 p-0 text-sm font-semibold text-ink">Lựa chọn</legend>
        {options.map((o, i) => {
          const bad = errors.blank?.includes(i)
          return (
            <div key={i} className="grid gap-1">
              <div className="flex items-center gap-1.5">
                {/* eslint-disable-next-line no-restricted-syntax -- Hàng lựa chọn không nhãn hiện (mockup #cards); tên truy cập là aria-label */}
                <input
                  aria-label={`Lựa chọn ${i + 1}`}
                  aria-invalid={bad || undefined}
                  value={o}
                  maxLength={QUESTION_MAX}
                  placeholder="Thêm lựa chọn"
                  onChange={(e) => setOption(i, e.target.value)}
                  className={`flex-1 min-w-0 h-11 px-3 rounded-md bg-raised text-base text-ink outline-none
                    placeholder:text-ink-muted ${bad ? 'field-danger' : 'field-line'} focus:field-focus`}
                />
                <Pressable
                  aria-label={`Bỏ lựa chọn ${i + 1}`}
                  disabled={options.length <= MIN_OPTIONS}
                  onClick={() => setOptions((prev) => prev.filter((_, j) => j !== i))}
                  className="grid place-items-center w-11 h-11 shrink-0 rounded-md text-ink-muted
                    hover:bg-hover hover:text-ink disabled:opacity-40 disabled:cursor-default"
                >
                  <X size={16} strokeWidth={1.75} aria-hidden="true" />
                </Pressable>
              </div>
              {bad && <span className="text-xs font-medium text-danger">Lựa chọn không được để trống.</span>}
            </div>
          )
        })}
        {errors.options && <span className="text-xs font-medium text-danger">{errors.options}</span>}
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="justify-self-start"
          disabled={options.length >= MAX_OPTIONS}
          onClick={() => setOptions((prev) => [...prev, ''])}
        >
          <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
          Thêm lựa chọn
        </Button>
      </fieldset>

      <label className="flex items-center gap-2.5 min-h-11 text-sm text-ink cursor-pointer">
        <Checkbox label="Cho chọn nhiều đáp án" checked={multi} onChange={(e) => setMulti(e.target.checked)} />
        <span>Cho chọn nhiều đáp án</span>
      </label>
      <label className="flex items-start gap-2.5 min-h-11 text-sm text-ink cursor-pointer">
        <Checkbox label="Ẩn danh" checked={anonymous} onChange={(e) => setAnonymous(e.target.checked)} />
        <span className="grid">
          Ẩn danh
          <small className="text-xs text-ink-muted">Không ai thấy ai đã chọn gì, chỉ thấy số phiếu.</small>
        </span>
      </label>

      <div className="grid gap-1.5">
        <label htmlFor="poll-end" className="text-sm font-semibold text-ink">Đóng bình chọn</label>
        <Select id="poll-end" value={endMode} onChange={(e) => setEndMode(e.target.value as EndMode)}>
          <option value="none">Không tự đóng</option>
          <option value="hour">Sau 1 giờ</option>
          <option value="day">Cuối ngày hôm nay (23:59)</option>
          <option value="custom">Chọn ngày giờ…</option>
        </Select>
        {endMode === 'custom' && (
          <TextField
            label="Ngày giờ đóng"
            type="datetime-local"
            value={custom}
            error={errors.endsAt}
            onChange={(e) => setCustom(e.target.value)}
          />
        )}
        {endMode !== 'custom' && errors.endsAt && <span className="text-xs font-medium text-danger">{errors.endsAt}</span>}
      </div>

      {failure && <AlertBanner variant="error">{failure}</AlertBanner>}

      <div className="flex justify-end gap-2">
        <Button type="button" variant="soft" onClick={onClose} disabled={create.isPending}>Huỷ</Button>
        <Button type="submit" loading={create.isPending}>Gửi bình chọn</Button>
      </div>
    </form>
  )
}
