# Component Reusability Refactor — Design

## Kiến trúc Component Layer

```
┌──────────────────────────────────────────────────────────────────┐
│                    Domain Components (drive/, chat/)             │
│  DeleteConfirmDialog  MoveItemDialog  CreateChannelModal  ...   │
│  ↓ COMPOSE FROM ↓                                                │
├──────────────────────────────────────────────────────────────────┤
│                    Composites (composites/)                       │
│  Modal + Modal.Header + Modal.Body + Modal.Actions              │
│  ConfirmDialog (icon + title + message + warning + actions)     │
│  AlertBanner (variant: error | warning | info | success)        │
│  Tabs  DataTable  PeekPanel  Breadcrumbs  Card  Timeline       │
│  ↓ COMPOSE FROM ↓                                                │
├──────────────────────────────────────────────────────────────────┤
│                    Primitives (primitives/)                       │
│  Button  IconButton  Input  Spinner  Badge  Avatar  Text        │
│  ↓ USE ↓                                                         │
├──────────────────────────────────────────────────────────────────┤
│                    Design Tokens (index.css @theme)               │
│  Colors  Typography  Spacing  Motion  Elevation                  │
└──────────────────────────────────────────────────────────────────┘

Rule: KHÔNG BAO GIỜ viết inline HTML/CSS cho pattern đã tồn tại ở layer dưới.
```

---

## 1. Upgrade Modal.tsx → M3 Tokens

### Hiện tại (legacy)
```tsx
// BUG: Legacy tokens
className="bg-gray-5 border-none md:border md:border-border md:rounded-xl
  p-4 md:p-5 w-full h-dvh md:h-auto shadow-overlay animate-slide-up"
// ModalTitle: text-section text-gray-13
```

### Sau refactor (M3)
```tsx
// M3 tokens
className="bg-surface-container-lowest rounded-xl shadow-lg animate-scale-in
  w-full mx-4 ${maxWidths[size]} max-h-[80vh] overflow-y-auto"
// ModalTitle: font-h3 text-on-surface
```

### Thêm Modal.Header sub-component
```tsx
function ModalHeader({ children, onClose, className = '' }: {
  children: ReactNode
  onClose?: () => void
  className?: string
}) {
  return (
    <div className={`flex items-center justify-between px-6 py-4
      border-b border-outline-variant ${className}`}>
      <div>{children}</div>
      {onClose && (
        <IconButton icon={X} size="sm" onClick={onClose} aria-label="Close" />
      )}
    </div>
  )
}
```

---

## 2. ConfirmDialog Composite (MỚI)

### Mục đích
Generic confirmation dialog — reuse cho delete file, leave workspace, revoke access, archive channel...

### API
```tsx
interface ConfirmDialogProps {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  title: string
  description: ReactNode
  /** Optional warning banner below description */
  warning?: string
  /** Icon shown above title */
  icon?: ReactNode
  /** Confirm button config */
  confirmLabel?: string        // default: "Confirm"
  confirmVariant?: 'primary' | 'error'  // default: "primary"
  confirmIcon?: ReactNode
  /** Loading state */
  loading?: boolean
}
```

### Cấu trúc JSX
```tsx
<Modal onClose={onClose} size="sm">
  <div className="p-6 flex flex-col items-center text-center">
    {/* Icon circle */}
    {icon && <div className="w-12 h-12 rounded-full ...">{icon}</div>}

    <Modal.Title>{title}</Modal.Title>
    <p>{description}</p>

    {/* Warning banner */}
    {warning && <AlertBanner variant="error">{warning}</AlertBanner>}

    <Modal.Actions>
      <Button variant="outline" onClick={onClose}>Cancel</Button>
      <Button variant={confirmVariant} loading={loading} onClick={onConfirm}>
        {confirmIcon} {confirmLabel}
      </Button>
    </Modal.Actions>
  </div>
</Modal>
```

### Sau khi có ConfirmDialog, DeleteConfirmDialog chỉ còn:
```tsx
export function DeleteConfirmDialog({ item, isDeleting, onConfirm, onClose }) {
  if (!item) return null
  return (
    <ConfirmDialog
      open={!!item}
      onClose={onClose}
      onConfirm={() => onConfirm(item)}
      title={`Delete ${item.item_type === 'folder' ? 'Folder' : 'File'}`}
      description={<>...delete <strong>{item.name}</strong>?</>}
      warning="This action cannot be undone. The file will be removed from all shared folders."
      icon={<Trash2 size={22} className="text-error" />}
      confirmLabel="Delete"
      confirmVariant="error"
      confirmIcon={<Trash2 size={16} />}
      loading={isDeleting}
    />
  )
}
// ~20 dòng thay vì 136 dòng
```

