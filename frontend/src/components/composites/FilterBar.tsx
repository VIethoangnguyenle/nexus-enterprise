import type { ReactNode } from 'react'

interface FilterBarProps {
  children: ReactNode
  className?: string
}

/** Horizontal filter bar container. Wrap filter Select/Input elements inside. */
export function FilterBar({ children, className = '' }: FilterBarProps) {
  return (
    <div className={`flex gap-2 items-center flex-wrap mb-4 ${className}`}>
      {children}
    </div>
  )
}
