import { createFileRoute } from '@tanstack/react-router'
import { useState, useMemo, useCallback } from 'react'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { useContacts, type Contact } from '../../hooks/useContacts'
import { ContactsSidebar, type ContactCategory } from '../../components/patterns/ContactsSidebar'
import { ContactsTable } from '../../components/patterns/ContactsTable'
import { ContactCard } from '../../components/patterns/ContactCard'
import { ContactProfilePanel } from '../../components/patterns/ContactProfilePanel'
import { LoadingState } from '../../components/LoadingState'
import { EmptyState } from '../../components/EmptyState'
import { Users, Search, MoreHorizontal, UserPlus, LayoutGrid, List, X, Building2 } from 'lucide-react'
import { ResponsiveDetailPanel } from '../../components/composites/ResponsiveDetailPanel'

export const Route = createFileRoute('/_workspace/contacts')({
  component: ContactsPage,
})

/** Contacts Directory matching Stitch contacts-directory.html:
 *  3-column layout: ContactsSidebar (w-64) + Main content + ContactProfilePanel (w-96).
 *  Header: px-8 py-6 bg-surface-container-lowest shadow-sm border-b border-outline-variant.
 *  Title: text-h2 with rounded-full member count badge.
 *  Breadcrumb: text-sm text-on-surface-variant with icon.
 *  Action buttons: border border-outline-variant rounded-lg. */
