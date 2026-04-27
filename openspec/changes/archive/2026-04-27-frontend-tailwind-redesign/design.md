# Frontend Tailwind Redesign — Design

## Architecture Overview

### 3-Column Layout (Lark-inspired)

```
┌──────┬──────────────┬──────────────────────────────────────┐
│ Rail │  List Panel   │           Content Area               │
│ 48px │   ~280px      │           flex-1                     │
│      │  collapsible  │                                      │
│ ┌──┐ │ ┌──────────┐ │  ┌────────────────────────────────┐  │
│ │📄│ │ │ Filter   │ │  │  Topbar (breadcrumb + actions)  │  │
│ │💬│ │ │ Tabs     │ │  ├────────────────────────────────┤  │
│ │📦│ │ ├──────────┤ │  │                                │  │
│ │⚙ │ │ │ Item 1   │ │  │  Page Content                  │  │
│ │  │ │ │ Item 2   │ │  │                                │  │
│ │  │ │ │ Item 3   │ │  │                                │  │
│ │  │ │ │ ...      │ │  │                                │  │
│ ├──┤ │ │          │ │  │                                │  │
│ │👤│ │ │          │ │  └────────────────────────────────┘  │
│ └──┘ │ └──────────┘ │                                      │
└──────┴──────────────┴──────────────────────────────────────┘
```

**Collapse behavior:**
- Rail: always visible (48px icon-only)
- List Panel: toggle via rail click or keyboard shortcut, animates width 280px → 0
- Content: fills remaining space

### Component Layer Architecture

```
Layer 4: Layouts         AppLayout, AuthLayout, AssetLayout
            ↑ uses
Layer 3: Patterns        AppRail, ListPanel, ChatView, NotificationDropdown
            ↑ uses
Layer 2: Composites      Card, Modal, DataTable, Tabs, FilterBar
            ↑ uses
Layer 1: Primitives      Button, Input, Badge, Avatar, Spinner, Text
```

**Rules:**
- Higher layers can import from lower layers
- Same-layer imports are allowed
- Lower layers NEVER import from higher layers
- Primitives have ZERO business logic or API dependencies

### Directory Structure

```
src/
├── components/
│   ├── primitives/           ← Layer 1
│   │   ├── Button.tsx
│   │   ├── IconButton.tsx
│   │   ├── Input.tsx
│   │   ├── Textarea.tsx
│   │   ├── Select.tsx
│   │   ├── Badge.tsx
│   │   ├── Avatar.tsx
│   │   ├── Spinner.tsx
│   │   ├── Text.tsx
│   │   ├── Heading.tsx
│   │   └── index.ts          ← barrel export
│   ├── composites/           ← Layer 2
│   │   ├── Card.tsx           (Card, Card.Header, Card.Body, Card.Footer)
│   │   ├── Modal.tsx          (Modal, Modal.Content, Modal.Actions)
│   │   ├── DataTable.tsx
│   │   ├── Tabs.tsx
│   │   ├── FilterBar.tsx
│   │   ├── Timeline.tsx
│   │   └── index.ts
│   ├── patterns/             ← Layer 3
│   │   ├── AppRail.tsx
│   │   ├── ListPanel.tsx
│   │   ├── ChatView.tsx
│   │   ├── ChatInput.tsx
│   │   ├── ThreadPanel.tsx
│   │   ├── MessageItem.tsx
│   │   ├── NotificationDropdown.tsx
│   │   ├── DocumentCard.tsx
│   │   ├── AssetCard.tsx
│   │   └── index.ts
│   └── layouts/              ← Layer 4
│       ├── AppLayout.tsx
│       ├── AuthLayout.tsx
│       └── AssetLayout.tsx
├── assets/
│   └── illustrations/        ← AI-generated Constellation art
│       ├── empty-inbox.svg
│       ├── empty-documents.svg
│       ├── welcome.svg
│       └── error.svg
```

## Tailwind v4 Setup

### Installation (CSS-first, no config file)

