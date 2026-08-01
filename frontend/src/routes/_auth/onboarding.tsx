import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useCreateWorkspace } from '../../hooks/useWorkspaces'
import { Spinner, Button, Input } from '../../components/primitives'
import { ArrowRight, Upload, Plus, X } from 'lucide-react'

export const Route = createFileRoute('/_auth/onboarding')({
  component: OnboardingPage,
})

/** 2-step onboarding wizard matching Stitch source:
 *  Step 1: .stitch/designs/onboarding-1.html — Create Organization
 *  Step 2: .stitch/designs/onboarding-2.html — Invite Team
 *  Key tokens: max-w-2xl card, rounded-xl, p-space-xl (40px), h-2 progress bars,
 *  label-caps uppercase labels, surface-bright bg for upload area. */
function OnboardingPage() {
  const navigate = useNavigate()
  const createWorkspace = useCreateWorkspace()
  const [step, setStep] = useState(1)
  const [orgName, setOrgName] = useState('')
  const [orgSlug, setOrgSlug] = useState('')
  const [inviteEmails, setInviteEmails] = useState([''])

  const handleOrgNameChange = (val: string) => {
    setOrgName(val)
    setOrgSlug(val.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, ''))
  }

  const handleCreateOrg = () => {
    createWorkspace.mutate(orgName, {
      onSuccess: () => {
        navigate({ to: '/workspace-select' as any })
      },
    })
  }

  const addEmailRow = () => setInviteEmails([...inviteEmails, ''])
  const removeEmailRow = (i: number) => setInviteEmails(inviteEmails.filter((_, idx) => idx !== i))
  const updateEmail = (i: number, val: string) => {
    const updated = [...inviteEmails]
    updated[i] = val
    setInviteEmails(updated)
  }

  return (
    <div className="flex items-center justify-center min-h-screen bg-background p-8">
      {/* Card — Stitch: max-w-2xl rounded-xl shadow-[0_8px_30px] p-space-xl */}
      <div className="w-full max-w-2xl bg-surface-container-lowest rounded-xl
        shadow-card p-10 relative overflow-hidden">

        {/* Progress Indicator — Stitch: two h-2 bars */}
        <div className="mb-10">
          <div className="flex items-center justify-between mb-2">
            <span className="text-label-caps text-secondary tracking-widest">
              STEP {step} OF 2
            </span>
            <span className="text-label-caps text-primary tracking-widest">
              {step === 1 ? 'NEXT: INVITE TEAM' : 'FINISH'}
            </span>
          </div>
          <div className="flex gap-2">
            <div className={`h-2 w-1/2 rounded-full ${step >= 1 ? 'bg-primary' : 'bg-surface-variant'}`} />
            <div className={`h-2 w-1/2 rounded-full ${step >= 2 ? 'bg-primary' : 'bg-surface-variant'}`} />
          </div>
        </div>

        {step === 1 ? (
          <div className="animate-fade-in">
            {/* Header — Stitch: font-h1 text-h1, body-lg secondary */}
            <div className="mb-10">
              <h1 className="text-h1 text-on-surface mb-1">Create your Organization</h1>
              <p className="text-body text-secondary">
                Set up your workspace. You can always change these details later.
              </p>
            </div>

            <form className="space-y-6" onSubmit={(e) => { e.preventDefault(); setStep(2) }}>
              {/* Logo Upload — Stitch: surface-bright bg, rounded-lg, dashed border circle */}
              <div className="flex flex-col sm:flex-row items-start sm:items-center gap-6 p-6
                bg-surface-bright rounded-lg border border-outline-variant/30">
                <div className="relative group cursor-pointer">
                  <div className="w-24 h-24 rounded-full bg-surface-container flex items-center justify-center
                    border-2 border-dashed border-outline-variant group-hover:border-primary transition-colors">
                    <Upload size={24} className="text-secondary group-hover:text-primary" />
                  </div>
                </div>
                <div className="flex-1">
                  <h3 className="text-h3 text-on-surface mb-1">Workspace Logo</h3>
                  <p className="text-small text-secondary mb-2">
                    Upload a square image. High-resolution PNG or JPG recommended (max 5MB).
                  </p>
                  <Button type="button" variant="secondary" size="md">
                    <Upload size={14} />
                    Upload Image
                  </Button>
                </div>
              </div>

              {/* Company Name — Stitch: label-caps uppercase, icon prefix */}
              <div>
                <label className="block text-label-caps text-on-surface-variant mb-1" htmlFor="company-name">
                  COMPANY NAME *
                </label>
                <Input
                  id="company-name"
                  type="text"
                  value={orgName}
                  onChange={(e) => handleOrgNameChange(e.target.value)}
                  placeholder="e.g. Acme Corporation"
                  required
                  autoFocus
                />
              </div>

              {/* Workspace URL — Stitch: split input with suffix block */}
              <div>
                <label className="block text-label-caps text-on-surface-variant mb-1" htmlFor="workspace-url">
                  WORKSPACE URL
                </label>
                <div className="flex items-center">
                  {/* eslint-disable-next-line no-restricted-syntax -- flex-1 con của hàng flex ghép
                      với khối hậu tố .enterpriseflow.com: bọc div của Input sẽ nhận chỗ flex thay
                      cho input (cùng lỗi đã gặp ở ô OTP), và bo góc chỉ-bên-trái (rounded-l, để
                      liền khối với hậu tố) không có trong variant nào của Input. Task 15. */}
                  <input
                    id="workspace-url"
                    type="text"
                    value={orgSlug}
                    onChange={(e) => setOrgSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
                    placeholder="acme"
                    className="flex-1 px-4 py-4 bg-surface-container-lowest border border-outline-variant border-r-0
                      rounded-l text-body text-on-surface placeholder:text-secondary
                      focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all z-10 relative"
                  />
                  <div className="bg-surface-container px-4 py-4 border border-outline-variant border-l-0
                    rounded-r text-small text-secondary whitespace-nowrap">
                    .enterpriseflow.com
                  </div>
                </div>
                <p className="text-small text-secondary mt-1">This is where your team will log in.</p>
              </div>

              {/* Action — Stitch: border-t, flex justify-end */}
              <div className="pt-4 mt-10 border-t border-outline-variant/30 flex justify-end">
                <Button type="submit" variant="primary" size="md" disabled={!orgName.trim()} className="group">
                  Next Step
                  <ArrowRight size={16} className="group-hover:translate-x-1 transition-transform" />
                </Button>
              </div>
            </form>
          </div>
        ) : (
          <div className="animate-fade-in">
            {/* Header — Step 2 */}
            <div className="mb-10">
              <h1 className="text-h1 text-on-surface mb-2">Invite your team</h1>
              <p className="text-body text-on-surface-variant">
                Collaboration is key. Add team members by entering their email addresses below.
              </p>
            </div>

            {/* Email rows */}
            <div className="flex flex-col gap-3 mb-4">
              {inviteEmails.map((email, i) => (
                <div key={i} className="flex gap-2">
                  {/* eslint-disable-next-line no-restricted-syntax -- flex-1 con của hàng flex cùng
                      nút xoá: bọc div của Input sẽ nhận chỗ flex thay cho input, làm vỡ hàng (cùng
                      lỗi ô OTP); rounded-lg (12px) cũng sẽ âm thầm rơi về rounded-md (8px) mặc định
                      của Input, đúng lỗi đã ghi nhận ở Button. Task 15. */}
                  <input
                    type="email"
                    value={email}
                    onChange={(e) => updateEmail(i, e.target.value)}
                    placeholder="colleague@company.com"
                    className="flex-1 px-4 py-3 bg-surface-bright border border-outline-variant rounded-lg
                      text-body text-on-surface placeholder:text-outline
                      focus:ring-2 focus:ring-primary/20 focus:border-primary outline-none transition-all"
                  />
                  {inviteEmails.length > 1 && (
                    /* eslint-disable-next-line no-restricted-syntax -- Không tone nào của IconButton
                       khớp: cần chữ trung tính lúc nghỉ rồi chuyển CẢ chữ lẫn viền sang error khi
                       hover; tone="danger" đã tô error ngay lúc nghỉ (sai), và không tone nào đổi
                       màu viền lúc hover. Thang size cũng dừng ở 36px (lg), thẻ này 40px. Ép qua sẽ
                       âm thầm mất một trong hai thuộc tính — cùng cơ chế specificity đã ghi nhận ở
                       Button. Task 15. */
                    <button
                      onClick={() => removeEmailRow(i)}
                      className="w-10 h-10 flex items-center justify-center bg-transparent border border-outline-variant
                        rounded-lg text-on-surface-variant hover:text-error hover:border-error
                        transition-colors cursor-pointer my-auto"
                    >
                      <X size={16} />
                    </button>
                  )}
                </div>
              ))}
            </div>

            <Button variant="ghost" size="sm" onClick={addEmailRow} className="mb-8">
              <Plus size={14} />
              Add another
            </Button>

            {/* Divider — Stitch: h-px bg-surface-container */}
            <div className="h-px bg-surface-container w-full my-6" />

            {/* Actions — Stitch: flex-col-reverse sm:flex-row */}
            <div className="flex flex-col-reverse sm:flex-row justify-between items-center gap-4 pt-2">
              <Button
                variant="ghost"
                size="cta"
                onClick={handleCreateOrg}
                disabled={createWorkspace.isPending}
                className="sm:w-auto px-4"
              >
                Skip for now
              </Button>
              <Button
                variant="primary"
                size="cta"
                onClick={handleCreateOrg}
                disabled={createWorkspace.isPending}
                className="sm:w-auto px-6 shadow-sm"
              >
                {createWorkspace.isPending ? <Spinner size="sm" /> : (
                  <>Send Invites & Continue <ArrowRight size={16} /></>
                )}
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
