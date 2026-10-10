import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { AlertCircle, Building2, ChevronRight, Plus, RefreshCw } from 'lucide-react'
import { logoutSession } from '../../api/client'
import type { MyInvitation } from '../../api/invitations'
import { useAcceptInvitation, useDeclineInvitation, useMyInvitations } from '../../hooks/useInvitations'
import { useMe, useMyWorkspaces, useProviders, useSwitchToWorkspace } from '../../hooks/useAuth'
import { describeAuthError, verifyErrorMessage, workspaceRowDetail, type DescribedError } from '../../lib/auth-flow'
import { staggerDelay, useMotionPresets, withDelay } from '../../lib/motion'
import { workspaceDisplayName } from '../../lib/workspace'
import { keys } from '../../hooks/keys'
import { queryClient } from '../../lib/query-client'
import { Button, Heading, Pressable, SpaceIcon, Spinner, Text, toast } from '../primitives'
import { InvitationRow } from './InvitationRow'
import { Notice } from './Notice'
import { VerifyEmailNote } from './VerifyEmailNote'

const WIDE = 'w-full h-11'

/**
 * Chọn workspace (design/mockups/auth.html §4): offers waiting for an answer on
 * top, then the person's own workspaces.
 *
 * Opening a workspace first re-scopes the session to it, because services that
 * keep per-workspace data read the workspace from the token. One workspace and
 * nothing to answer goes straight in; the screen never appears for them.
 */
