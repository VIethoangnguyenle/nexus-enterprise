import type { ReactNode } from 'react'
import { Heading } from '../primitives'
import { SettingsNav } from './SettingsNav'
import { SettingsLayoutContext } from './settings-layout'
import type { SettingsSection } from './settings-nav'

/**
 * Desktop Cài đặt (≥ 1024; DESIGN.md §6 "Cài đặt trên máy tính", mockup settings-desktop.html): the page title, then
 * the section list (248) beside the open section, under one rule. The section's pieces render in their `wide` form:
 * the same component as on the phone, laid out for the right pane.
 */
export function SettingsShell({ section, children }: { section: SettingsSection; children: ReactNode }) {
  return (
    <section className="flex-1 flex flex-col min-w-0 min-h-0 bg-base" aria-label="Cài đặt">
      <header className="px-5 pt-4 pb-3">
        <Heading as="h1" look="page">Cài đặt</Heading>
      </header>
      <div className="flex-1 min-h-0 grid grid-cols-[248px_minmax(0,1fr)] border-t border-line">
        <SettingsNav current={section} />
        <div className="flex flex-col min-w-0 min-h-0">
          <SettingsLayoutContext value="wide">{children}</SettingsLayoutContext>
        </div>
      </div>
    </section>
  )
}
