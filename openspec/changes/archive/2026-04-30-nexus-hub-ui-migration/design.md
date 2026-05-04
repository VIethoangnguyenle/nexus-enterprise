# Design — Nexus Hub UI Migration

## Design Source of Truth

All UI designs reference Google Stitch project `14852434379132121789` ("Nexus Hub"). Exported HTML/PNG stored at `/tmp/stitch-screens/`.

## Architecture Decisions

### AD-1: No New Microservice for Contacts

Contacts is an **aggregated view** of workspace members + user profiles. User identity data (name, title, department, location) belongs in the Auth domain. Creating a separate contacts microservice would violate DRY and require cross-service data sync.

**Decision**: Extend Auth service with profile fields + contacts listing endpoint.

### AD-2: Workspace Type Field

Differentiate personal vs organization workspaces via a `type` column on `workspaces` table rather than a separate table. Values: `personal` | `organization`.

- Personal workspace: auto-created on registration, 1 member, non-deletable
- Organization workspace: user-created via onboarding wizard, supports multi-member, roles, permissions

### AD-3: Frontend Layout Architecture

Replace compact LarkRail (90px) with a full Nexus Hub layout:

```
┌──────────────────────────────────────────────────────────┐
│  TopBar (64px) — brand, search, notifications, avatar   │
├─────────────┬────────────────────────────────────────────┤
│             │                                            │
│  Sidebar    │   Content Area                             │
│  (280px)    │   - Chat: ListPanel + MessagePanel         │
│             │   - Drive: TreePanel + FileGrid            │
│  Workspace  │   - Contacts: FilterBar + CardGrid        │
│  switcher   │   - Documents: sidebar + editor            │
│  Nav items  │                                            │
│  Settings   │                                            │
│  Support    │                                            │
│             │                                            │
├─────────────┼────────────────────────────────────────────┤
│  (mobile: drawer overlay)   (mobile: full width)         │
└──────────────────────────────────────────────────────────┘
```

### AD-4: Auth Flow State Machine

```
                    ┌────────────┐
                    │  /login    │
                    │ Email input│
                    └─────┬──────┘
                          │ Continue
                    ┌─────▼──────┐
               ┌────│  Decision  │────┐
               │    │ User exists?│   │
               │    └────────────┘    │
             Yes                      No
               │                      │
        ┌──────▼──────┐    ┌──────────▼──────┐
        │  /verify    │    │  /register      │
        │  6-digit OTP│    │  Set password   │
        └──────┬──────┘    │  + display name │
               │           └──────────┬──────┘
               │                      │
        ┌──────▼──────────────────────▼──────┐
        │           /welcome-back            │
        │    "Logging you in..." splash      │
        │    (fetch workspaces, 2s delay)    │
        └──────────────┬─────────────────────┘
                       │
              ┌────────▼────────┐
              │ /workspace-     │
              │  select         │
              │ Organizations   │
              │ vs Personal     │
              └──┬──────────┬───┘
                 │          │
          Select WS    Create New
                 │          │
                 │   ┌──────▼──────┐
                 │   │ /onboarding │
                 │   │ Step 1: Org │
                 │   │ Step 2: Inv │
                 │   └──────┬──────┘
                 │          │
              ┌──▼──────────▼──┐
              │   /channels    │
              │  (workspace)   │
              └────────────────┘
```

### AD-5: Contacts Data Model

Extend `users` table (Auth service DB):

```sql
ALTER TABLE users ADD COLUMN title VARCHAR(100) DEFAULT '';
ALTER TABLE users ADD COLUMN department VARCHAR(100) DEFAULT '';
ALTER TABLE users ADD COLUMN location VARCHAR(100) DEFAULT '';
ALTER TABLE users ADD COLUMN avatar_url TEXT DEFAULT '';
```

Contacts API endpoint (Auth REST handler):

```
GET /api/workspaces/:id/contacts?department=X&location=Y&page=1&limit=20
```

Returns: workspace members enriched with profile data. Internally:
1. Call Workspace gRPC `ListMembers` → get `[ngac_node_id]`
2. Batch query `users` table by ngac_node_id for profile data
3. Apply filter + pagination

### AD-6: Drive Folder Tree

Use existing Drive service `ListFolders` API which returns flat list of folders with `parent_id`. Frontend builds tree structure client-side using recursive grouping.

No backend change needed — the API already returns enough data for tree construction.

## Design Token Mapping

From Stitch DNA:

| Token | Value | Usage |
|-------|-------|-------|
| `--primary` | #004AC6 | Buttons, active states, links |
| `--primary-container` | #2563EB | Filled buttons, sidebar active bg |
| `--surface` | #F9F9FF | Page background |
| `--surface-container` | #E7EEFF | Cards, sidebar bg |
| `--surface-container-lowest` | #FFFFFF | Input fields, modal bg |
| `--on-surface` | #111C2D | Primary text |
| `--on-surface-variant` | #434655 | Secondary/muted text |
| `--outline` | #737686 | Input borders, dividers |
| `--outline-variant` | #C3C6D7 | Subtle borders |
| `--font-family` | Manrope | All text |
| `--spacing-unit` | 4px | Base spacing multiplier |

## Typography Scale

| Name | Size / Line-height | Weight | Usage |
|------|-------------------|--------|-------|
| h1 | 32px / 40px | 700 | Page titles |
| h2 | 24px / 32px | 600 | Section headers |
| h3 | 20px / 28px | 600 | Card titles, modal headers |
| body-lg | 16px / 24px | 400 | Prominent body text |
| body-md | 14px / 20px | 400 | Default body text |
| body-sm | 13px / 18px | 400 | Captions, metadata |
| button | 14px / 20px | 600 | Button labels |
| label-caps | 11px / 16px | 700, uppercase, 0.05em tracking | Badge labels, section headers |

## Component Specifications

### TopBar
- Height: 64px, white bg, bottom shadow
- Left: Brand "Nexus Enterprise" (h3 bold, primary color)
- Center: Search bar (max-w-md, surface-container-lowest bg, outline-variant border, rounded-lg)
- Right: Icon buttons (notifications, help, settings) + User avatar (36px round)

### AppSidebar
- Width: 280px fixed, surface-container-low bg
- Header: Workspace icon + name + tier badge
- "+ New Project" CTA button (primary, full width)
- Nav items: icon + label, hover → slate-100, active → white bg + primary text + shadow-sm
- Footer: Settings, Support links + border-t separator

### Contact Card
- surface-container-lowest bg, rounded-xl, p-6
- Avatar: 96px round, 4px border, green dot for online
- Name: h3, Title: body-md muted
- Badges: label-caps in surface-variant pills
- Actions: "Message" primary button + "Email" outline icon button
- Hover: translateY(-4px) with transition

### Message Bubbles
- Others: surface-container-low bg, left-aligned, avatar + name + timestamp
- Self: primary bg, on-primary text, right-aligned
- File attachments: surface-container card with icon + filename + size
- Max-width: 70% of container
