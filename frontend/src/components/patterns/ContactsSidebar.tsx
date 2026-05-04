import { useState } from 'react'
import {
  ChevronRight, ChevronDown, Building2, Users, UserPlus,
  Star, FolderOpen, Globe,
} from 'lucide-react'

export interface ContactCategory {
  key: string
  label: string
  count: number
}

interface ContactsSidebarProps {
  departments: ContactCategory[]
  activeDept: string
  onSelectDept: (key: string) => void
  onAddContact?: () => void
}

/** Contacts context sidebar matching Stitch contacts-directory.html:
 *  w-64, bg-surface-container-lowest, border-r border-outline-variant.
 *  Active dept items use bg-primary-fixed text-on-primary-fixed-variant.
 *  Inactive: text-on-surface-variant hover:bg-surface-container-low.
 *  "Add Contact" CTA: bg-primary rounded-lg py-2 shadow-sm. */
export function ContactsSidebar({
  departments,
  activeDept,
  onSelectDept,
  onAddContact,
}: ContactsSidebarProps) {
  const [orgExpanded, setOrgExpanded] = useState(true)

  return (
    <div className="w-64 flex-shrink-0 bg-surface-container-lowest border-r border-outline-variant flex flex-col h-full">
      {/* Add Contact button — Stitch: p-4 pb-2, bg-primary rounded-lg py-2 shadow-sm */}
      <div className="p-4 pb-2">
        <button
          onClick={onAddContact}
          className="w-full flex items-center justify-center gap-2 py-2 bg-primary hover:bg-primary/90
            text-on-primary font-semibold text-sm rounded-lg transition-colors cursor-pointer border-none shadow-sm"
        >
          <UserPlus size={16} />
          Add Contact
        </button>
      </div>

      {/* Category tree — Stitch: p-2 space-y-1 */}
      <nav className="flex-1 overflow-y-auto p-2 space-y-1">
        {/* Organization Contacts section */}
        <div>
          {/* Stitch: bg-surface-container text-primary font-medium rounded-lg when expanded */}
          <button
            onClick={() => setOrgExpanded(!orgExpanded)}
            className={`w-full flex items-center gap-2 px-3 py-2 rounded-lg border-none cursor-pointer
              transition-colors text-sm font-medium
              ${orgExpanded
                ? 'bg-surface-container text-primary'
                : 'bg-transparent text-on-surface hover:bg-surface-container-low'
              }`}
          >
            <span className={`transition-transform ${orgExpanded ? 'rotate-90' : ''}`}>
              <ChevronRight size={20} />
            </span>
            <Building2 size={20} />
            <span>Organization Contacts</span>
          </button>

          {/* Sub-items — Stitch: pl-9 pr-2 py-1 space-y-1 */}
          {orgExpanded && (
            <div className="pl-9 pr-2 py-1 space-y-1">
              {/* All contacts */}
              <SidebarItem
                icon={<Users size={18} className="text-outline" />}
                label="All"
                count={departments.reduce((sum, d) => sum + d.count, 0)}
                active={activeDept === ''}
                onClick={() => onSelectDept('')}
              />
              {/* Department items */}
              {departments.map((dept) => (
                <SidebarItem
                  key={dept.key}
                  icon={<span className="w-2 h-2 rounded-full bg-primary" />}
                  label={dept.label}
                  count={dept.count}
                  active={activeDept === dept.key}
                  onClick={() => onSelectDept(dept.key)}
                />
              ))}
            </div>
          )}
        </div>

        {/* Fixed categories — Stitch: same rounded-lg hover:bg-surface-container-low pattern */}
        <SidebarItem
          icon={<Globe size={20} className="text-outline" />}
          label="External Contacts"
          active={activeDept === '__external'}
          onClick={() => onSelectDept('__external')}
          isTopLevel
        />
        <SidebarItem
          icon={<UserPlus size={20} className="text-outline" />}
          label="New Contacts"
          active={activeDept === '__new'}
          onClick={() => onSelectDept('__new')}
          isTopLevel
        />
        <SidebarItem
          icon={<Star size={20} className="text-outline" />}
          label="Starred Contacts"
          active={activeDept === '__starred'}
          onClick={() => onSelectDept('__starred')}
          isTopLevel
        />
        <SidebarItem
          icon={<FolderOpen size={20} className="text-outline" />}
          label="My Groups"
          active={activeDept === '__groups'}
          onClick={() => onSelectDept('__groups')}
          isTopLevel
        />
      </nav>
    </div>
  )
}

/** Individual sidebar item matching Stitch contacts-directory.html:
 *  Active: bg-primary-fixed text-on-primary-fixed-variant font-medium.
 *  Inactive: text-on-surface-variant hover:bg-surface-container-low.
 *  Stitch uses rounded-lg, px-3 py-1.5, count as text-xs text-outline. */
function SidebarItem({
  icon,
  label,
  count,
  active,
  onClick,
  isTopLevel,
}: {
  icon: React.ReactNode
  label: string
  count?: number
  active: boolean
  onClick: () => void
  isTopLevel?: boolean
}) {
  return (
    <button
      onClick={onClick}
      className={`w-full flex items-center justify-between ${isTopLevel ? 'gap-2 px-3 py-2' : 'px-3 py-1.5'} rounded-lg
        border-none cursor-pointer transition-colors text-sm group
        ${active
          ? 'bg-primary-fixed font-medium'
          : 'bg-transparent text-on-surface-variant hover:bg-surface-container-low'
        }`}
    >
      <div className="flex items-center gap-2">
        <span className="flex items-center justify-center w-5 flex-shrink-0">{icon}</span>
        <span className={`truncate text-left ${active ? 'text-on-surface font-medium' : 'group-hover:text-on-surface'}`}>
          {label}
        </span>
      </div>
      {count != null && count > 0 && (
        <span className={`text-xs flex-shrink-0 ${active ? 'text-primary font-medium' : 'text-outline'}`}>
          {count}
        </span>
      )}
    </button>
  )
}
