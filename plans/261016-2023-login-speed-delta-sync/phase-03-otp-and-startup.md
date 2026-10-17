---
phase: 3
title: "OTP gửi nền + khởi động ít vòng"
status: pending
priority: P1
effort: "1d"
dependencies: []
---

# Phase 03: OTP gửi nền, khởi động ít vòng

## Requirements
- `POST /api/auth/otp/request` trả ngay sau khi lưu mã; email gửi qua worker trong process (hàng đợi có giới hạn,
  kết nối SMTP dùng lại khi còn sống, thử lại 3 lần có backoff, log + đếm lỗi không chứa nội dung thư). Không lộ email
  tồn tại hay không; giới hạn tần suất giữ nguyên. Tắt service: chờ hàng đợi tối đa vài giây rồi bỏ (log số bỏ).
- Đo từng bước `POST /otp/verify` (đã thấy 1,2 s): Redis, DB, ký token, switch tenant — sửa bước chậm nếu có.
- Frontend khởi động: không `resetTenantScopedState()` khi tenant không đổi (`useSwitchWorkspace.ts:51`,
  `useTenantSync.ts:38-46`); không tải `/workspaces` hai lần; chunk route tải song song với `POST /auth/refresh`.

## Implementation Steps
1. Test trước: mailer giả chậm 5 s → request < 100 ms và thư vẫn gửi; mailer lỗi → request vẫn 200, lỗi được đếm;
   hàng đợi đầy → hành vi đã định (từ chối mềm + log), không treo.
2. `backend/pkg/mailer` (kết nối dùng lại) + worker trong `services/auth`; `bootstrap` đóng worker khi shutdown.
3. Test frontend: cùng tenant → không reset cache, `/workspaces` gọi một lần.
4. Spec `tenant-auth-flow`.

## Related Code Files
- Modify: `backend/services/auth/internal/domain/otp.go`, `backend/pkg/mailer/mailer.go`, `services/auth/cmd/main.go`,
  `frontend/src/hooks/useSwitchWorkspace.ts`, `useTenantSync.ts`, `frontend/src/routes/__root.tsx`, `_workspace.tsx`

## Success Criteria
- [ ] Test trên xanh; dev: `otp/request` < 100 ms với SMTP thật.

## Risk Assessment
- Mất thư khi container restart giữa chừng → chấp nhận, "Gửi lại mã" có sẵn (brainstorm đã chọn).
