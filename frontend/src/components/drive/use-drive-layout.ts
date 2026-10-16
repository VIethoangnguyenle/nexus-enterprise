import { useBottomBarLayout, usePhone } from '../../hooks/usePhone'

/** The three sizes Tài liệu is laid out for (DESIGN.md §5): 1024 and up, 768–1023, and under 768. */
export type DriveLayout = 'desktop' | 'tablet' | 'phone'

export function useDriveLayout(): DriveLayout {
  const phone = usePhone()
  const belowDesktop = useBottomBarLayout()
  return phone ? 'phone' : belowDesktop ? 'tablet' : 'desktop'
}
