import { create } from 'zustand'

type AppModule = 'messaging' | 'documents' | 'drive' | 'assets' | 'settings'

interface UiState {
  sidebarCollapsed: boolean
  toggleSidebar: () => void
  activeModal: string | null
  openModal: (id: string) => void
  closeModal: () => void
  activeModule: AppModule
  setActiveModule: (m: AppModule) => void
  listPanelOpen: boolean
  toggleListPanel: () => void
  setListPanelOpen: (open: boolean) => void
}

export const useUiStore = create<UiState>()((set) => ({
  sidebarCollapsed: false,
  toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
  activeModal: null,
  openModal: (id) => set({ activeModal: id }),
  closeModal: () => set({ activeModal: null }),
  activeModule: 'messaging',
  setActiveModule: (m) => set({ activeModule: m, listPanelOpen: true }),
  listPanelOpen: true,
  toggleListPanel: () => set((s) => ({ listPanelOpen: !s.listPanelOpen })),
  setListPanelOpen: (open) => set({ listPanelOpen: open }),
}))