---

## 3. AlertBanner Composite (MỚI)

### Mục đích
Reusable inline alert banner — cho warning trong dialogs, error messages, info notices.

### API
```tsx
interface AlertBannerProps {
  variant: 'error' | 'warning' | 'info' | 'success'
  icon?: ReactNode               // auto-picks default per variant nếu không truyền
  children: ReactNode
  className?: string
}
```

### Token mapping
```
variant     bg                        text                       default icon
─────────── ───────────────────────── ────────────────────────── ──────────────
error       bg-error-container        text-on-error-container    AlertTriangle
warning     bg-warning-container*     text-on-warning-container* AlertTriangle
info        bg-primary-container      text-on-primary-container  Info
success     bg-success-container*     text-on-success-container* CheckCircle
```

---

## 4. Button variant="error" (Mở rộng primitive)

### Thay đổi trong Button.tsx
```tsx
const variantStyles = {
  // ...existing
  error: 'bg-error text-on-error hover:bg-error/90',
}
```

Cho phép viết `<Button variant="error" loading={isDeleting}>Delete</Button>` thay vì inline 18 dòng button.

---

## 5. MoveItemDialog Refactor

### Sau refactor
```tsx
export function MoveItemDialog({ item, workspaceId, isMoving, onConfirm, onClose }) {
  const [selectedFolderId, setSelectedFolderId] = useState<string | null>(null)
  if (!item) return null

  return (
    <Modal onClose={onClose} size="lg">
      <Modal.Header onClose={onClose}>Move to…</Modal.Header>
      {/* Info bar */}
      <div className="px-6 py-3 bg-surface-container-low border-b border-outline-variant">
        <p className="text-sm text-on-surface-variant">
          Moving <strong className="text-on-surface">{item.name}</strong>
        </p>
      </div>
      <Modal.Body>
        <FolderTreeSelect workspaceId={workspaceId} selectedId={selectedFolderId} onSelect={setSelectedFolderId} />
      </Modal.Body>
      <Modal.Actions>
        <Button variant="outline" onClick={onClose}>Cancel</Button>
        <Button variant="primary" disabled={!selectedFolderId} loading={isMoving}
          onClick={() => selectedFolderId && onConfirm(item, selectedFolderId)}>
          Move Here
        </Button>
      </Modal.Actions>
    </Modal>
  )
}
// ~25 dòng thay vì 123 dòng
```

---

## 6. FolderTreeSelect — Dọn inline Spinner

```diff
- <svg className="animate-spin h-3 w-3" ...>...</svg>
+ <Spinner size="sm" />
```

4 chỗ → 4 dòng thay vì 16 dòng SVG.

---

## 7. Skill: `component-reuse-checklist` (MỚI)

### Vị trí
`.agent/skills/component-reuse-checklist/SKILL.md`

### Nội dung (tóm tắt)
Bắt buộc check trước khi tạo BẤT KỲ component nào:

| # | Check | Action nếu YES |
|---|-------|-----------------|
| 1 | Pattern này đã có ở `primitives/`? | Dùng primitive |
| 2 | Pattern này đã có ở `composites/`? | Dùng composite |
| 3 | Component tương tự đã tồn tại ở module khác? | Extract shared |
| 4 | Inline HTML/CSS trùng >3 dòng với component có sẵn? | Dùng component |
| 5 | Có thể tái sử dụng cho ≥2 nơi khác? | Tạo composite mới |

**Forbidden patterns:**
- ❌ Inline SVG spinner khi `<Spinner>` đã có
- ❌ Inline modal shell khi `<Modal>` đã có
- ❌ Inline button styling khi `<Button variant="x">` đã có
- ❌ Copy-paste >5 dòng JSX giống nhau giữa 2 files

---

## 8. Cập nhật component-design.md

Thêm section **"Reuse Before Create"** vào skill `frontend-best-practices/references/component-design.md`:

```markdown
## Reuse Before Create (MANDATORY)

Before creating ANY new component, check:
1. Does this pattern exist in `primitives/`? → Use it
2. Does this pattern exist in `composites/`? → Use it
3. Is there a similar component elsewhere? → Extract to shared
4. Can this be used in ≥2 places? → Create in composites/

If the existing component is missing a feature:
- EXTEND the existing component (add prop/variant)
- Do NOT create a parallel implementation

Example: Need a red "Delete" button?
- ❌ Write inline <button className="bg-error...">
- ✅ Add variant="error" to Button primitive
```
