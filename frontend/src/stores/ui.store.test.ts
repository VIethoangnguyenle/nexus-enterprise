import { beforeEach, describe, expect, it } from 'vitest'
import { useUiStore } from './ui.store'

beforeEach(() => {
  localStorage.clear()
  useUiStore.setState({ activeModule: 'messaging', listPanelOpen: true, listPanelWidth: 280, starredChannels: [] })
})

describe('ui store', () => {
  it('switching module reopens a collapsed list panel', () => {
    useUiStore.getState().setListPanelOpen(false)
    useUiStore.getState().setActiveModule('drive')
    const s = useUiStore.getState()
    expect(s.activeModule).toBe('drive')
    expect(s.listPanelOpen).toBe(true)
  })

  it('toggles the list panel', () => {
    useUiStore.getState().toggleListPanel()
    expect(useUiStore.getState().listPanelOpen).toBe(false)
    useUiStore.getState().toggleListPanel()
    expect(useUiStore.getState().listPanelOpen).toBe(true)
  })

  it('sets the panel width and resets it to the default', () => {
    useUiStore.getState().setListPanelWidth(400)
    expect(useUiStore.getState().listPanelWidth).toBe(400)
    useUiStore.getState().resetListPanelWidth()
    expect(useUiStore.getState().listPanelWidth).toBe(280)
  })

  it('stars and unstars a channel without disturbing the others', () => {
    const { toggleStarChannel } = useUiStore.getState()
    toggleStarChannel('a')
    toggleStarChannel('b')
    toggleStarChannel('a')
    expect(useUiStore.getState().starredChannels).toEqual(['b'])
  })

  it('persists width and stars only, not which module or panel state is active', () => {
    useUiStore.getState().setListPanelWidth(333)
    useUiStore.getState().setActiveModule('assets')
    const saved = JSON.parse(localStorage.getItem('ngac-ui') ?? '{}').state
    expect(saved).toEqual({ listPanelWidth: 333, starredChannels: [] })
  })
})
