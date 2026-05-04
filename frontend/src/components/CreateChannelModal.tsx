import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useCreateChannel } from '../hooks/useMessaging'
import { useWorkspaces } from '../hooks/useWorkspaces'
import { Modal } from './composites'
import { Button, Input, Spinner } from './primitives'

interface CreateChannelModalProps {
  onClose: () => void
}

/** Modal for creating a new messaging channel within the active workspace. */
export function CreateChannelModal({ onClose }: CreateChannelModalProps) {
  const navigate = useNavigate()
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const createChannel = useCreateChannel(wsId)
  const [name, setName] = useState('')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = name.trim()
    if (!trimmed || !wsId) return
    createChannel.mutate(
      { name: trimmed, channel_type: 'workspace' },
      {
        onSuccess: (channel) => {
          onClose()
          navigate({ to: '/channels/$channelId', params: { channelId: channel.id } })
        },
      },
    )
  }

  return (
    <Modal onClose={onClose} size="sm">
      <Modal.Title>Create Channel</Modal.Title>
      <form onSubmit={handleSubmit}>
        <Modal.Body>
          {!wsId && (
            <div className="bg-warning-bg text-warning px-3 py-2 rounded-sm text-small">
              No workspace available. Create or join a workspace first.
            </div>
          )}
          {createChannel.error && (
            <div className="bg-danger-bg text-danger px-3 py-2 rounded-sm text-small">
              {createChannel.error.message}
            </div>
          )}
          <Input
            label="Channel Name"
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. general, engineering"
          />
        </Modal.Body>
        <Modal.Actions>
          <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
          <Button type="submit" disabled={!name.trim() || !wsId || createChannel.isPending}>
            {createChannel.isPending ? <Spinner size="sm" /> : 'Create'}
          </Button>
        </Modal.Actions>
      </form>
    </Modal>
  )
}
