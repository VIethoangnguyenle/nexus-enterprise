import { useState } from 'react'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { TreeView, type TreeNode } from './TreeView'

const kids: Record<string, TreeNode[]> = {
  a: [{ id: 'a1', label: 'Tháng 9' }, { id: 'a2', label: 'Tháng 10', hasChildren: false }],
  b: [],
}
const roots: TreeNode[] = [{ id: 'a', label: 'Đối soát' }, { id: 'b', label: 'Hợp đồng' }, { id: 'c', label: 'Báo cáo quý', hasChildren: false }]

const asked: string[] = []
function useChildren(id: string) {
  asked.push(id)
  return { nodes: kids[id], isLoading: id === 'loading', isError: id === 'bad' }
}

function Harness({ onSelect = () => {}, start = new Set<string>() }: { onSelect?: (id: string) => void; start?: Set<string> }) {
  const [open, setOpen] = useState(start)
  const [sel, setSel] = useState<string | null>(null)
  return (
    <TreeView
      label="Thư mục"
      roots={roots}
      useChildren={useChildren}
      expanded={open}
      onToggle={(id) => setOpen((o) => { const n = new Set(o); if (n.has(id)) n.delete(id); else n.add(id); return n })}
      selectedId={sel}
      onSelect={(id) => { setSel(id); onSelect(id) }}
    />
  )
}

describe('TreeView', () => {
  it('shows the roots as tree items one level deep, and loads no children until a node opens', () => {
    asked.length = 0
    render(<Harness />)
    const tree = screen.getByRole('tree', { name: 'Thư mục' })
    const items = within(tree).getAllByRole('treeitem')
    expect(items.map((i) => i.textContent)).toEqual(['Đối soát', 'Hợp đồng', 'Báo cáo quý'])
    expect(items.every((i) => i.getAttribute('aria-level') === '1')).toBe(true)
    expect(items[0]).toHaveAttribute('aria-expanded', 'false')
    expect(items[2]).not.toHaveAttribute('aria-expanded')
    expect(asked).toEqual([])
  })

  it('opens with the chevron without selecting, then lists children one level deeper', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(<Harness onSelect={onSelect} />)
    const first = screen.getByRole('treeitem', { name: 'Đối soát' })
    await user.click(within(first).getByTestId('tree-toggle'))
    expect(onSelect).not.toHaveBeenCalled()
    expect(first).toHaveAttribute('aria-expanded', 'true')
    const child = screen.getByRole('treeitem', { name: 'Tháng 9' })
    expect(child).toHaveAttribute('aria-level', '2')
  })

  it('selects on click and marks only that item selected', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(<Harness onSelect={onSelect} />)
    await user.click(screen.getByRole('treeitem', { name: 'Hợp đồng' }))
    expect(onSelect).toHaveBeenCalledWith('b')
    expect(screen.getByRole('treeitem', { name: 'Hợp đồng' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('treeitem', { name: 'Đối soát' })).toHaveAttribute('aria-selected', 'false')
  })

  it('moves with the arrow keys: down, right to open, left to close, left again to the parent', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    await user.tab()
    expect(screen.getByRole('treeitem', { name: 'Đối soát' })).toHaveFocus()

    await user.keyboard('{ArrowRight}')
    expect(screen.getByRole('treeitem', { name: 'Đối soát' })).toHaveAttribute('aria-expanded', 'true')
    await user.keyboard('{ArrowRight}')
    expect(screen.getByRole('treeitem', { name: 'Tháng 9' })).toHaveFocus()
    await user.keyboard('{ArrowDown}')
    expect(screen.getByRole('treeitem', { name: 'Tháng 10' })).toHaveFocus()
    await user.keyboard('{ArrowLeft}')
    expect(screen.getByRole('treeitem', { name: 'Đối soát' })).toHaveFocus()
    await user.keyboard('{ArrowLeft}')
    expect(screen.getByRole('treeitem', { name: 'Đối soát' })).toHaveAttribute('aria-expanded', 'false')
    await user.keyboard('{ArrowDown}{ArrowDown}')
    expect(screen.getByRole('treeitem', { name: 'Báo cáo quý' })).toHaveFocus()
  })

  it('Enter selects the focused item', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(<Harness onSelect={onSelect} />)
    await user.tab()
    await user.keyboard('{ArrowDown}{Enter}')
    expect(onSelect).toHaveBeenCalledWith('b')
  })

  it('says so when an open node has no subfolders', () => {
    render(<Harness start={new Set(['b'])} />)
    expect(screen.getByText('Không có thư mục con')).toBeInTheDocument()
  })

  it('stays tabbable when the active item is no longer rendered', async () => {
    const user = userEvent.setup()
    const { rerender } = render(<Fixed list={roots} selected="b" />)
    await user.click(screen.getByRole('treeitem', { name: 'Đối soát' }))
    // The focused node disappears (its parent closed, a folder was trashed).
    rerender(<Fixed list={roots.slice(1)} selected="b" />)
    const stops = screen.getAllByRole('treeitem').filter((i) => i.tabIndex === 0)
    expect(stops.map((i) => i.textContent)).toEqual(['Hợp đồng'])

    rerender(<Fixed list={roots.slice(1)} selected={null} />)
    expect(screen.getAllByRole('treeitem').filter((i) => i.tabIndex === 0).map((i) => i.textContent)).toEqual(['Hợp đồng'])
  })
})

function Fixed({ list, selected }: { list: TreeNode[]; selected: string | null }) {
  return (
    <TreeView
      label="Thư mục"
      roots={list}
      useChildren={useChildren}
      expanded={new Set()}
      onToggle={() => {}}
      selectedId={selected}
      onSelect={() => {}}
    />
  )
}

describe('TreeView trailing figure', () => {
  it('puts a figure at the right of each row when asked', () => {
    render(
      <TreeView
        label="Phòng ban"
        roots={roots}
        useChildren={useChildren}
        expanded={new Set()}
        onToggle={() => {}}
        onSelect={() => {}}
        trailing={(n) => (n.id === 'a' ? '12' : null)}
      />,
    )
    const first = screen.getByRole('treeitem', { name: /Đối soát/ })
    expect(within(first).getByText('12').className).toMatch(/tnum/)
    expect(within(screen.getByRole('treeitem', { name: /Hợp đồng/ })).queryByText('12')).toBeNull()
  })
})
