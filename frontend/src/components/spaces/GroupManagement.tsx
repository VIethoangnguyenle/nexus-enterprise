import { useMemo, useState, type ReactNode } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { CircleAlert, Plus, UserMinus, ChevronRight } from 'lucide-react'
import type { GroupPolicy, GroupSettings } from '../../api/group-management'
import {
  GROUP_ROLE_LABEL,
  groupActionError,
  useAppointManager,
  useDisbandGroup,
  useGroupManagement,
  useGroupRole,
  useRemoveManager,
  useTransferOwnership,
  useUpdateGroupSettings,
} from '../../hooks/useGroupManagement'
import { useChannelMembers } from '../../hooks/useMessaging'
import type { PeopleDirectory, Person } from '../../lib/people'
import { ConfirmDialog } from '../composites/ConfirmDialog'
import { Avatar, Button, IconButton, Pressable, toast } from '../primitives'
import { EmptyState } from './EmptyState'
import { SidePanel } from './SidePanel'
import { GroupMemberPicker } from './group-management/GroupMemberPicker'
import { membersAsPeople } from './group-management/member-people'

const POLICIES: { field: keyof GroupSettings; title: string; restrictedHint?: string }[] = [
  { field: 'post_policy', title: 'Ai được gửi tin', restrictedHint: 'Thành viên vẫn trả lời trong thread' },
  { field: 'invite_policy', title: 'Ai được thêm người' },
  { field: 'mention_all_policy', title: 'Ai được nhắc @Tất cả' },
]

const CHOICES: { value: GroupPolicy; label: string }[] = [
  { value: 'everyone', label: 'Mọi thành viên' },
  { value: 'managers', label: 'Chỉ chủ nhóm và quản trị nhóm' },
]

type Step = { kind: 'main' } | { kind: 'add-manager' } | { kind: 'transfer' }

/**
 * Quản lý nhóm (DESIGN.md "Cài đặt cuộc trò chuyện", mockup mobile-shell §5):
 * the three rules, the people who run the group, handing it over and ending
 * it. Every choice is the server's decision: a refused one returns to what it
 * was, with a toast. Shown only to the owner and managers; the caller hides it
 * when that stops being true.
 */
