import type { CSSProperties, ReactNode } from 'react'
import { Avatar, Heading, Text } from '../primitives'
import { useMotionPresets } from '../../lib/motion'
import { personStyle } from '../../lib/person-hue'
import { NexusLogo } from '../brand/NexusLogo'
import { AuthGraph } from './AuthGraph'
import { useLogoAssembly } from './auth-motion'
import './auth-motion.css'

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

function BrandPane({ assemble }: { assemble: boolean }) {
  return (
    <aside aria-hidden="true" className="relative overflow-hidden hidden lg:grid grid-rows-[auto_1fr_auto] gap-6 p-10 bg-sunk">
      <AuthGraph />
      <NexusLogo size={44} assemble={assemble} subtitle="Không gian làm việc chung" className="relative" />

      <div className="relative self-center grid gap-4 max-w-sm">
        <Heading as="h2" className="font-display text-xl font-semibold text-ink leading-snug">
          Mỗi thay đổi đều mang tên và màu của người làm.
        </Heading>
        <ol className="auth-acts grid gap-3 m-0 p-0 list-none">
          {/* Oldest first on arrival: the newest change lands last and carries the wash. */}
          {SAMPLE_ACTIVITY.map((a, i) => (
            <li
              key={a.name}
              style={{ ...(a.fresh ? personStyle(a.name) : {}), '--n': SAMPLE_ACTIVITY.length - 1 - i } as CSSProperties}
              className={`grid grid-cols-[1.5rem_minmax(0,1fr)] gap-2.5 items-start rounded-surface p-2 -m-2
                ${a.fresh ? 'auth-fresh' : ''}`}
            >
              <Avatar name={a.name} size={24} />
              <span className="text-sm leading-snug text-ink">
                <b className="font-semibold">{a.name}</b> {a.did}
                <time className="block text-xs text-ink-muted">
                  {a.when}
                  {a.fresh && <span className="auth-tag">vừa tải lên</span>}
                </time>
              </span>
            </li>
          ))}
        </ol>
      </div>

      <Text variant="caption" muted className="relative">Chat, tài liệu, phê duyệt và tài sản trong một chỗ.</Text>
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
  const { reduced } = useMotionPresets()
  const assemble = useLogoAssembly()
  return (
    <div className="auth-shell min-h-dvh grid lg:grid-cols-2 bg-base text-ink" data-reduced={reduced ? 'true' : undefined}>
      <BrandPane assemble={assemble} />
      <main className="grid place-items-center px-4 py-8 sm:px-6 lg:p-10">
        <div className={`w-full ${wide ? 'max-w-105' : 'max-w-95'} grid gap-5`}>
          <NexusLogo size={36} assemble={assemble} className="lg:hidden" />
          {children}
        </div>
      </main>
    </div>
  )
}
