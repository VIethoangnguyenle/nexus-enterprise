# Spec: Nexus Contacts Module

## Overview
Full-stack contacts directory — a workspace-scoped view of all members with profile information, filtering, and quick-action buttons for messaging and email.

## Data Model

### User Profile Extension (Auth DB)
```sql
ALTER TABLE users ADD COLUMN title VARCHAR(100) DEFAULT '';
ALTER TABLE users ADD COLUMN department VARCHAR(100) DEFAULT '';
ALTER TABLE users ADD COLUMN location VARCHAR(100) DEFAULT '';
ALTER TABLE users ADD COLUMN avatar_url TEXT DEFAULT '';
```

### Contact Response Shape
```json
{
  "contacts": [
    {
      "user_id": "uuid",
      "ngac_node_id": "uuid",
      "username": "sarah.jenkins",
      "display_name": "Sarah Jenkins",
      "email": "sarah@acme.com",
      "title": "Lead Product Designer",
      "department": "Design",
      "location": "New York",
      "avatar_url": "https://...",
      "is_online": true
    }
  ],
  "total": 42,
  "page": 1,
  "limit": 20
}
```

## API

### GET /api/workspaces/:id/contacts
- Auth: JWT required (workspace member)
- Query params: `department`, `location`, `search`, `page`, `limit`
- Implementation:
  1. Call Workspace gRPC `ListMembers(workspace_id)` → `[ngac_node_id]`
  2. Batch query `SELECT id, username, display_name, email, title, department, location, avatar_url FROM users WHERE ngac_node_id = ANY($1)`
  3. Apply `department` and `location` filters via WHERE clauses
  4. Apply `search` filter on `display_name` ILIKE
  5. Paginate with LIMIT/OFFSET (acceptable for contacts — low cardinality)
  6. Online status: check Redis presence keys if available, else omit

### PATCH /api/me/profile
- Auth: JWT required
- Body: `{ "title": "", "department": "", "location": "", "display_name": "" }`
- Updates caller's user row in `users` table
- Does NOT allow changing email/username (those have separate flows)

## Frontend

### Route: `/_workspace/contacts.tsx`
- Page title: "Contacts Directory" (h1)
- Subtitle: "Manage and connect with your enterprise network." (body-lg, muted)
- `ContactsFilterBar` component below header
- Responsive card grid: `grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6`

### Component: `ContactCard.tsx`
- White card with rounded-xl, p-6, subtle shadow, hover lift (-translate-y-1)
- Avatar: 96px circle, 4px white border, shadow
  - If `avatar_url` → `<img>` 
  - Else → initials in `secondary-container` bg
  - Green dot (12px) at bottom-right for online status
- Name: h3 weight, centered
- Title: body-md, on-surface-variant color
- Badges: flex-wrap row of `label-caps` pills in `surface-variant` bg
- Actions row at bottom: "Message" primary button (flex-1) + "Email" icon button
- "Message" onClick: navigate to DM channel with that user
- "Email" onClick: `window.open('mailto:' + email)`

### Component: `ContactsFilterBar.tsx`
- White container, rounded-xl, p-4, shadow, border
- Left: department `<select>` + location `<select>` (surface-container-low bg, outline-variant border)
- Right: "More Filters" outline button with filter icon
- Responsive: stack vertically on mobile

### Hook: `useContacts.ts`
```ts
export function useContacts(workspaceId: string, filters: ContactFilters) {
  return useQuery({
    queryKey: ['contacts', workspaceId, filters],
    queryFn: () => contactsApi.list(workspaceId, filters),
  })
}
```

## Behavior
- Contacts page loads automatically when navigating to "Contacts" in sidebar
- Department/location filters are additive (AND logic)
- "All Departments" / "All Locations" = no filter applied
- Clicking "Message" checks for existing DM channel → if exists, navigate; if not, create one then navigate
- Online status is best-effort — based on WebSocket connection presence