```bash
npm install tailwindcss @tailwindcss/vite
```

### Vite Plugin

```js
// vite.config.js
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [
    TanStackRouterVite({ ... }),
    tailwindcss(),
    react(),
  ],
})
```

### CSS Entry Point (`index.css`)

```css
@import "tailwindcss";

/* NGAC Design Tokens — Constellation Theme */
@theme {
  --color-bg-primary: #0f1218;
  --color-bg-secondary: #161b22;
  --color-bg-tertiary: #1c2128;
  --color-bg-rail: #0d1117;
  --color-bg-hover: rgba(255, 255, 255, 0.04);
  --color-bg-active: rgba(99, 102, 241, 0.12);

  --color-border: rgba(255, 255, 255, 0.06);
  --color-border-focus: #6366f1;

  --color-text-primary: #e8eaed;
  --color-text-secondary: #8b949e;
  --color-text-muted: #636e7b;

  --color-accent: #6366f1;
  --color-accent-hover: #818cf8;
  --color-accent-glow: rgba(99, 102, 241, 0.25);

  --color-success: #10b981;
  --color-warning: #f59e0b;
  --color-danger: #ef4444;
  --color-info: #06b6d4;

  --radius-sm: 6px;
  --radius-md: 10px;
  --radius-lg: 14px;

  --font-sans: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;

  --shadow-lg: 0 16px 48px rgba(0, 0, 0, 0.4);
  --shadow-glow: 0 0 20px var(--color-accent-glow);
}

/* Scrollbar */
::-webkit-scrollbar { width: 5px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.08); border-radius: 3px; }

/* Animations */
@keyframes fade-in { from { opacity: 0; } to { opacity: 1; } }
@keyframes slide-up { from { opacity: 0; transform: translateY(12px); } to { opacity: 1; transform: translateY(0); } }
@keyframes slide-left { from { opacity: 0; transform: translateX(16px); } to { opacity: 1; transform: translateX(0); } }
@keyframes spin { to { transform: rotate(360deg); } }
```

## Color Palette — Constellation Theme

Inspired by Lark's dark theme with NGAC's indigo accent:

| Token | Value | Usage |
|-------|-------|-------|
| `bg-primary` | `#0f1218` | Content area background |
| `bg-secondary` | `#161b22` | List panel, modals |
| `bg-tertiary` | `#1c2128` | Cards, elevated surfaces |
| `bg-rail` | `#0d1117` | Icon rail (darkest) |
| `accent` | `#6366f1` | Primary actions, active states |
| `accent-hover` | `#818cf8` | Hover on accent elements |
| `text-primary` | `#e8eaed` | Headings, body text |
| `text-secondary` | `#8b949e` | Secondary info, labels |
| `text-muted` | `#636e7b` | Timestamps, disabled text |
| `border` | `rgba(255,255,255,0.06)` | Subtle dividers |

## Component Design Patterns

### Primitive Example — Button

```tsx
import { type ButtonHTMLAttributes, forwardRef } from 'react'

type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost'
type ButtonSize = 'sm' | 'md' | 'lg'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
}

const variantStyles: Record<ButtonVariant, string> = {
  primary: 'bg-accent text-white shadow-glow hover:bg-accent-hover',
  secondary: 'bg-bg-tertiary text-text-primary border border-border hover:bg-bg-hover',
  danger: 'bg-danger/10 text-danger border border-danger/20 hover:bg-danger/20',
  ghost: 'bg-transparent text-text-secondary hover:text-text-primary hover:bg-bg-hover',
}

const sizeStyles: Record<ButtonSize, string> = {
  sm: 'px-3 py-1.5 text-xs',
  md: 'px-4 py-2 text-sm',
  lg: 'px-5 py-2.5 text-base',
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', size = 'md', className = '', children, ...props }, ref) => (
    <button
      ref={ref}
      className={`inline-flex items-center justify-center gap-2 font-medium rounded-md
        transition-all duration-200 disabled:opacity-40 disabled:cursor-not-allowed
        ${variantStyles[variant]} ${sizeStyles[size]} ${className}`}
      {...props}
    >
      {children}
    </button>
  ),
)
```

