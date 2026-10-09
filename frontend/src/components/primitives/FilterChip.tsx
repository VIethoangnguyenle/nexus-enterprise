import type { ReactNode } from 'react'

interface FilterChipProps {
  pressed: boolean
  onClick: () => void
  children: ReactNode
}

/** Toggle chip for list filters ("Tất cả", "Chưa đọc"…). State is `aria-pressed`. */
export function FilterChip({ pressed, onClick, children }: FilterChipProps) {
  return (
    <button
      type="button"
      aria-pressed={pressed}
      onClick={onClick}
      className={`press h-8 px-3 rounded-md border-none cursor-pointer text-small focus-ring
        ${pressed ? 'bg-accent-wash text-ink font-semibold' : 'bg-sunk text-ink font-medium hover:bg-hover'}`}
    >
      {children}
    </button>
  )
}
