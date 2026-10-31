import { createContext, useContext } from 'react'

/**
 * How the settings pieces lay out. `phone` is a full-screen page with a back bar and grouped cards (DESIGN.md §6
 * "Cài đặt"); `wide` is the right pane of the desktop shell, flat rows under a title (§6 "Cài đặt trên máy tính").
 * Only the desktop shell provides `wide`, so anything rendered outside it — every phone page — keeps the phone look.
 */
export type SettingsLayout = 'phone' | 'wide'

export const SettingsLayoutContext = createContext<SettingsLayout>('phone')

/** True inside the desktop settings shell. */
export function useSettingsWide(): boolean {
  return useContext(SettingsLayoutContext) === 'wide'
}