export function GroupManagement({ channelId, groupName, people, startWith = 'main', onBack, onClose }: {
  channelId: string
  groupName: string
  people: PeopleDirectory
  /** Open on a step directly (the owner's "Rời nhóm" leads to the transfer). */
  startWith?: Step['kind']
  onBack: () => void
  onClose: () => void
}) {
  const navigate = useNavigate()
  const mgmt = useGroupManagement(channelId)
  const role = useGroupRole(channelId)
  const roster = useChannelMembers(channelId)
  const update = useUpdateGroupSettings(channelId)
  const appoint = useAppointManager(channelId)
  const demote = useRemoveManager(channelId)
  const transfer = useTransferOwnership(channelId)
  const disband = useDisbandGroup(channelId)
  const [step, setStep] = useState<Step>({ kind: startWith })
  const [transferTo, setTransferTo] = useState<Person | null>(null)
  const [disbanding, setDisbanding] = useState(false)

  const m = mgmt.data
  const members = useMemo(() => membersAsPeople(people, roster.data?.members), [people, roster.data])
  const managerIds = useMemo(() => new Set(m?.manager_ids ?? []), [m])
  const owner = members.find((p) => p.userId === m?.owner_id)
  const managers = members.filter((p) => p.userId !== m?.owner_id && managerIds.has(p.userId))

  const choose = (field: keyof GroupSettings, value: GroupPolicy) => {
    if (!m || m[field] === value) return
    const next: GroupSettings = {
      post_policy: m.post_policy,
      invite_policy: m.invite_policy,
      mention_all_policy: m.mention_all_policy,
      [field]: value,
    }
    update.mutate(next, { onError: (err) => toast.error(groupActionError(err, 'đổi cài đặt nhóm')) })
  }

  const addManager = (p: Person) => {
    setStep({ kind: 'main' })
    appoint.mutate(p.userId, {
      onSuccess: () => toast(`Đã đặt ${p.name} làm quản trị nhóm`),
      onError: (err) => toast.error(groupActionError(err, `đặt ${p.name} làm quản trị nhóm`)),
    })
  }

  const removeManager = (p: Person) =>
    demote.mutate(p.userId, {
      onSuccess: () => toast(`${p.name} không còn là quản trị nhóm`),
      onError: (err) => toast.error(groupActionError(err, `gỡ quyền quản trị nhóm của ${p.name}`)),
    })

  const confirmTransfer = () => {
    const p = transferTo
    if (!p) return
    transfer.mutate(p.userId, {
      onSuccess: () => {
        setTransferTo(null)
        setStep({ kind: 'main' })
        toast(`${p.name} là chủ nhóm mới`)
      },
      onError: (err) => {
        setTransferTo(null)
        toast.error(groupActionError(err, 'chuyển quyền chủ nhóm'))
      },
    })
  }

  const confirmDisband = () =>
    disband.mutate(undefined, {
      onSuccess: () => {
        setDisbanding(false)
        toast(`Đã giải tán “${groupName}”`)
        onClose()
        void navigate({ to: '/channels' })
      },
      onError: (err) => {
        setDisbanding(false)
        toast.error(groupActionError(err, 'giải tán nhóm'))
      },
    })

  const picking = step.kind !== 'main'
  const title = step.kind === 'add-manager' ? 'Thêm quản trị nhóm' : step.kind === 'transfer' ? 'Chuyển quyền chủ nhóm' : 'Quản lý nhóm'

  return (
    <SidePanel
      label={title}
      title={title}
      closeLabel="quản lý nhóm"
      onBack={picking && startWith === 'main' ? () => setStep({ kind: 'main' }) : onBack}
      backOnSheet
      onClose={onClose}
    >
      <div className="grid gap-3 px-3 pb-3">
        {mgmt.isError ? (
          <EmptyState
            compact
            icon={<CircleAlert size={24} strokeWidth={1.75} />}
            text="Chưa tải được cài đặt quản lý nhóm."
            action={<Button variant="soft" size="sm" onClick={() => void mgmt.refetch()}>Thử lại</Button>}
          />
        ) : !m ? (
          <div className="grid gap-2" aria-busy="true" aria-label="Đang tải">
            {[0, 1, 2].map((i) => <div key={i} className="skeleton h-24 rounded-2xl" />)}
          </div>
        ) : step.kind === 'add-manager' ? (
          <GroupMemberPicker
            label="Chọn quản trị nhóm"
            candidates={members.filter((p) => p.userId !== m.owner_id && !managerIds.has(p.userId))}
            emptyText="Mọi thành viên đều đã là quản trị nhóm."
            onPick={addManager}
          />
        ) : step.kind === 'transfer' ? (
          <GroupMemberPicker
            label="Chọn chủ nhóm mới"
            candidates={members.filter((p) => p.userId !== m.owner_id)}
            emptyText="Nhóm chưa có thành viên nào khác để nhận quyền chủ nhóm."
            onPick={setTransferTo}
          />
        ) : (
          <>
            {POLICIES.map(({ field, title: heading, restrictedHint }) => (
              <section key={field} className="grid gap-1.5">
                <h3 className="m-0 px-1 text-label text-ink-muted">{heading}</h3>
                <div role="radiogroup" aria-label={heading} className="rounded-2xl bg-raised overflow-hidden">
                  {CHOICES.map((c) => (
                    <PolicyChoice
                      key={c.value}
                      label={c.label}
                      hint={c.value === 'managers' ? restrictedHint : undefined}
                      checked={m[field] === c.value}
                      onChoose={() => choose(field, c.value)}
                    />
                  ))}
                </div>
              </section>
            ))}

            <section className="grid gap-1.5">
              <h3 className="m-0 px-1 text-label text-ink-muted">Người quản lý</h3>
              <ul className="rounded-2xl bg-raised overflow-hidden list-none m-0 p-0">
                {owner && <PersonRow person={owner} role={GROUP_ROLE_LABEL.owner} />}
                {managers.map((p) => (
                  <PersonRow
                    key={p.userId}
                    person={p}
                    role={GROUP_ROLE_LABEL.manager}
                    action={
                      role.isManager ? (
                        <IconButton
                          size="sm"
                          aria-label={`Gỡ quyền quản trị nhóm của ${p.name}`}
                          title="Gỡ quyền quản trị nhóm"
                          onClick={() => removeManager(p)}
                        >
                          <UserMinus size={16} strokeWidth={1.75} />
                        </IconButton>
                      ) : undefined
                    }
                  />
                ))}
                <li className="border-t border-line first:border-t-0">
                  <Pressable
                    onClick={() => setStep({ kind: 'add-manager' })}
                    className="flex items-center gap-3 w-full min-h-13 px-3.5 py-2 text-accent font-medium"
                  >
                    <span className="grid place-items-center w-8 h-8 rounded-md bg-sunk shrink-0">
                      <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
                    </span>
                    <span className="flex-1 min-w-0 text-left">Thêm quản trị nhóm</span>
                  </Pressable>
                </li>
              </ul>
            </section>

            {role.isOwner && (
              <div className="rounded-2xl bg-raised overflow-hidden">
                <Pressable
                  onClick={() => setStep({ kind: 'transfer' })}
                  className="flex items-center gap-3 w-full min-h-13 px-3.5 py-2 font-medium text-ink"
                >
                  <span className="flex-1 min-w-0 text-left">Chuyển quyền chủ nhóm</span>
                  <ChevronRight size={18} strokeWidth={1.75} className="text-ink-subtle shrink-0" aria-hidden="true" />
                </Pressable>
                <Pressable
                  onClick={() => setDisbanding(true)}
                  className="flex items-center gap-3 w-full min-h-13 px-3.5 py-2 border-t border-line text-danger"
                >
                  <span className="flex-1 min-w-0 grid text-left">
                    <span className="font-semibold">Giải tán nhóm</span>
                    <span className="text-sm text-ink-muted">Chỉ chủ nhóm</span>
                  </span>
                </Pressable>
              </div>
            )}
          </>
        )}
      </div>

      <ConfirmDialog
        open={!!transferTo}
        onClose={() => { if (!transfer.isPending) setTransferTo(null) }}
        onConfirm={confirmTransfer}
        title={`Chuyển quyền chủ nhóm cho ${transferTo?.name ?? ''}?`}
        description={
          <p>
            {transferTo?.name} sẽ là chủ nhóm “{groupName}”. Bạn vẫn là quản trị nhóm, nhưng không còn chuyển quyền
            hay giải tán nhóm được nữa.
          </p>
        }
        confirmLabel="Chuyển quyền"
        loading={transfer.isPending}
      />

      <ConfirmDialog
        open={disbanding}
        onClose={() => { if (!disband.isPending) setDisbanding(false) }}
        onConfirm={confirmDisband}
        title={`Giải tán “${groupName}”?`}
        description={
          <p>Nhóm sẽ bị xoá cho mọi thành viên: không ai đọc hay gửi tin trong nhóm được nữa. Không hoàn tác được.</p>
        }
        confirmLabel="Giải tán nhóm"
        confirmVariant="danger"
        loading={disband.isPending}
      />
    </SidePanel>
  )
}

