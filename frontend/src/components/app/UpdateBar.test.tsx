import { act, fireEvent, render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { usePwaStore } from '../../lib/pwa'
import { UpdateBar } from './UpdateBar'

const apply = vi.fn()
let client: QueryClient

beforeEach(() => {
  vi.useFakeTimers()
  apply.mockReset()
  client = new QueryClient()
  usePwaStore.setState({ needRefresh: false, applyUpdate: apply })
})
afterEach(() => {
  vi.useRealTimers()
  document.body.innerHTML = ''
})

const mount = () =>
  render(
    <QueryClientProvider client={client}>
      <UpdateBar />
    </QueryClientProvider>,
  )
const update = () => act(() => usePwaStore.setState({ needRefresh: true }))
const wait = (ms: number) => act(() => { vi.advanceTimersByTime(ms) })

/** A text box the person types into: tracked by its input events, like a real keystroke. */
function typeInto(el: HTMLElement, text: string) {
  if (el instanceof HTMLTextAreaElement || el instanceof HTMLInputElement) el.value = text
  else el.textContent = text
  el.dispatchEvent(new Event('input', { bubbles: true }))
}
function addEditor(): HTMLElement {
  const box = document.createElement('div')
  box.className = 'chat-editor-content'
  box.setAttribute('contenteditable', 'true')
  document.body.appendChild(box)
  return box
}

describe('UpdateBar', () => {
  it('keeps one live region from the start and fills it when a version is waiting', () => {
    mount()
    const region = screen.getByRole('status')
    expect(region).toBeEmptyDOMElement()
    update()
    expect(screen.getByRole('status')).toBe(region)
    expect(region).toHaveTextContent('Có bản mới của Nexus')
  })

  it('activates the waiting worker when Tải lại is pressed', () => {
    mount()
    update()
    fireEvent.click(screen.getByRole('button', { name: 'Tải lại' }))
    expect(apply).toHaveBeenCalledTimes(1)
  })

  it('hides on Để sau and does not reload', () => {
    mount()
    update()
    fireEvent.click(screen.getByRole('button', { name: 'Để sau' }))
    expect(screen.getByRole('status')).toBeEmptyDOMElement()
    expect(apply).not.toHaveBeenCalled()
  })

  it('never reloads under a draft: waits until it is gone', () => {
    const box = addEditor()
    mount()
    update()
    typeInto(box, 'đang soạn dở')
    wait(600)
    fireEvent.click(screen.getByRole('button', { name: 'Tải lại sau khi gửi' }))
    expect(apply).not.toHaveBeenCalled()
    act(() => { box.textContent = '' })
    wait(600)
    expect(apply).not.toHaveBeenCalled() // typed a moment ago: still inside the edit window
    wait(3_000)
    expect(apply).toHaveBeenCalledTimes(1)
  })

  it('Để sau after arming cancels the reload even when the draft is later sent', () => {
    const box = addEditor()
    mount()
    update()
    typeInto(box, 'dở dang')
    wait(600)
    fireEvent.click(screen.getByRole('button', { name: 'Tải lại sau khi gửi' }))
    fireEvent.click(screen.getByRole('button', { name: 'Để sau' }))
    act(() => { box.textContent = '' })
    wait(2_000)
    expect(apply).not.toHaveBeenCalled()
  })

  it('does not treat a pre-filled field the person never touched as a draft', () => {
    const field = document.createElement('textarea')
    field.value = 'mô tả có sẵn'
    document.body.appendChild(field)
    mount()
    update()
    wait(600)
    fireEvent.click(screen.getByRole('button', { name: 'Tải lại' }))
    expect(apply).toHaveBeenCalledTimes(1)
  })

  it('counts an edited input and an edited document as unsaved work', () => {
    const input = document.createElement('input')
    document.body.appendChild(input)
    mount()
    update()
    typeInto(input, 'tiêu đề')
    wait(600)
    expect(screen.getByRole('button', { name: 'Tải lại sau khi gửi' })).toBeInTheDocument()
  })

  it('a document edit holds the reload only for the autosave window', () => {
    const doc = document.createElement('div')
    doc.className = 'doc-prose'
    doc.setAttribute('contenteditable', 'true')
    document.body.appendChild(doc)
    mount()
    update()
    typeInto(doc, 'nội dung')
    wait(600)
    expect(screen.getByRole('button', { name: 'Tải lại sau khi gửi' })).toBeInTheDocument()
    wait(3_500)
    expect(screen.getByRole('button', { name: 'Tải lại' })).toBeInTheDocument()
  })

  it('waits for a send in flight, then reloads', async () => {
    vi.useRealTimers()
    let done!: () => void
    const pending = new Promise<void>((r) => { done = r })
    void client.getMutationCache().build(client, { mutationFn: () => pending }).execute(undefined)
    mount()
    update()
    fireEvent.click(await screen.findByRole('button', { name: 'Tải lại' }))
    expect(apply).not.toHaveBeenCalled()
    await act(async () => { done(); await pending; await new Promise((r) => setTimeout(r, 700)) })
    expect(apply).toHaveBeenCalledTimes(1)
  })

  it('moves to the top when a composer is on screen, so it never covers the send button', () => {
    addEditor()
    mount()
    update()
    wait(600)
    expect(screen.getByText('Có bản mới của Nexus').parentElement).toHaveAttribute('data-placement', 'top')
  })
})
