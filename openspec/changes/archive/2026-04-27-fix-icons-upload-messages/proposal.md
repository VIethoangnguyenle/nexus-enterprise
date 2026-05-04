# Fix Icons, Upload & Message Order

## Problem

Three critical UX/functionality issues identified during QA:

1. **Icons quá nhỏ & mono-color** — Sidebar (AppRail) và toàn bộ UI sử dụng emoji Unicode (`💬`, `📄`, `⚙️`...) thay vì icon library. Emoji render phụ thuộc OS/browser, kích thước không kiểm soát được, thiếu visual hierarchy. So với Lark, icons phải lớn hơn, có màu sắc riêng biệt.

2. **File upload broken** — Upload từ Drive page hoặc chat attachment không hoạt động. 2 nguyên nhân:
   - **Proxy routing sai:** `/api/workspaces/:id/drive/*` bị route tới workspace service (:8181) thay vì drive service (:8185). Workspace service không có endpoint drive → 404.
   - **Field name mismatch:** Frontend gửi `{ filename: "..." }` nhưng drive handler expects `{ name: "..." }` → file được tạo với name rỗng.

3. **Thứ tự tin nhắn đảo ngược** — Backend dùng `ORDER BY created_at DESC` (cursor-based load more), frontend render trực tiếp mà không reverse → tin nhắn mới nhất hiện ở trên, cũ nhất ở dưới. (Đã fix sơ bộ bằng `.reverse()` nhưng cần review toàn diện.)

## Goal

Sửa 3 vấn đề trên để đạt được:
- Icon system đẹp, nhất quán, kích thước tương đồng Lark
- File upload hoạt động end-to-end (Drive page + chat attachment)
- Tin nhắn hiển thị đúng thứ tự: cũ trên, mới dưới

## Scope

### In Scope

1. **Lucide React icon library** — Thay thế toàn bộ emoji Unicode bằng SVG icons
2. **Vite proxy routing fix** — Route `/api/workspaces/:id/drive/*` tới drive :8185
3. **Drive API field name fix** — Frontend `filename` → `name` hoặc backend accept cả hai
4. **Message order fix** — Reverse messages array trước khi render (đã apply)
5. **Drive handler nil-pointer fix** — Apply `RequireClaims` cho drive handlers

### Out of Scope

- Redesign toàn bộ sidebar layout (chỉ fix icons)
- Drag-and-drop upload
- File preview/thumbnail generation
- Infinite scroll load-more cho messages

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Icon library | Lucide React | Tree-shakable, 1400+ icons, cùng style Lark/Notion, SVG gọn |
| Proxy fix strategy | Add drive/document regex in `configure` | Giữ cùng pattern đã dùng cho channels routing |
| Field name fix | Frontend sửa `filename` → `name` | Backend đã deploy, ít risk hơn sửa backend |
| Message order | Frontend `.reverse()` | Backend DESC+cursor là pattern chuẩn cho load-more |

## Risk

| Risk | Mitigation |
|------|-----------|
| Lucide bundle size | Tree-shaking — chỉ import icons sử dụng |
| Proxy override `proxy.web` fragile | Integration test qua browser |
| MinIO presigned URL từ Docker | Verify `MINIO_ENDPOINT` env var cho native dev |
