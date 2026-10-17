---
phase: 2
title: "Ô soạn sát bàn phím trên iPhone PWA"
status: pending
priority: P1
effort: "4h"
dependencies: [1]
---

# Phase 02: Ô soạn sát bàn phím (iPhone, app đã cài)

## Overview
Người dùng báo: trên iPhone PWA, ô soạn cách bàn phím một khoảng **cố định** (không phải nhảy). Muốn như Google Chat.

## Hiện trạng (đã kiểm)
- `frontend/src/lib/app-height.ts`: shell `h-app` = `visualViewport.height` + translate theo `offsetTop`.
- `frontend/index.html:16`: `interactive-widget=resizes-content` (iOS bỏ qua).
- `frontend/src/components/chat/ChatEditor.tsx:480`: ô soạn luôn `mx-4 mb-4` (raised) / `mx-3 mb-3` → 16 px dưới ô soạn kể cả khi bàn phím mở.
- Không có `safe-area-inset-bottom` nào dưới ô soạn (vậy khi bàn phím đóng, 16 px có thể nằm trên vạch home 34 px).
- iOS gắn thanh phụ "‹ › Xong" trên bàn phím cho ô contenteditable — web không có API tắt; thuộc giới hạn nền tảng.

## Requirements
- Bàn phím mở: khoảng giữa đáy ô soạn và đỉnh bàn phím (gồm thanh phụ iOS) = 0 phần do app thêm.
- Bàn phím đóng: ô soạn không đè vạch home (`max(12px, env(safe-area-inset-bottom))`).
- Máy tính, Android: không đổi hành vi ngoài phần nêu trong DESIGN.md mới.

## Implementation Steps
1. **Đo trước (bắt buộc, theo ak:debug):** thêm tạm (sau cờ `?kbdebug=1`, không commit) một dải chữ nhỏ hiện
   `innerHeight`, `visualViewport.height/offsetTop`, `getBoundingClientRect().bottom` của ô soạn, `--app-height`.
   Người dùng mở trên iPhone PWA, chụp màn khi bàn phím mở và đóng. Từ số đo tách: (a) margin app, (b) thanh phụ iOS,
   (c) sai lệch `visualViewport` (nếu `rect.bottom < vv.height + vv.offsetTop - mb`).
2. Nếu (c) ≠ 0 → chẩn đoán riêng trong `app-height.ts` (vd đọc trong `requestAnimationFrame` sau `resize`, hoặc
   iOS standalone trả `offsetTop` âm) — test trong `app-height.test.ts` trước khi sửa.
3. **Test trước (TDD):** `useKeyboardOpen()` (hook mới, `frontend/src/hooks/useKeyboardOpen.ts`): true khi
   `innerHeight - visualViewport.height > 120`; test với `VisualViewportLike` giả như `app-height.test.ts`.
   Test `ChatEditor`: khi bàn phím mở, gốc có lớp tràn ngang (không `mx-*`/`mb-*`, không bo); khi đóng, có
   `mb-[max(0.75rem,env(safe-area-inset-bottom))]`.
4. Sửa `ChatEditor.tsx:480` theo DESIGN.md mới; áp cả ô soạn trong `SidePanel` (thread trên điện thoại, đã `h-app`).
5. Gỡ dải đo; người dùng kiểm lại trên iPhone, chụp màn.

## Related Code Files
- Create: `frontend/src/hooks/useKeyboardOpen.ts` (+ test)
- Modify: `frontend/src/components/chat/ChatEditor.tsx`, test của nó; có thể `frontend/src/lib/app-height.ts`
- Modify: DESIGN.md "Ô soạn" (điện thoại) — đã thiết kế ở phase 01

## Success Criteria
- [ ] Số đo trước/sau từ iPhone PWA: phần app thêm dưới ô soạn = 0 khi bàn phím mở.
- [ ] Bàn phím đóng: ô soạn trên vạch home.
- [ ] Test hook + ChatEditor xanh; máy tính không đổi (test hiện có xanh).

## Risk Assessment
- Thanh phụ iOS vẫn còn → nói rõ với người dùng đây là giới hạn của web, không phải lỗi; không giả lập bằng hack.
- Ngưỡng 120 px sai với bàn phím nổi iPad → chỉ áp `max-md`.
