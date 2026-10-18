import { useRef, useState, type ReactNode } from 'react'
import { MoreHorizontal, Settings } from 'lucide-react'
import { IconButton, MenuItem, Popover } from '../primitives'
import { useSettingsNav, type SettingsSection } from '../settings/settings-nav'

/**
 * The ⋯ on a module's title row on a phone (DESIGN.md §5, "Hàng tiêu đề của phân hệ"; mockup mobile-shell.html §2):
 * a menu 240 wide with 48-high rows whose first row is always Cài đặt <phân hệ>, opening that module's settings
 * page in the same workspace; the module's own shortcuts follow as `children` (MenuItem rows). Danh bạ has no
 * settings page: with neither a section nor other rows nothing is drawn.
 */
export function ModuleMenu({ section, label, children }: {
  /** The module's settings page; none for Danh bạ. */
  section?: SettingsSection
  /** The module's name: "Thêm tuỳ chọn <label>", "Cài đặt <label>". */
  label: string
  children?: ReactNode
}) {
  const anchor = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  const settings = useSettingsNav()
  if (!section && !children) return null
  return (
    <>
      <IconButton
        ref={anchor}
        size="lg"
        className="size-11 shrink-0"
        aria-label={`Thêm tuỳ chọn ${label}`}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        <MoreHorizontal size={20} strokeWidth={1.75} />
      </IconButton>
      <Popover
        open={open}
        onClose={() => setOpen(false)}
        anchorRef={anchor}
        role="menu"
        label={`Tuỳ chọn ${label}`}
        placement="bottom-end"
        className="w-60"
      >
        {section && (
          <MenuItem
            icon={<Settings size={20} strokeWidth={1.75} aria-hidden="true" />}
            className="h-12 text-[0.9375rem]"
            onClick={() => settings.open(section)}
          >
            Cài đặt {label}
          </MenuItem>
        )}
        {children}
      </Popover>
    </>
  )
}
