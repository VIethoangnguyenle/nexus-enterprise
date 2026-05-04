import { useCallback, useEffect, useRef, useState } from 'react'

interface UseResizableOptions {
  /** Resize direction. */
  direction: 'horizontal' | 'vertical'
  /** Default size in px. */
  defaultSize: number
  /** Minimum size in px. */
  minSize: number
  /** Maximum size in px. */
  maxSize: number
  /** Called when size changes (e.g. to persist to store). */
  onResize?: (size: number) => void
  /** Initial size (overrides defaultSize, useful for persisted values). */
  initialSize?: number
}

interface UseResizableReturn {
  /** Current size in px. */
  size: number
  /** Whether the user is currently dragging. */
  isDragging: boolean
  /** Spread these props on the resize handle element. */
  handleProps: {
    onMouseDown: (e: React.MouseEvent) => void
    onDoubleClick: () => void
    style: React.CSSProperties
  }
  /** Reset to default size. */
  resetSize: () => void
}

/** Drag-to-resize hook with min/max constraints and RAF throttling. */
export function useResizable(options: UseResizableOptions): UseResizableReturn {
  const { direction, defaultSize, minSize, maxSize, onResize, initialSize } = options
  const [size, setSize] = useState(initialSize ?? defaultSize)
  const [isDragging, setIsDragging] = useState(false)
  const startRef = useRef({ pos: 0, size: 0 })
  const rafRef = useRef<number>(0)

  const clamp = useCallback(
    (v: number) => Math.max(minSize, Math.min(maxSize, v)),
    [minSize, maxSize],
  )

  const handleMouseDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault()
      const pos = direction === 'horizontal' ? e.clientX : e.clientY
      startRef.current = { pos, size }
      setIsDragging(true)
    },
    [direction, size],
  )

  useEffect(() => {
    if (!isDragging) return

    const onMouseMove = (e: MouseEvent) => {
      cancelAnimationFrame(rafRef.current)
      rafRef.current = requestAnimationFrame(() => {
        const pos = direction === 'horizontal' ? e.clientX : e.clientY
        const delta = pos - startRef.current.pos
        const next = clamp(startRef.current.size + delta)
        setSize(next)
        onResize?.(next)
      })
    }

    const onMouseUp = () => {
      cancelAnimationFrame(rafRef.current)
      setIsDragging(false)
    }

    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
    document.body.style.cursor = direction === 'horizontal' ? 'col-resize' : 'row-resize'
    document.body.style.userSelect = 'none'

    return () => {
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }
  }, [isDragging, direction, clamp, onResize])

  const resetSize = useCallback(() => {
    setSize(defaultSize)
    onResize?.(defaultSize)
  }, [defaultSize, onResize])

  const handleDoubleClick = useCallback(() => {
    resetSize()
  }, [resetSize])

  const handleProps = {
    onMouseDown: handleMouseDown,
    onDoubleClick: handleDoubleClick,
    style: {
      cursor: direction === 'horizontal' ? 'col-resize' : 'row-resize',
    } as React.CSSProperties,
  }

  return { size, isDragging, handleProps, resetSize }
}
