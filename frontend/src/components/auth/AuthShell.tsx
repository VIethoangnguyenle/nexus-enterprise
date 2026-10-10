import type { ReactNode } from 'react'
import { Avatar, Heading, Text } from '../primitives'
import { personStyle } from '../../lib/person-hue'

/** The mark: the product's initial on the accent. */
function Mark({ className = '' }: { className?: string }) {
  return (
    <span
      aria-hidden="true"
      className={`inline-grid place-items-center shrink-0 w-8 h-8 rounded-surface bg-accent text-on-accent
        font-display font-bold text-base leading-none ${className}`}
    >
      N
    </span>
  )
}

/**
 * Three recent changes, each with its author's name and colour: the product's
 * one idea (DESIGN.md §1) shown on the brand pane. Decoration only: the people
 * are not real and the pane is hidden from assistive technology.
 */
const SAMPLE_ACTIVITY = [
  { name: 'Nguyễn Thu Lan', did: 'đã tải lên doi-soat-09-10.xlsx', when: 'Vừa xong', fresh: true },
  { name: 'Đỗ Văn Khải', did: 'đã duyệt Tạm ứng công tác phí', when: '5 phút trước', fresh: false },
  { name: 'Lê Quang Vinh', did: 'đã nhắn trong Đối soát giao dịch', when: '12 phút trước', fresh: false },
]

function BrandPane() {
  return (
    <aside aria-hidden="true" className="hidden lg:grid grid-rows-[auto_1fr_auto] gap-6 p-10 bg-sunk">
      <div className="flex items-center gap-2.5">
        <Mark />
        <div>
          <div className="font-display font-bold text-base leading-tight text-ink">Nexus Hub</div>
          <div className="text-xs text-ink-muted">Không gian làm việc chung</div>
        </div>
      </div>

      <div className="self-center grid gap-4 max-w-sm">
        <Heading as="h2" className="font-display text-xl font-semibold text-ink leading-snug">
          Mỗi thay đổi đều mang tên và màu của người làm.
        </Heading>
        <ol className="grid gap-3 m-0 p-0 list-none">
          {SAMPLE_ACTIVITY.map((a) => (
            <li
              key={a.name}
              style={a.fresh ? personStyle(a.name) : undefined}
              className={`grid grid-cols-[1.5rem_minmax(0,1fr)] gap-2.5 items-start
                ${a.fresh ? 'rounded-surface p-2 -m-2 bg-(--pw)' : ''}`}
            >
              <Avatar name={a.name} size={24} />
              <span className="text-sm leading-snug text-ink">
                <b className="font-semibold">{a.name}</b> {a.did}
                <time className="block text-xs text-ink-muted">{a.when}</time>
              </span>
            </li>
          ))}
        </ol>
      </div>

      <Text variant="caption" muted>Chat, tài liệu, phê duyệt và tài sản trong một chỗ.</Text>
    </aside>
  )
}

/**
 * Frame of every sign-in screen (design/mockups/auth.html): a brand pane on
 * the sunk tone beside the form on the base tone from 1024px up, and only the
 * form, topped by the mark, below it. `wide` (420px instead of 380px) is for
 * the workspace list, whose rows carry more text than a form field.
 */
export function AuthShell({ children, wide = false }: { children: ReactNode; wide?: boolean }) {
  return (
    <div className="min-h-dvh grid lg:grid-cols-2 bg-base text-ink">
      <BrandPane />
      <main className="grid place-items-center px-4 py-8 sm:px-6 lg:p-10">
        <div className={`w-full ${wide ? 'max-w-105' : 'max-w-95'} grid gap-5`}>
          <div className="flex items-center gap-2.5 lg:hidden">
            <Mark />
            <span className="font-display font-bold text-base text-ink">Nexus Hub</span>
          </div>
          {children}
        </div>
      </main>
    </div>
  )
}
