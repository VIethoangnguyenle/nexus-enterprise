import { useChannelDrive } from '../../hooks/useDrive'
import { FilePreviewCard } from './FilePreviewCard'
import { Heading, Spinner, Text } from '../primitives'
import { FolderOpen, X, Paperclip } from 'lucide-react'

interface ChannelDrivePanelProps {
  wsId: string
  channelId: string
  onClose: () => void
}

/** Slide-in panel showing all files uploaded in a channel's drive. */
export function ChannelDrivePanel({ wsId, channelId, onClose }: ChannelDrivePanelProps) {
  const { data, isLoading, error } = useChannelDrive(wsId, channelId)
  const items = data?.items || []
  const files = items.filter(i => i.item_type === 'file' && i.status === 'active')

  return (
    <div className="w-[300px] flex-shrink-0 flex flex-col bg-surface-container-lowest animate-slide-left
      border-l border-outline-variant">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-outline-variant">
        <div className="flex items-center gap-2">
          <FolderOpen size={16} className="text-on-surface-variant" />
          <h3 className="text-sm font-semibold text-on-surface">Channel Files</h3>
        </div>
        <button
          onClick={onClose}
          className="w-7 h-7 flex items-center justify-center rounded text-on-surface-variant
            hover:text-on-surface hover:bg-surface-container-high cursor-pointer border-none
            bg-transparent transition-colors"
        >
          <X size={16} />
        </button>
      </div>

      {/* File count */}
      <div className="px-4 py-2 border-b border-outline-variant">
        <span className="text-xs text-on-surface-variant">
          {files.length} {files.length === 1 ? 'file' : 'files'}
        </span>
      </div>

      {/* File list */}
      <div className="flex-1 overflow-y-auto px-3 py-2">
        {error ? (
          <div className="py-6 text-center">
            <span className="text-xs text-on-surface-variant">Failed to load files</span>
          </div>
        ) : isLoading ? (
          <div className="flex justify-center py-6"><Spinner /></div>
        ) : files.length === 0 ? (
          <div className="py-8 text-center">
            <FolderOpen size={32} className="block mb-2 opacity-40 mx-auto" />
            <span className="text-xs text-on-surface-variant block">No files uploaded yet</span>
            <span className="text-xs text-on-surface-variant mt-1 flex items-center gap-1 justify-center">
              Use <Paperclip size={12} className="inline" /> in the chat to upload files
            </span>
          </div>
        ) : (
          <div className="flex flex-col gap-1">
            {files.map(file => (
              <FilePreviewCard key={file.id} item={file} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
