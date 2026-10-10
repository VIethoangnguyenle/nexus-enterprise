import { motion } from 'motion/react'
import type { MyInvitation } from '../../api/invitations'
import { expiryLabel } from '../../lib/auth-flow'
import { useMotionPresets } from '../../lib/motion'
import { Avatar, Button } from '../primitives'

interface InvitationRowProps {
  invitation: MyInvitation
  /** This row's own request is out. */
  busy: boolean
  /** Some request is out: nothing else can be pressed. */
  locked: boolean
  onAccept: () => void
  onDecline: () => void
}

/**
 * One offer: who invited you, to what, in which role, and when it lapses. The
 * accent wash marks it as the thing that needs an answer.
 */
export function InvitationRow({ invitation: inv, busy, locked, onAccept, onDecline }: InvitationRowProps) {
  const m = useMotionPresets()
  const detail = [inv.role_name, inv.department_name, expiryLabel(inv.expires_at)].filter(Boolean).join(' · ')
  return (
    <motion.li {...m.row} className="overflow-hidden list-none">
      <div className="grid grid-cols-[2.5rem_minmax(0,1fr)] items-center gap-x-3 gap-y-2.5 p-3 rounded-surface bg-accent-wash">
        <Avatar name={inv.inviter_name} size={40} />
        <span className="min-w-0 text-sm leading-snug text-ink">
          <b className="font-semibold">{inv.inviter_name}</b> mời bạn vào <b className="font-semibold">{inv.workspace_name}</b>
          {detail && <small className="block text-sm text-ink-muted">{detail}</small>}
        </span>
        {/* Under the text, not beside it: the sentence needs the width. */}
        <div className="col-start-2 flex gap-2">
          <Button variant="primary" size="md" className="h-11 sm:h-9 flex-1 sm:flex-none" loading={busy} disabled={locked} onClick={onAccept}>
            Tham gia
          </Button>
          <Button variant="ghost" size="md" className="h-11 sm:h-9" disabled={locked} onClick={onDecline}>
            Từ chối
          </Button>
        </div>
      </div>
    </motion.li>
  )
}
