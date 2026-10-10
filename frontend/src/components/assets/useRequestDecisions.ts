import { useState } from 'react'
import type { AssetRequest } from '../../api/assets'
import { useApproveRequest, useAssignRequest, useRejectRequest } from '../../hooks/useAssets'
import { explainAsset } from '../../lib/asset-model'
import { statusOf } from '../../lib/errors'
import { toast } from '../primitives'
import type { ApproveMode } from './ApproveDialog'

/** Why a decision failed. 403 says which rights it takes; the rest follow the server's reason. */
function decisionFailure(err: unknown, verb: string): string {
  if (statusOf(err) === 403) return `Bạn chưa có quyền ${verb} yêu cầu này. Cần quyền Duyệt và Quản lý trên loại tài sản; hỏi quản trị viên.`
  return explainAsset(err, verb)
}

/** Approve, assign and reject a request: which dialog is open, and what each decision does. */
export function useRequestDecisions() {
  const [deciding, setDeciding] = useState<{ request: AssetRequest; mode: ApproveMode } | null>(null)
  const [rejecting, setRejecting] = useState<AssetRequest | null>(null)
  const [decisionError, setDecisionError] = useState<string | undefined>()
  const approve = useApproveRequest()
  const assign = useAssignRequest()
  const reject = useRejectRequest()

  const closeDecision = () => {
    setDeciding(null)
    setDecisionError(undefined)
  }
  const closeReject = () => {
    setRejecting(null)
    setDecisionError(undefined)
  }
  const doApprove = async (request: AssetRequest, assetId: string) => {
    setDecisionError(undefined)
    try {
      if (deciding?.mode === 'assign') {
        await assign.mutateAsync({ id: request.id, assetId })
        toast('Đã giao tài sản')
      } else {
        await approve.mutateAsync({ id: request.id, ...(assetId ? { asset_id: assetId } : {}) })
        toast(assetId ? 'Đã duyệt và giao tài sản' : 'Đã duyệt yêu cầu')
      }
      closeDecision()
    } catch (err) {
      setDecisionError(decisionFailure(err, deciding?.mode === 'assign' ? 'giao tài sản cho' : 'duyệt'))
    }
  }
  // A request the caller may decide but not give an asset to is approved at once.
  const startApprove = async (request: AssetRequest) => {
    if (request.can_assign) {
      setDecisionError(undefined)
      setDeciding({ request, mode: 'approve' })
      return
    }
    try {
      await approve.mutateAsync({ id: request.id })
      toast('Đã duyệt yêu cầu')
    } catch (err) {
      toast.error(decisionFailure(err, 'duyệt'))
    }
  }
  const startReject = (request: AssetRequest) => {
    setDecisionError(undefined)
    setRejecting(request)
  }
  const startAssign = (request: AssetRequest) => {
    setDecisionError(undefined)
    setDeciding({ request, mode: 'assign' })
  }
  const doReject = async (request: AssetRequest, reason: string) => {
    setDecisionError(undefined)
    try {
      await reject.mutateAsync({ id: request.id, reason })
      setRejecting(null)
      toast('Đã từ chối yêu cầu')
    } catch (err) {
      setDecisionError(decisionFailure(err, 'từ chối'))
    }
  }

  return {
    deciding, rejecting, decisionError,
    decidePending: approve.isPending || assign.isPending,
    rejectPending: reject.isPending,
    anyPending: approve.isPending || assign.isPending || reject.isPending,
    closeDecision, closeReject, doApprove, startApprove, startReject, startAssign, doReject,
  }
}
