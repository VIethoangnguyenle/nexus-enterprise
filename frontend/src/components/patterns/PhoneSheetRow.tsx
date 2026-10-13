import type { ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { Pressable } from '../primitives'

const ROW = `flex items-center gap-3.5 w-full min-h-14 px-3.5 rounded-lg text-left text-base font-medium no-underline
  hover:bg-hover active:bg-hover transition-colors duration-quick focus-ring`

interface PhoneSheetRowProps {
  /** A 20px icon; it sits in a 40px tile. */
  icon: ReactNode
  /** One muted line under the label. */
  hint?: string
  tone?: 'default' | 'danger'
  children: ReactNode
}

/**
 * A 56px row of a phone sheet (design/mockups/chat-mobile-home.html `.sheet .item`):
 * icon tile, label and an optional muted line. A button, or a link with `to`.
 */
export function PhoneSheetRow(
  props: PhoneSheetRowProps & ({ onClick: () => void; to?: undefined } | { to: string; search?: Record<string, string>; onClick?: () => void }),
) {
  const { icon, hint, tone = 'default', children } = props
  const inner = (
    <>
      <span
        aria-hidden="true"
        className={`grid place-items-center w-10 h-10 rounded-xl bg-sunk shrink-0 ${tone === 'danger' ? 'text-danger' : 'text-ink-muted'}`}
      >
        {icon}
      </span>
      <span className="grid min-w-0">
        <span className="truncate">{children}</span>
        {hint && <span className="text-[0.8125rem] font-normal text-ink-muted truncate">{hint}</span>}
      </span>
    </>
  )
  const className = `${ROW} ${tone === 'danger' ? 'text-danger' : 'text-ink'}`
  if (props.to !== undefined) {
    return (
      <Link to={props.to} search={props.search} onClick={props.onClick} className={className}>
        {inner}
      </Link>
    )
  }
  return <Pressable onClick={props.onClick} className={className}>{inner}</Pressable>
}