### Compound Component Example — Card

```tsx
function CardRoot({ children, className = '' }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={`bg-bg-tertiary border border-border rounded-lg overflow-hidden ${className}`}>
      {children}
    </div>
  )
}

function CardHeader({ children, className = '' }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={`flex items-center justify-between px-4 py-3 border-b border-border ${className}`}>
      {children}
    </div>
  )
}

function CardBody({ children, className = '' }: { children: React.ReactNode; className?: string }) {
  return <div className={`p-4 ${className}`}>{children}</div>
}

export const Card = Object.assign(CardRoot, { Header: CardHeader, Body: CardBody })
```

### Layout Example — AppLayout (3-column)

```tsx
export function AppLayout() {
  const listPanelOpen = useUiStore(s => s.listPanelOpen)

  return (
    <div className="flex h-screen bg-bg-primary overflow-hidden">
      {/* Rail — always visible */}
      <AppRail />

      {/* List Panel — collapsible */}
      {listPanelOpen && (
        <div className="w-70 flex-shrink-0 bg-bg-secondary border-r border-border
                        animate-[slide-left_0.2s_ease]">
          <ListPanel />
        </div>
      )}

      {/* Content — flex grow */}
      <main className="flex-1 flex flex-col min-w-0">
        <Outlet />
      </main>
    </div>
  )
}
```

## Constellation Illustrations

AI-generated SVG/PNG illustrations with these characteristics:
- **Style**: Minimalist line art with soft indigo/purple glow
- **Motif**: Nodes connected by thin lines forming constellation patterns
- **Palette**: Monochrome with `#6366f1` accent highlights
- **Uses**:
  - Empty channel: "Start a conversation" — lone star waiting for connections
  - Empty documents: "No documents yet" — scattered nodes not yet linked
  - Welcome/onboarding: Full constellation forming — "Your workspace is ready"
  - Error state: Broken constellation — "Something went wrong"

## Migration Strategy

### Phase Order

**Phase 1 — Foundation** (Tailwind + Primitives + Auth)
- Install Tailwind v4, configure theme
- Build 10 primitives
- Redesign auth pages (login/register) — immediate visual payoff
- Generate Constellation illustrations

**Phase 2 — Layout + Navigation** (3-Column + Rail + List Panel)
- Build AppLayout, AppRail, ListPanel
- Migrate `_workspace.tsx` layout route
- Build Topbar component
- List panel shows channels/documents based on active rail item

**Phase 3 — Messaging** (Chat redesign)
- Build ChatView, MessageItem, ChatInput, ThreadPanel
- Migrate `channels.$channelId.tsx`
- Modal redesign (CreateChannelModal)

**Phase 4 — Documents + Assets** (CRUD pages)
- Build DocumentCard, DataTable, FilterBar
- Migrate documents page, asset pages (dashboard, list, detail, types, requests)
- Build AssetLayout with sub-navigation

**Phase 5 — Polish + Tests** (Production ready)
- Empty state illustrations placed
- NotificationDropdown redesign
- Settings page
- Update 3 test files
- Delete old CSS
- Browser verification

### Coexistence Strategy

During migration, **old CSS and Tailwind coexist**:
1. `@import "tailwindcss"` added to top of `index.css`
2. Old CSS classes remain — Tailwind's reset won't conflict (both are dark theme)
3. Each migrated component switches to Tailwind classes
4. After all components migrated, delete old CSS sections
5. Final `index.css` = only `@import "tailwindcss"` + `@theme` + animations

## Testing Impact

- Component tests (`CreateChannelModal.test.tsx`, `Sidebar.test.tsx`) need updated class assertions
- API tests (`messaging.test.ts`) — zero impact (no CSS)
- `test_app.sh` — zero impact (tests API responses, not UI classes)
- New browser tests recommended for visual regression
