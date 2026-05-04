# Stitch Full UI Refactor

## What
Refactor toàn bộ frontend UI để đạt pixel-perfect parity với 24 Stitch screens trong Nexus Hub project, sử dụng mandatory Stitch-First workflow (fetch source → extract tokens → implement).

## Why
- Current UI dùng dual-token system (legacy aliases + Material 3) gây visual drift
- ~30 files vẫn dùng legacy tokens (`bg-bg-primary`, `text-text-secondary`)
- Responsive gaps lớn: Stitch có tablet/mobile screens nhưng frontend chưa implement đầy đủ
- 1 module (Workplace) chưa build

## Scope
- 24 Stitch screens across 7 modules
- ~55 frontend files (routes + components)
- 8 phases, ưu tiên foundation → independent → dependent

## Out of Scope
- Backend logic changes
- New API endpoints
- Database schema changes
