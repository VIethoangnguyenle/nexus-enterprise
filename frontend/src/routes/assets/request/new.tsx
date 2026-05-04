import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useAssetTypes, useCreateAssetRequest } from '../../../hooks/useAssets'
import { useWorkspaces } from '../../../hooks/useWorkspaces'
import { useState } from 'react'
import { Button, Select, Textarea, Heading, Spinner } from '../../../components/primitives'
import { Card } from '../../../components/composites'

export const Route = createFileRoute('/assets/request/new')({ component: AssetRequestNew })

const URGENCY_LEVELS = ['low', 'normal', 'high', 'critical'] as const
const urgencyColors: Record<string, string> = {
  low: 'border-outline text-on-surface-variant',
  normal: 'border-primary text-primary',
  high: 'border-secondary text-secondary',
  critical: 'border-error text-error',
}

function AssetRequestNew() {
  const navigate = useNavigate()
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const { data: typesData } = useAssetTypes(wsId)
  const createReq = useCreateAssetRequest(wsId)
  const [typeId, setTypeId] = useState('')
  const [justification, setJustification] = useState('')
  const [urgency, setUrgency] = useState('normal')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    createReq.mutate({ type_id: typeId, justification, urgency }, { onSuccess: () => navigate({ to: '/assets/requests' }) })
  }

  return (
    <div className="animate-fade-in">
      <Button variant="ghost" size="sm" onClick={() => navigate({ to: '/assets/dashboard' })}>← Back</Button>
      <h1 className="font-h1 text-h1 text-on-surface mt-3 mb-4">Request Asset</h1>

      <Card className="max-w-[640px]">
        <Card.Body>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            {createReq.error && (
              <div className="bg-error-container text-on-error-container px-3 py-2 rounded text-sm">
                {createReq.error.message}
              </div>
            )}

            <Select
              label="Asset Type"
              value={typeId}
              onChange={e => setTypeId(e.target.value)}
            >
              <option value="">Select...</option>
              {(typesData?.types || []).map(t => (
                <option key={t.id} value={t.id}>{t.name} ({t.category})</option>
              ))}
            </Select>

            {/* Urgency selector */}
            <div>
              <label className="block text-[11px] font-bold text-on-surface-variant uppercase tracking-widest mb-2">
                Urgency
              </label>
              <div className="flex gap-2">
                {URGENCY_LEVELS.map(u => (
                  <button
                    key={u}
                    type="button"
                    onClick={() => setUrgency(u)}
                    className={`px-3 py-2 rounded text-sm border capitalize
                      transition-all cursor-pointer
                      ${urgency === u
                        ? `${urgencyColors[u]} bg-surface-container-high font-medium`
                        : 'border-outline-variant bg-transparent text-on-surface-variant hover:border-primary'
                      }`}
                  >
                    {u}
                  </button>
                ))}
              </div>
            </div>

            <Textarea
              label="Justification"
              value={justification}
              onChange={e => setJustification(e.target.value)}
              rows={4}
              placeholder="Explain why..."
            />

            <Button type="submit" disabled={createReq.isPending || !typeId || !justification.trim()}>
              {createReq.isPending ? <Spinner size="sm" /> : 'Submit Request'}
            </Button>
          </form>
        </Card.Body>
      </Card>
    </div>
  )
}
