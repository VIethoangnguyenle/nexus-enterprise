import { createFileRoute } from '@tanstack/react-router'
import { useAssetTypes, useCreateAssetType } from '../../hooks/useAssets'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { useState } from 'react'
import { Button, Heading, Input, Select, Badge, Spinner } from '../../components/primitives'
import { Card, Modal } from '../../components/composites'
import { EmptyState } from '../../components/EmptyState'
import { LoadingState } from '../../components/LoadingState'
import { Tag } from 'lucide-react'

export const Route = createFileRoute('/assets/types')({ component: AssetTypeConfig })

const CATEGORIES = ['hardware', 'software', 'license', 'furniture', 'other']

function AssetTypeConfig() {
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const { data, isLoading } = useAssetTypes(wsId)
  const createType = useCreateAssetType(wsId)
  const types = data?.types || []
  const [showForm, setShowForm] = useState(false)
  const [name, setName] = useState('')
  const [category, setCategory] = useState('')

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = name.trim()
    if (!trimmed || !category) return
    createType.mutate(
      { name: trimmed, category },
      {
        onSuccess: () => {
          setName('')
          setCategory('')
          setShowForm(false)
        },
      },
    )
  }

  return (
    <div className="animate-fade-in">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-4 md:mb-6">
        <h1 className="font-h1 text-h1 text-on-surface">Asset Types</h1>
        <Button onClick={() => setShowForm(true)}>+ New Type</Button>
      </div>

      {showForm && (
        <Modal onClose={() => setShowForm(false)} size="sm">
          <Modal.Title>Create Asset Type</Modal.Title>
          <form onSubmit={handleCreate}>
            <Modal.Body>
              {createType.error && (
                <div className="bg-error-container text-on-error-container px-3 py-2 rounded text-sm mb-3">
                  {createType.error.message}
                </div>
              )}
              <Input
                label="Type Name"
                autoFocus
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Laptop, Monitor, Software License"
              />
              <Select
                label="Category"
                value={category}
                onChange={(e) => setCategory(e.target.value)}
                className="mt-3"
              >
                <option value="">Select category...</option>
                {CATEGORIES.map((c) => (
                  <option key={c} value={c}>{c.charAt(0).toUpperCase() + c.slice(1)}</option>
                ))}
              </Select>
            </Modal.Body>
            <Modal.Actions>
              <Button type="button" variant="ghost" onClick={() => setShowForm(false)}>Cancel</Button>
              <Button type="submit" disabled={!name.trim() || !category || createType.isPending}>
                {createType.isPending ? <Spinner size="sm" /> : 'Create'}
              </Button>
            </Modal.Actions>
          </form>
        </Modal>
      )}

      {isLoading ? <LoadingState /> :
       types.length > 0 ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 md:gap-4">
          {types.map(t => (
            <Card key={t.id} className="hover:-translate-y-0.5 transition-transform duration-200">
              <Card.Body className="flex items-center gap-3">
                <div className="w-10 h-10 rounded bg-surface-variant
                  flex items-center justify-center">
                  <Tag size={20} className="text-on-surface-variant" />
                </div>
                <div>
                  <div className="text-sm font-medium text-on-surface">{t.name}</div>
                  <Badge variant="default">{t.category}</Badge>
                </div>
              </Card.Body>
            </Card>
          ))}
        </div>
      ) : <EmptyState icon={<Tag size={40} className="text-outline" strokeWidth={1.5} />} title="No asset types" description="Create your first asset type to get started."
             action={{ label: '+ Create Type', onClick: () => setShowForm(true) }} />}
    </div>
  )
}
