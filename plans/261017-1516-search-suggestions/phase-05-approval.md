---
phase: 5
title: "Phê duyệt: ô tìm + gợi ý"
status: pending
priority: P2
effort: "4h"
dependencies: [2]
---

# Phase 05: Phê duyệt

## Overview
Phê duyệt chưa có ô tìm. Phase này thêm ô tìm lọc phía client trên danh sách của tab đang mở (theo cách tìm đã chốt ở
phase 01), rồi gắn panel gợi ý vào ô đó.

## Requirements
- Ô tìm có `moduleSearch`, để thanh tìm trên điện thoại dẫn tới nó. Lọc theo `matchesAll` (bỏ dấu, mọi từ đều phải
  khớp) trên các trường đã chốt ở phase 01. Chuỗi tìm lưu vào URL (`?q=` trong `ApprovalSearch`); `tabSearch` xoá nó.
- Gợi ý:
  - "Tìm gần đây".
  - "Chờ bạn duyệt" (dùng `approvalPendingOptions` đang có).
  - "Đề nghị của bạn" (`/my-requests`).
  - Chạm một đề nghị thì gọi `requestSearch`.
- Không có ô tìm khi danh sách đang tải lỗi.

## Related Code Files
- Modify: `frontend/src/lib/approval-search.ts` + test (`q`)
- Modify: `frontend/src/components/approval/ApprovalScreen.tsx` + test
- Create: `frontend/src/lib/suggestions/approval.ts` + test

## Implementation Steps (TDD)
1. Test `validateApprovalSearch`: `q` rỗng hoặc chỉ có khoảng trắng thì bị bỏ; `tabSearch` xoá `q`.
2. Test lọc: "de nghi mua" tìm ra "Đề nghị mua laptop"; tab khác vẫn giữ bộ lọc riêng.
3. Test gợi ý: deny (đề nghị mình không xem được thì không có trong gợi ý); không có ID; hiện tên người gửi.
4. Viết code.

## Success Criteria
- [ ] Thanh tìm trên điện thoại ở Phê duyệt hiện "Tìm đề nghị" và dẫn tới ô tìm này.
- [ ] Test xanh.

## Risk Assessment
Lọc phía client chỉ thấy trang đang tải. Nếu danh sách có phân trang thì phải ghi rõ trong spec là chỉ tìm trong những
gì đã tải; nếu Zane không chấp nhận thì xem lại Risk của phase 01.
