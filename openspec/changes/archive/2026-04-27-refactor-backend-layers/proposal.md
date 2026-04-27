## Why

Backend services (đặc biệt Messaging và Drive) đang mắc 3 vấn đề kiến trúc nghiêm trọng cản trở việc scale và maintain:

1. **Hardcoded strings rải khắp nơi** — NGAC operations (`"read"`, `"write"`), node names (`"PC_Global"`, `"PublicUsers"`), và naming conventions (`"%s_Channels"`) xuất hiện dưới dạng raw strings trong 9 file khác nhau. Đổi tên 1 node → phải grep toàn codebase.
2. **Không tách layer** — Messaging service 524 dòng server.go mix handler + business logic + SQL queries. Không có store layer, không có domain layer. Hàm `CreateChannel` 137 dòng, if lồng 4 cấp. `findExistingDM` full table scan + N+1 gRPC calls.
3. **Gateway-Traefik redundancy** — CORS, WS proxy, routing đều duplicate giữa Traefik và Gateway. Gateway không validate input, `json.Decode` error bị ignore.

## What Changes

- Tạo shared `ngac` package chứa constants cho operations, node names, naming conventions — single source of truth
- **Messaging service**: Tách thành 3 layer: `handler/` (thin gRPC) → `domain/` (business logic) → `store/` (SQL). Fix N+1 DM lookup bằng DB-level query
- **Drive service**: Hoàn thiện layer separation — move direct DB calls từ handler xuống store/domain
- **Gateway**: Bỏ CORS middleware (Traefik xử lý), bỏ WS proxy (Traefik route trực tiếp), thêm input validation

## Capabilities

### New Capabilities
- `ngac-constants`: Shared Go package (`ngac-platform/ngac`) cung cấp typed constants cho operations, well-known node names, và naming convention functions. Thay thế ~40 raw strings rải trong 9 files.
- `messaging-layers`: Store + Domain layer separation cho Messaging service. Store xử lý SQL, Domain orchestrate business logic, Handler chỉ parse/delegate.
- `drive-layers`: Hoàn thiện Domain layer cho Drive service — extract business logic từ gRPC handler, loại bỏ direct DB calls trong handler.
- `gateway-cleanup`: Loại bỏ redundancy Gateway/Traefik, thêm input validation helper.

### Modified Capabilities
_(Không có spec hiện hữu nào bị thay đổi requirement — đây là refactor nội bộ, API contract giữ nguyên)_

## Impact

- **Backend services**: messaging, drive, gateway, workspace — tất cả sẽ import shared `ngac` package
- **Database**: Cần thêm table `channel_members` để tối ưu DM lookup (thay N+1 gRPC calls)
- **API contract**: Không thay đổi — REST endpoints và gRPC proto giữ nguyên hoàn toàn
- **Docker**: Bỏ WS proxy route khỏi Gateway labels (Traefik đã handle trực tiếp)
- **Tests**: Existing tests cần update imports nhưng logic không đổi
