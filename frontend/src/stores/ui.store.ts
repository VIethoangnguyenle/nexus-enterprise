import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type AppModule = 'messaging' | 'documents' | 'drive' | 'assets' | 'contacts' | 'approval' | 'admin' | 'settings'

/** Default width (px) for the resizable list panel. */
const LIST_PANEL_DEFAULT_WIDTH = 280

interface UiState {
  activeModal: string | null
  openModal: (id: string) => void
  closeModal: () => void
  activeModule: AppModule
  setActiveModule: (m: AppModule) => void
  listPanelOpen: boolean
  toggleListPanel: () => void
  setListPanelOpen: (open: boolean) => void
  listPanelWidth: number
  setListPanelWidth: (w: number) => void
  resetListPanelWidth: () => void
  peekPanelOpen: boolean
  peekPanelContent: { type: string; id: string } | null
  openPeekPanel: (type: string, id: string) => void
  closePeekPanel: () => void
  starredChannels: string[]
  toggleStarChannel: (channelId: string) => void
}

export const useUiStore = create<UiState>()(
  persist(
    (set) => ({
      activeModal: null,
      openModal: (id) => set({ activeModal: id }),
      closeModal: () => set({ activeModal: null }),
      activeModule: 'messaging',
      setActiveModule: (m) => set({ activeModule: m, listPanelOpen: true }),
      listPanelOpen: true,
      toggleListPanel: () => set((s) => ({ listPanelOpen: !s.listPanelOpen })),
      setListPanelOpen: (open) => set({ listPanelOpen: open }),
      listPanelWidth: LIST_PANEL_DEFAULT_WIDTH,
      setListPanelWidth: (w) => set({ listPanelWidth: w }),
      resetListPanelWidth: () => set({ listPanelWidth: LIST_PANEL_DEFAULT_WIDTH }),
      peekPanelOpen: false,
      peekPanelContent: null,
      openPeekPanel: (type, id) => set({ peekPanelOpen: true, peekPanelContent: { type, id } }),
      closePeekPanel: () => set({ peekPanelOpen: false, peekPanelContent: null }),
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
