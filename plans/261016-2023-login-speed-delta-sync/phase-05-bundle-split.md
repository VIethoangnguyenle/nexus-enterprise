---
phase: 5
title: "Chia bundle"
status: pending
priority: P1
effort: "1d"
dependencies: [2]
---

# Phase 05: Chia bundle

## Requirements
- TanStack Router `autoCodeSplitting` (plugin ở `frontend/vite.config.js:140`); lazy tiptap/ChatEditor, EmojiPicker +
  dữ liệu emoji-mart, react-markdown, màn Quản trị/Phê duyệt/Tài sản/Tài liệu; `manualChunks` cho vendor lớn ổn định
  (react, tanstack, motion) để cache qua deploy.
- Prefetch chunk Trang chủ/chat khi rảnh sau khi shell hiện; màn đăng nhập không tải code chat.
- Mục tiêu: màn đăng nhập ≤ 250 KB nén, Trang chủ ≤ 450 KB nén (đo bằng build + gzip).

## Implementation Steps
1. Đo mốc: build + gzip từng chunk (lưu số vào report). Hook chặn đường dẫn `dist` → build ra thư mục scratch như phase 02.
2. Bật auto split, lazy các khối nặng có Suspense fallback đúng DESIGN.md (Skeleton), không nháy layout.
3. `npm test`, `npm run lint`, `npm run build`; test component lazy dùng `await findBy…`.

## Related Code Files
- Modify: `frontend/vite.config.js`, `frontend/src/components/spaces/SpaceView.tsx`, `chat/ChatEditor.tsx`,
  `chat/EmojiPicker.tsx`, route files

## Success Criteria
- [ ] Hai mục tiêu kích thước đạt; không đổi hành vi (test xanh).

## Risk Assessment
- Lazy làm chậm lần đầu mở màn chưa nạp → prefetch khi rảnh.