function ContactsPage() {
  const { data: wsData } = useWorkspaces()
  const wsParam = new URLSearchParams(window.location.search).get('ws')
  const wsId = (wsParam && wsData?.workspaces?.find((w: any) => w.id === wsParam)?.id) || wsData?.workspaces?.[0]?.id || ''

  const [activeDept, setActiveDept] = useState('')
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedContact, setSelectedContact] = useState<Contact | null>(null)
  const [viewMode, setViewMode] = useState<'table' | 'grid'>('table')

  // Fetch contacts with department filter
  const filters = {
    department: activeDept.startsWith('__') ? '' : activeDept,
    location: '',
    search: '',
  }
  const { data, isLoading } = useContacts(wsId, filters)
  const contacts = data?.contacts || []

  // Client-side search
  const filteredContacts = useMemo(() => {
    if (!searchQuery) return contacts
    const q = searchQuery.toLowerCase()
    return contacts.filter((c) =>
      c.display_name.toLowerCase().includes(q)
      || c.username.toLowerCase().includes(q)
      || c.email.toLowerCase().includes(q)
      || c.title?.toLowerCase().includes(q),
    )
  }, [contacts, searchQuery])

  // Build department categories from full contact list
  const departments = useMemo<ContactCategory[]>(() => {
    const deptMap = new Map<string, number>()
    for (const c of contacts) {
      if (c.department) {
        deptMap.set(c.department, (deptMap.get(c.department) || 0) + 1)
      }
    }
    return Array.from(deptMap.entries())
      .map(([key, count]) => ({ key, label: key, count }))
      .sort((a, b) => a.label.localeCompare(b.label))
  }, [contacts])

  const handleMessage = useCallback((_userId: string) => {
    // Future: navigate to DM channel
  }, [])

  const handleSelectContact = useCallback((contact: Contact) => {
    setSelectedContact((prev) => (prev?.user_id === contact.user_id ? null : contact))
  }, [])

  // Title for current view
  const currentTitle = activeDept && !activeDept.startsWith('__')
    ? activeDept
    : activeDept === '__external' ? 'External Contacts'
    : activeDept === '__new' ? 'New Contacts'
    : activeDept === '__starred' ? 'Starred Contacts'
    : activeDept === '__groups' ? 'My Groups'
    : 'Contacts Directory'

  return (
    <div className="flex-1 flex min-h-0">
      {/* Left sidebar — desktop only */}
      <div className="hidden lg:flex">
        <ContactsSidebar
          departments={departments}
          activeDept={activeDept}
          onSelectDept={setActiveDept}
          onAddContact={() => {}}
        />
      </div>

      {/* Main content — Stitch: flex-1 bg-surface flex flex-col overflow-hidden */}
      <div className="flex-1 flex flex-col min-h-0 min-w-0 bg-surface overflow-hidden">
        {/* Header — Stitch: px-8 py-6 border-b border-outline-variant bg-surface-container-lowest shadow-sm */}
        <div className="px-5 md:px-8 py-4 md:py-6 border-b border-outline-variant bg-surface-container-lowest
          flex items-center justify-between shrink-0 shadow-sm z-10">
          <div>
            <h1 className="text-h2 text-on-surface flex items-center gap-3">
              {currentTitle}
              {/* Member count badge — Stitch: text-sm text-outline px-2.5 py-0.5 bg-surface-container-low rounded-full border */}
              <span className="text-sm font-normal text-outline px-2.5 py-0.5 bg-surface-container-low
                border border-outline-variant/30 rounded-full">
                {filteredContacts.length} Members
              </span>
            </h1>
            {/* Breadcrumb — Stitch: text-sm text-on-surface-variant mt-1.5 flex items-center gap-1 */}
            <p className="text-sm text-on-surface-variant mt-1.5 flex items-center gap-1">
              <Building2 size={16} />
              Organization Contacts{activeDept && !activeDept.startsWith('__') ? ` / ${activeDept}` : ''}
            </p>
          </div>

          {/* Actions — Stitch: flex items-center gap-3 */}
          <div className="flex items-center gap-3">
            {/* Search input — shown inline on md+ */}
            <div className="hidden md:block relative">
              <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-outline-variant pointer-events-none" />
              <input
                type="text"
                placeholder="Search contacts..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-64 pl-9 pr-3 py-1.5 bg-surface-container-low border border-outline-variant
                  rounded-full text-sm outline-none focus:border-primary focus:ring-2 focus:ring-primary/10 transition-all"
              />
            </div>

            {/* View toggle — desktop only */}
            <div className="hidden lg:flex items-center border border-outline-variant rounded-lg overflow-hidden">
              <button
                onClick={() => setViewMode('table')}
                className={`p-1.5 border-none cursor-pointer transition-colors
                  ${viewMode === 'table' ? 'bg-primary text-on-primary' : 'bg-transparent text-on-surface-variant hover:bg-surface-container'}`}
                title="Table view"
              >
                <List size={16} />
              </button>
              <button
                onClick={() => setViewMode('grid')}
                className={`p-1.5 border-none cursor-pointer transition-colors
                  ${viewMode === 'grid' ? 'bg-primary text-on-primary' : 'bg-transparent text-on-surface-variant hover:bg-surface-container'}`}
                title="Grid view"
              >
                <LayoutGrid size={16} />
              </button>
            </div>

            {/* Invite button — Stitch: border border-outline-variant text-on-surface rounded-lg */}
            <button className="hidden md:flex items-center gap-2 px-4 py-2 bg-surface-container-lowest
              hover:bg-surface-container-low border border-outline-variant text-on-surface font-semibold
              text-sm rounded-lg transition-colors cursor-pointer">
              <UserPlus size={18} />
              Invite
            </button>

            {/* More — Stitch: p-2 rounded-lg border border-outline-variant */}
            <button className="p-2 text-on-surface-variant hover:bg-surface-container-low rounded-lg
              transition-colors border border-outline-variant cursor-pointer bg-transparent">
              <MoreHorizontal size={20} />
            </button>
          </div>
        </div>

        {/* Mobile search bar */}
        <div className="md:hidden px-4 py-3 bg-surface-container-lowest border-b border-outline-variant/30">
          <div className="relative">
            <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-outline-variant pointer-events-none" />
            <input
              type="text"
              placeholder="Search contacts..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-9 pr-3 py-2 bg-surface-container-low border border-outline-variant
                rounded-full text-sm outline-none focus:border-primary transition-all"
            />
          </div>
        </div>

        {/* Content — Stitch: flex-1 overflow-y-auto bg-background */}
        {isLoading ? (
          <LoadingState />
        ) : filteredContacts.length === 0 ? (
          <div className="flex-1 flex items-center justify-center bg-background">
            <EmptyState
              icon={<Users size={40} className="text-on-surface-variant" strokeWidth={1.5} />}
              title="No contacts found"
              description={searchQuery || activeDept
                ? 'Try adjusting your search or department filter.'
                : 'Invite team members to see them here.'}
            />
          </div>
        ) : viewMode === 'table' ? (
          <ContactsTable
            contacts={filteredContacts}
            selectedId={selectedContact?.user_id}
            onSelect={handleSelectContact}
          />
        ) : (
          <div className="flex-1 overflow-y-auto p-5 md:p-8 bg-background">
            <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
              {filteredContacts.map((c) => (
                <ContactCard
                  key={c.user_id}
                  contact={c}
                  onSelect={handleSelectContact}
                  onMessage={handleMessage}
                />
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Profile panel — right side on desktop, overlay on mobile */}
      {selectedContact && (
        <ResponsiveDetailPanel>
          <ContactProfilePanel
            contact={selectedContact}
            onClose={() => setSelectedContact(null)}
            onMessage={handleMessage}
          />
        </ResponsiveDetailPanel>
      )}
    </div>
  )
}