export function WorkspaceSelectScreen() {
  const m = useMotionPresets()
  const navigate = useNavigate()
  const { ws: linked, verified: justVerified, verify_error: verifyErrorCode } =
    useSearch({ strict: false }) as { ws?: string; verified?: string; verify_error?: string }

  const me = useMe()
  const providers = useProviders()
  // The list is the person's own memberships (active rows only): exactly the
  // set the server will let them re-scope to when they open one. Building it
  // from anything wider (every workspace the graph reaches) would offer rows
  // that open to a refusal.
  const mine = useMyWorkspaces()
  const invitations = useMyInvitations()
  const accept = useAcceptInvitation()
  const decline = useDeclineInvitation()
  const switchTo = useSwitchToWorkspace()

  const [entering, setEntering] = useState<string | null>(null)
  const [accepting, setAccepting] = useState<string | null>(null)
  const [enterProblem, setEnterProblem] = useState<DescribedError | null>(null)
  const [offerProblem, setOfferProblem] = useState<DescribedError | null>(null)
  const listRef = useRef<HTMLUListElement>(null)

  const account = me.data
  const email = account?.email ?? ''
  const verified = account?.email_verified === true
  const canHaveOffers = !!email && verified
  const list = mine.data ?? []
  const offers: MyInvitation[] = canHaveOffers ? (invitations.data ?? []) : []

  const enter = useCallback(
    (workspaceId: string) => {
      setEnterProblem(null)
      setEntering(workspaceId)
      switchTo.mutate(workspaceId, {
        onSuccess: () => void navigate({ to: '/channels', search: { ws: workspaceId } }),
        onError: (err) => {
          setEntering(null)
          setEnterProblem(describeAuthError(err, 'enter-workspace'))
        },
      })
    },
    [switchTo, navigate],
  )

  // Decide once, on first load, whether there is anything to choose. Deciding
  // again later would throw a person into a workspace the moment they decline
  // their last offer.
  const decided = useRef(false)
  const settled = mine.isSuccess && !!account && (!canHaveOffers || invitations.isSuccess || invitations.isError)
  useEffect(() => {
    if (decided.current || !settled) return
    decided.current = true
    const linkedOne = list.find((w) => w.id === linked)
    if (linkedOne) return enter(linkedOne.id)
    const nothingToAnswer = !canHaveOffers || (invitations.isSuccess && offers.length === 0)
    const nothingToExplain = !email || verified
    if (list.length === 1 && nothingToAnswer && nothingToExplain) enter(list[0]!.id)
  }, [settled, list, linked, canHaveOffers, invitations.isSuccess, offers.length, email, verified, enter])

  // Google sends a person who was proving their address back with the outcome in
  // the address. Say it once, and clear it so a reload does not say it again.
  const [verifyProblem] = useState(verifyErrorCode)
  const reported = useRef(false)
  useEffect(() => {
    if (reported.current || (!justVerified && !verifyErrorCode)) return
    reported.current = true
    if (justVerified) {
      toast('Đã xác minh email.')
      void queryClient.invalidateQueries({ queryKey: keys.auth.me() })
      void queryClient.invalidateQueries({ queryKey: keys.auth.invitations() })
    }
    void navigate({ to: '/workspace-select', search: linked ? { ws: linked } : {}, replace: true })
  }, [justVerified, verifyErrorCode, linked, navigate])

  const answer = (inv: MyInvitation, join: boolean) => {
    setOfferProblem(null)
    if (join) {
      setAccepting(inv.id)
      accept.mutate(inv.id, {
        onSuccess: (res) => {
          setAccepting(null)
          if (!res.role_applied || !res.department_applied) {
            toast(
              `Bạn đã vào ${res.workspace_name}, nhưng vai trò đi kèm lời mời chưa được áp dụng vì người mời không còn đủ quyền. Nhờ quản trị viên gán lại.`,
              { tone: 'info' },
            )
          }
          enter(res.workspace_id)
        },
        onError: (err) => {
          setAccepting(null)
          setOfferProblem(describeAuthError(err, 'accept-invitation'))
          // A refusal that means the offer is gone: let the list catch up.
          void invitations.refetch()
        },
      })
      return
    }
    decline.mutate(inv.id, {
      onSuccess: () => toast('Đã từ chối lời mời.'),
      onError: (err) => {
        setOfferProblem(describeAuthError(err, 'decline-invitation'))
        void invitations.refetch()
      },
    })
  }

  const changeAccount = async () => {
    await logoutSession()
    void navigate({ to: '/login' })
  }

  const moveFocus = (e: KeyboardEvent<HTMLUListElement>) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
    const rows = Array.from(listRef.current?.querySelectorAll<HTMLButtonElement>('[data-ws-row]:not(:disabled)') ?? [])
    const at = rows.indexOf(document.activeElement as HTMLButtonElement)
    if (at === -1) return
    e.preventDefault()
    rows[Math.min(Math.max(at + (e.key === 'ArrowDown' ? 1 : -1), 0), rows.length - 1)]?.focus()
  }

  const busy = entering !== null || accepting !== null || decline.isPending

  return (
    <div className="grid gap-5">
      <div className="grid gap-1.5">
        <Heading as="h1" look="page">Chọn workspace</Heading>
        <Text variant="body" muted className="block">
          {email ? `Đăng nhập bằng ${email}. ` : account ? `Đăng nhập với tên ${account.display_name}. ` : ''}
          <Button variant="link" size="link" onClick={() => void changeAccount()}>Đổi tài khoản</Button>
        </Text>
      </div>

      {enterProblem && <Notice>{enterProblem.message}</Notice>}
      {verifyProblem && <Notice>{verifyErrorMessage(verifyProblem, email)}</Notice>}
      {offerProblem && <Notice>{offerProblem.message}</Notice>}

      {account && !!email && !verified && (
        <VerifyEmailNote
          email={email}
          providers={providers.data}
          purpose="invitations"
          onVerified={() => {
            toast('Đã xác minh email.')
            void queryClient.invalidateQueries({ queryKey: keys.auth.me() })
            void queryClient.invalidateQueries({ queryKey: keys.auth.invitations() })
          }}
        />
      )}
      {account && !email && (
        <Notice tone="info">
          Lời mời được gửi tới email. Tài khoản này đăng ký bằng số điện thoại nên chưa có lời mời nào.
        </Notice>
      )}

      {canHaveOffers && invitations.isError && (
        <Notice>
          <span className="flex flex-wrap items-center justify-between gap-2">
            Không tải được lời mời.
            <Button variant="link" size="link" onClick={() => void invitations.refetch()}>Thử lại</Button>
          </span>
        </Notice>
      )}

      {offers.length > 0 && (
        <section aria-labelledby="offers-label" className="grid gap-2">
          <div id="offers-label" className="text-label text-ink-muted">Lời mời</div>
          <ul className="grid gap-2 m-0 p-0">
            <AnimatePresence initial={false}>
              {offers.map((inv) => (
                <InvitationRow
                  key={inv.id}
                  invitation={inv}
                  busy={accepting === inv.id}
                  locked={busy}
                  onAccept={() => answer(inv, true)}
                  onDecline={() => answer(inv, false)}
                />
              ))}
            </AnimatePresence>
          </ul>
        </section>
      )}

      <section aria-labelledby="own-label" className="grid gap-2">
        {(list.length > 0 || offers.length > 0) && <div id="own-label" className="text-label text-ink-muted">Workspace của bạn</div>}

        {mine.isError ? (
          <Problem
            text="Không tải được danh sách workspace. Kiểm tra kết nối rồi thử lại."
            action={<Button variant="soft" size="sm" onClick={() => void mine.refetch()}>
              <RefreshCw size={16} strokeWidth={1.75} aria-hidden="true" />
              Thử lại
            </Button>}
          />
        ) : mine.isPending || !account ? (
          <ul className="grid gap-2 m-0 p-0" aria-busy="true">
            {[0, 1].map((i) => <RowSkeleton key={i} />)}
          </ul>
        ) : list.length === 0 ? (
          offers.length === 0 && (
            <Problem
              icon="empty"
              text={`Bạn chưa thuộc workspace nào. Nhờ quản trị viên mời ${email || 'bạn'}, hoặc tạo workspace cho nhóm của bạn.`}
              action={<Button variant="primary" size="sm" onClick={() => void navigate({ to: '/onboarding' })}>
                <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
                Tạo workspace
              </Button>}
            />
          )
        ) : (
          <ul ref={listRef} onKeyDown={moveFocus} className="grid gap-2 m-0 p-0">
            {list.map((w, i) => {
              const name = workspaceDisplayName(w.name)
              const detail = workspaceRowDetail(w)
              return (
                <motion.li
                  key={w.id}
                  {...withDelay(m.route, staggerDelay(i, m.reduced))}
                  className="list-none"
                >
                  <Pressable
                    data-ws-row
                    disabled={busy}
                    onClick={() => enter(w.id)}
                    className="w-full grid grid-cols-[2.5rem_minmax(0,1fr)_auto] items-center gap-3 p-3 rounded-surface
                      bg-raised hover:bg-hover transition-colors duration-quick"
                  >
                    <SpaceIcon name={name} hueKey={w.id} size={40} />
                    <span className="min-w-0">
                      <b className="block truncate font-semibold text-ink">{name}</b>
                      {detail && <small className="block truncate text-sm text-ink-muted">{detail}</small>}
                    </span>
                    {entering === w.id
                      ? <Spinner size="sm" />
                      : <ChevronRight size={16} strokeWidth={1.75} className="text-ink-muted" aria-hidden="true" />}
                  </Pressable>
                </motion.li>
              )
            })}
          </ul>
        )}
      </section>

      <Button variant="soft" size="md" className={WIDE} disabled={busy} onClick={() => void navigate({ to: '/onboarding' })}>
        <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
        Tạo workspace mới
      </Button>
    </div>
  )
}

