import { Check, X as XIcon } from 'lucide-react'
import { Button } from '../primitives'

interface BatchActionBarProps {
  selectedCount: number
  onBatchApprove: () => void
  onClear: () => void
  isProcessing?: boolean
}

/** Floating bottom bar for batch approval actions — visible when items selected. */
export function BatchActionBar({ selectedCount, onBatchApprove, onClear, isProcessing }: BatchActionBarProps) {
  if (selectedCount === 0) return null

  return (
    <div className="fixed bottom-0 lg:bottom-0 inset-x-0 z-40 px-4 pb-4
      lg:left-[280px] pointer-events-none">
      {/* Offset for MobileNav on mobile */}
      <div className="mb-14 lg:mb-0 pointer-events-auto">
        <div className="max-w-2xl mx-auto flex items-center justify-between gap-4
          bg-surface-container-highest border border-outline-variant rounded-xl px-4 py-3 shadow-lg">
          <span className="text-sm font-medium text-on-surface">
            {selectedCount} selected
          </span>
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={onClear}
              className="text-on-surface-variant"
            >
              <XIcon size={14} />
              <span className="ml-1">Clear</span>
            </Button>
            <Button
              size="sm"
              onClick={onBatchApprove}
              disabled={isProcessing}
              className="bg-success text-on-success hover:bg-success/90"
            >
              <Check size={14} />
              <span className="ml-1">Batch Approve</span>
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
