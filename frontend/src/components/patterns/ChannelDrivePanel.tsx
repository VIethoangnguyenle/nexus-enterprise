import { useChannelDrive } from '../../hooks/useDrive'
import { FilePreviewCard } from './FilePreviewCard'
import { Heading, Spinner, Text } from '../primitives'

interface ChannelDrivePanelProps {
  channelId: string
  onClose: () => void
}

/** Slide-in panel showing all files uploaded in a channel's drive. */
export function ChannelDrivePanel({ channelId, onClose }: ChannelDrivePanelProps) {
  const { data, isLoading, error } = useChannelDrive(channelId)
  const items = data?.items || []
  const files = items.filter(i => i.item_type === 'file' && i.status === 'active')

  return (
    <div className="w-[300px] flex-shrink-0 flex flex-col bg-bg-secondary animate-slide-left
      border-l border-border">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-border">
        <div className="flex items-center gap-2">
          <span className="text-base">📁</span>
          <Heading as="h4">Channel Files</Heading>
        </div>
        <button
          onClick={onClose}
          className="w-6 h-6 flex items-center justify-center rounded text-text-muted
            hover:text-text-primary hover:bg-bg-hover cursor-pointer border-none
            bg-transparent transition-colors"
        >
          ✕
        </button>
      </div>

      {/* File count */}
      <div className="px-4 py-2 border-b border-border/50">
        <Text variant="caption" muted>
          {files.length} {files.length === 1 ? 'file' : 'files'}
        </Text>
      </div>

      {/* File list */}
      <div className="flex-1 overflow-y-auto px-3 py-2">
        {error ? (
          <div className="py-6 text-center">
            <Text variant="caption" muted>Failed to load files</Text>
          </div>
        ) : isLoading ? (
          <div className="flex justify-center py-6"><Spinner /></div>
        ) : files.length === 0 ? (
          <div className="py-8 text-center">
            <span className="text-3xl block mb-2 opacity-40">📂</span>
            <Text variant="caption" muted>No files uploaded yet</Text>
            <Text variant="caption" muted className="mt-1">
              Use 📎 in the chat to upload files
            </Text>
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