function RowSkeleton() {
  return (
    <li
      data-skeleton
      className="list-none grid grid-cols-[2.5rem_minmax(0,1fr)] items-center gap-3 p-3 rounded-surface bg-raised"
    >
      <div className="skeleton w-10 h-10 rounded-surface" />
      <div className="grid gap-2">
        <div className="skeleton h-3.5 w-3/5 rounded-md" />
        <div className="skeleton h-2.5 w-2/5 rounded-md" />
      </div>
    </li>
  )
}

/** Icon, one sentence, one action (DESIGN.md §6 Empty / Error state). */
function Problem({ text, action, icon = 'error' }: { text: string; action: React.ReactNode; icon?: 'error' | 'empty' }) {
  const Icon = icon === 'error' ? AlertCircle : Building2
  return (
    <div className="grid justify-items-center gap-3 px-4 py-8 text-center rounded-surface bg-raised">
      <span
        aria-hidden="true"
        className={`grid place-items-center w-10 h-10 rounded-overlay ${icon === 'error' ? 'bg-danger-wash text-danger' : 'bg-sunk text-ink-muted'}`}
      >
        <Icon size={20} strokeWidth={1.75} />
      </span>
      <Text variant="body" className="block max-w-72">{text}</Text>
      {action}
    </div>
  )
}
