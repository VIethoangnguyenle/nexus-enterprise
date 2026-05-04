import { useState } from 'react'
import { useInviteMember } from '../hooks/useWorkspaces'
import { Button, Input } from './primitives'
import { UserPlus, Check, AlertCircle } from 'lucide-react'

interface InviteMemberFormProps {
  wsId: string
}

/** Compact form to invite a user to the workspace by username. */
export function InviteMemberForm({ wsId }: InviteMemberFormProps) {
  const [username, setUsername] = useState('')
  const invite = useInviteMember(wsId)

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = username.trim()
    if (!trimmed) return
    invite.mutate(trimmed, {
      onSuccess: () => setUsername(''),
    })
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-2">
      <label className="text-[11px] font-bold text-on-surface-variant uppercase tracking-widest">
        Invite Member
      </label>
      <div className="flex gap-2">
        <Input
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          placeholder="Enter username"
          className="flex-1"
        />
        <Button
          type="submit"
          disabled={!username.trim() || invite.isPending}
          className="flex items-center gap-2"
        >
          <UserPlus size={14} />
          Invite
        </Button>
      </div>
      {invite.isSuccess && (
        <div className="flex items-center gap-2 text-xs text-tertiary animate-fade-in">
          <Check size={12} /> Invited successfully
        </div>
      )}
      {invite.isError && (
        <div className="flex items-center gap-2 text-xs text-error animate-fade-in">
          <AlertCircle size={12} /> {invite.error?.message || 'Failed to invite'}
        </div>
      )}
    </form>
  )
}
