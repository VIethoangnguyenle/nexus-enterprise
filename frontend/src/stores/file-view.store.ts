import { create } from 'zustand'
import { createJSONStorage, persist, type StateStorage } from 'zustand/middleware'

/** The two screens that list files, each remembering its own layout. */
export type FileViewScreen = 'drive' | 'space-files'
export type FileViewMode = 'grid' | 'list'

const SCREENS: FileViewScreen[] = ['drive', 'space-files']
const DEFAULT_MODES: Record<FileViewScreen, FileViewMode> = { drive: 'grid', 'space-files': 'grid' }

interface FileViewState {
  modes: Record<FileViewScreen, FileViewMode>
  setMode: (screen: FileViewScreen, mode: FileViewMode) => void
}

/**
 * Storage that never throws. Reading or writing `localStorage` fails in private
 * windows, with storage blocked, or when the quota is full; the layout choice is
 * a nicety, so it then lives in memory for the session instead of breaking the screen.
 */
const safeStorage: StateStorage = {
  getItem: (name) => {
    try { return localStorage.getItem(name) } catch { return null }
  },
  setItem: (name, value) => {
    try { localStorage.setItem(name, value) } catch { /* kept in memory only */ }
  },
  removeItem: (name) => {
    try { localStorage.removeItem(name) } catch { /* nothing to remove */ }
  },
}

/** Client-only UI state (DESIGN.md "Tài liệu: lưới, danh sách, panel xem trước"): grid by default. */
export const useFileViewStore = create<FileViewState>()(
  persist(
    (set) => ({
      modes: { ...DEFAULT_MODES },
      setMode: (screen, mode) => set((s) => ({ modes: { ...s.modes, [screen]: mode } })),
    }),
    {
      name: 'ngac-file-view',
      storage: createJSONStorage(() => safeStorage),
      // Whatever a stored value says, only known screens with a known mode get through.
      merge: (persisted, current) => {
        const stored = (persisted as Partial<FileViewState> | undefined)?.modes
        const modes = { ...current.modes }
        for (const screen of SCREENS) {
          const mode = stored?.[screen]
          if (mode === 'grid' || mode === 'list') modes[screen] = mode
        }
        return { ...current, modes }
      },
    },
  ),
)

/** The layout of one screen and the way to change it. */
export function useFileViewMode(screen: FileViewScreen): [FileViewMode, (mode: FileViewMode) => void] {
  const mode = useFileViewStore((s) => s.modes[screen])
  const setMode = useFileViewStore((s) => s.setMode)
  return [mode, (next) => setMode(screen, next)]
}
