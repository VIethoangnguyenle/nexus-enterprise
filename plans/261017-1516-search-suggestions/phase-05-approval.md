---
phase: 5
title: "Phê duyệt: ô tìm + gợi ý"
status: completed
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

## Thực tế khi làm (2026-10-17)
- Ô "Tìm đề nghị" có trên mọi tab trừ Mẫu. Lọc ngay trong trình duyệt theo tiêu đề (tên mẫu), người gửi và loại đề
  nghị; từ khớp được tô; bên cạnh ô ghi "N trong M đề nghị". Chuỗi tìm ghi vào `?q=` sau 300ms (replace); đổi tab thì
  xoá. Không khớp gì: EmptyState có nút "Xoá tìm kiếm". Spec ghi rõ chỉ tìm trong những đề nghị đã tải.
- Khi đang tìm, nút chọn tất cả chỉ tích các hàng đang hiện.
- Gợi ý: "Tìm gần đây", "Chờ bạn duyệt", "Đề nghị của bạn" (tải khi ô được focus lần đầu).
- Tách `Marked` (tô từ khớp) thành `components/patterns/Marked.tsx`, dùng chung với Tìm trong chat.
- Review gộp 04+05: `reports/code-review-261017-phase-04-05.md`. Đã sửa:
  - H1: duyệt hàng loạt và chọn tất cả chỉ tính các hàng đang hiện; có test cho trường hợp tích 2 rồi lọc còn 1.
  - M1: thanh tab Tài sản ẩn khi gợi ý che danh sách trên điện thoại.
  - M2: đổi tab Phê duyệt xoá chuỗi tìm ngay, kể cả khi vừa gõ dưới 300ms; link có `?q=` vẫn giữ chuỗi tìm.
  - M3: đồng bộ URL không còn ghi đè chữ vừa gõ (`writtenQ`), cho cả Phê duyệt và Tài sản.
  - Nhỏ: tên người làm trong "Hoạt động gần đây" khớp với avatar; view Được chia sẻ dùng nhãn "Sửa gần đây".
- Để lại: đóng Trình xem tệp mở từ gợi ý thì focus về ô tìm và gợi ý mở lại; Phê duyệt vẽ lại mọi hàng mỗi lần gõ;
  hàng khớp theo người gửi hay loại thì không có chữ được tô.
- Kiểm tra: vitest 3126/3126, lint ok, `typecheck:diff` không tăng lỗi, `vite build` ok.