function PolicyChoice({ label, hint, checked, onChoose }: {
  label: string
  hint?: string
  checked: boolean
  onChoose: () => void
}) {
  return (
    <Pressable
      role="radio"
      aria-checked={checked}
      onClick={onChoose}
      className="flex items-center gap-3 w-full min-h-13 px-3.5 py-2 text-left [&+&]:border-t [&+&]:border-line"
    >
      <span className="flex-1 min-w-0 grid">
        <span className="font-medium text-md text-ink">{label}</span>
        {hint && <span className="text-sm text-ink-muted">{hint}</span>}
      </span>
      <span
        aria-hidden="true"
        className={`grid place-items-center w-5.5 h-5.5 rounded-full border-2 shrink-0 ${checked ? 'border-accent' : 'border-line'}`}
      >
        {checked && <span className="w-2.5 h-2.5 rounded-full bg-accent" />}
      </span>
    </Pressable>
  )
}

function PersonRow({ person, role, action }: { person: Person; role: string; action?: ReactNode }) {
  return (
    <li className="flex items-center gap-3 min-h-13 px-3.5 py-2 [&+&]:border-t [&+&]:border-line">
      <Avatar name={person.name} hueKey={person.userId} src={person.avatarUrl || undefined} size={32} />
      <span className="flex-1 min-w-0 grid">
        <span className="font-medium text-ink truncate">{person.name}</span>
        <span className="text-sm text-ink-muted">{role}</span>
      </span>
      {action}
    </li>
  )
}
