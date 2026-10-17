import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type AppModule = 'messaging' | 'calendar' | 'documents' | 'drive' | 'assets' | 'contacts' | 'approval' | 'admin' | 'settings'

/** Default width (px) for the resizable list panel. */
const LIST_PANEL_DEFAULT_WIDTH = 280

interface UiState {
  activeModule: AppModule
  setActiveModule: (m: AppModule) => void
  listPanelOpen: boolean
  toggleListPanel: () => void
  setListPanelOpen: (open: boolean) => void
  listPanelWidth: number
  setListPanelWidth: (w: number) => void
  resetListPanelWidth: () => void
  starredChannels: string[]
  toggleStarChannel: (channelId: string) => void
  /** True while the phone's Chat bottom bar is on screen; what floats above the bottom edge makes room for it. */
  chatBarShown: boolean
  setChatBarShown: (shown: boolean) => void
}

export const useUiStore = create<UiState>()(
  persist(
    (set) => ({
      activeModule: 'messaging',
      setActiveModule: (m) => set({ activeModule: m, listPanelOpen: true }),
      listPanelOpen: true,
      toggleListPanel: () => set((s) => ({ listPanelOpen: !s.listPanelOpen })),
      setListPanelOpen: (open) => set({ listPanelOpen: open }),
      listPanelWidth: LIST_PANEL_DEFAULT_WIDTH,
      setListPanelWidth: (w) => set({ listPanelWidth: w }),
      resetListPanelWidth: () => set({ listPanelWidth: LIST_PANEL_DEFAULT_WIDTH }),
      chatBarShown: false,
      setChatBarShown: (shown) => set({ chatBarShown: shown }),
      starredChannels: [],
      toggleStarChannel: (channelId) => set((s) => ({
        starredChannels: s.starredChannels.includes(channelId)
          ? s.starredChannels.filter(id => id !== channelId)
          : [...s.starredChannels, channelId],
      })),
    }),
    {
      name: 'ngac-ui',
      partialize: (state) => ({ listPanelWidth: state.listPanelWidth, starredChannels: state.starredChannels }),
    },
  ),
)
