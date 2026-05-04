# Design — Fix Icons, Upload & Message Order

## 1. Icon System — Lucide React

### Hiện trạng
```
AppRail:    { icon: '💬', label: 'Messaging' }  → text-sm (14px)
Drive page: getIcon() → emoji '📁', '📄', '🖼️'
Chat:       '📎', '💬', '📌', '🔍' scattered across 15+ files
```

### Giải pháp
```
npm install lucide-react
```

Lucide React cung cấp 1400+ SVG icons, mỗi icon là React component có props:
- `size` (px) — kiểm soát kích thước chính xác
- `color` — CSS color string
- `strokeWidth` — độ dày nét (default 2)

### Mapping cụ thể

```
Emoji → Lucide Component         Size   Color
──────────────────────────────────────────────
💬    → MessageSquare             20px   var(--accent)
📄    → FileText                  20px   var(--text-muted)
💾    → HardDrive                 20px   var(--text-muted)
📦    → Package                   20px   var(--text-muted)
⚙️    → Settings                  20px   var(--text-muted)
🚪    → LogOut                    18px   inherit
📎    → Paperclip                 16px   inherit
🔍    → Search                    16px   inherit
📌    → Pin                       16px   inherit
📁    → Folder                    16px   var(--accent)
🖼️    → Image                     16px   inherit
🗑️    → Trash2                    16px   var(--danger)
📥    → Download                  16px   inherit
📤    → Upload                    16px   inherit
🔔    → Bell                      18px   inherit
```

### AppRail redesign
```tsx
// Before
const railItems = [
  { id: 'messaging', icon: '💬', label: 'Messaging' },
]

// After
import { MessageSquare, FileText, HardDrive, Package, Settings } from 'lucide-react'

const railItems = [
  { id: 'messaging', icon: MessageSquare, label: 'Messaging', color: '#6366f1' },
  { id: 'documents', icon: FileText, label: 'Documents', color: '#3b82f6' },
  { id: 'drive', icon: HardDrive, label: 'Drive', color: '#10b981' },
  { id: 'assets', icon: Package, label: 'Assets', color: '#f59e0b' },
  { id: 'settings', icon: Settings, label: 'Settings', color: undefined },
]

// Render
<item.icon size={20} color={activeModule === item.id ? item.color : undefined} />
```

### Files bị ảnh hưởng
```
components/patterns/AppRail.tsx          — Rail icons
components/patterns/ListPanel.tsx        — Panel icons
components/patterns/FilePreviewCard.tsx  — File type icons
components/chat/ChatEditor.tsx           — Toolbar icons
components/chat/HoverActionBar.tsx       — Action icons
components/chat/ChannelInfoPanel.tsx     — Tab icons
components/NotificationBell.tsx          — Bell icon
routes/_workspace/drive.tsx              — File/folder icons
routes/_workspace/documents.tsx          — Empty state icon
routes/_assets.tsx                       — Settings icon
```

---

## 2. Vite Proxy Routing Fix

### Hiện trạng
```
Request:  POST /api/workspaces/:id/drive/files
Vite proxy match: '/api/workspaces' prefix → workspace :8181
Result: 404 (workspace service has no /drive endpoint)
```

### Giải pháp — Extend `configure` callback
```
/api/workspaces/:id/drive/*     → drive    :8185
/api/workspaces/:id/documents/* → document :8182
/api/workspaces/:id/channels/*  → messaging :8183
/api/workspaces/:id/*           → workspace :8181 (default)
```

```javascript
// vite.config.js — workspace proxy configure
configure: (proxy) => {
  const originalWeb = proxy.web.bind(proxy)
  proxy.web = (req, res, opts = {}) => {
    const url = req.url || ''
    // /api/workspaces/:id/drive/* → drive service
    if (/^\/api\/workspaces\/[^/]+\/drive/.test(url)) {
      return originalWeb(req, res, { ...opts, target: 'http://localhost:8185' })
    }
    // /api/workspaces/:id/documents/* → document service
    if (/^\/api\/workspaces\/[^/]+\/documents/.test(url)) {
      return originalWeb(req, res, { ...opts, target: 'http://localhost:8182' })
    }
    // /api/workspaces/:id/channels/* → messaging service
    if (/^\/api\/workspaces\/[^/]+\/channels/.test(url)) {
      return originalWeb(req, res, { ...opts, target: 'http://localhost:8183' })
    }
    return originalWeb(req, res, opts)
  }
}
```

---

## 3. Drive API Field Name Fix

### Hiện trạng
```
Frontend (drive.ts:100):
  body: JSON.stringify({ filename, mime_type, size_bytes, parent_id })
                          ^^^^^^^^
Backend (handler.go:154):
  Name string `json:"name"`
                     ^^^^
→ Go binds body.Name = "" → file created with empty name
```

### Giải pháp — Fix frontend to match backend
```typescript
// drive.ts — createFile
body: JSON.stringify({
  name: filename,        // was: filename
  mime_type: mimeType,
  size_bytes: sizeBytes,
  parent_id: parentId || '',
})
```

---

## 4. Drive Handler RequireClaims

### Hiện trạng
Drive handlers use `httputil.GetClaims(c)` which returns nil for unauthenticated requests, causing nil-pointer panic — same bug we fixed in messaging.

### Giải pháp
Apply same `RequireClaims` pattern to all drive handlers:
```go
claims, err := httputil.RequireClaims(c)
if err != nil {
    return err
}
```

---

## 5. Message Order (Already Fixed)

Frontend applies `[...msgs].reverse()` at line 35 of `channels.$channelId.tsx`.
Backend `ORDER BY DESC` + cursor-based `before` pagination remains unchanged.
