## Why

NGAC Platform hiện có document management và messaging cơ bản, nhưng thiếu hai khả năng quan trọng cho SME: (1) messaging tích hợp sâu với business events thay vì chat đơn thuần, và (2) quản lý tài sản — bài toán mà mọi doanh nghiệp vừa và nhỏ đều gặp nhưng chưa có giải pháp nào kết hợp được access control granular kiểu NGAC. Đây cũng là cơ hội học kiến trúc hệ thống lớn: event-driven architecture, state machines, CQRS, saga patterns — tất cả trong một codebase thực tế.

## What Changes

- **Asset Service mới** (`services/asset/:50056`) — microservice quản lý tài sản generic với custom asset types, custom fields (JSON Schema), configurable lifecycle state machine, và NGAC-enforced access control cho mọi operation
- **Messaging Service nâng cấp** — tích hợp event-driven notifications từ Asset Service qua Kafka, hỗ trợ asset-linked threads, và notification center
- **Proto mới** (`proto/asset/`) — gRPC contract cho Asset Service bao gồm asset type definition, asset CRUD, lifecycle transitions, và request/approval flow
- **Messaging Proto mở rộng** — thêm notification types, thread linking, và asset event subscription
- **Gateway mở rộng** — route mới cho asset endpoints, proxy asset-related WebSocket events
- **Frontend mở rộng** — Asset management UI (list, create, request, approve, lifecycle view) và integrated notification trong chat
- **Kafka topics mới** — `asset.created`, `asset.requested`, `asset.approved`, `asset.assigned`, `asset.returned`, `asset.disposed` cho event-driven integration
- **NGAC graph mở rộng** — Policy Class cho Asset Management, OA hierarchy cho asset categories/types, UA associations cho department-level access

## Capabilities

### New Capabilities
- `asset-type-system`: Hệ thống định nghĩa asset type dynamic — admin tạo types (Laptop, Vehicle, License...) với custom fields schema (JSON Schema validation), configurable lifecycle states, và NGAC OA scaffolding tự động
- `asset-management`: CRUD operations cho asset instances — tạo, cập nhật, assign, transfer, dispose assets với NGAC access check trên mọi operation. Mỗi asset là Object (O) trong NGAC graph
- `asset-lifecycle`: State machine engine configurable per asset type — Request → Approved → Assigned → In-Use → Returned → Disposed. Mỗi transition yêu cầu NGAC permission check và emit Kafka event
- `asset-request-flow`: Quy trình request/approve tài sản — employee request, manager approve, IT assign. Multi-step approval với NGAC-enforced permissions tại mỗi bước
- `messaging-notifications`: Notification system tích hợp — asset events từ Kafka → tin nhắn tự động trong channels liên quan. Notification center trong UI
- `messaging-threads`: Thread support cho messages — gắn thread với asset cụ thể để discussion in-context. Asset state changes tự động post vào linked thread
- `asset-frontend`: React UI cho asset management — dashboard, type configuration, asset list/detail, request form, approval queue, lifecycle visualization
- `asset-ngac-model`: NGAC graph model cho assets — PC_AssetManagement, OA hierarchy theo category/type, UA associations cho department access, prohibitions cho individual asset restrictions

### Modified Capabilities
- `messaging-system`: Thêm notification message types, thread support, và Kafka consumer cho asset events
- `realtime-websocket`: Thêm asset notification events vào WebSocket broadcast
- `frontend-ui`: Thêm asset management pages và notification indicator vào existing layout

## Impact

- **New service**: `services/asset/` — Go microservice mới với cmd/, internal/grpc/, internal/domain/, internal/store/
- **New proto**: `proto/asset/asset.proto` — AssetTypeService + AssetService gRPC definitions
- **Modified protos**: `proto/messaging/messaging.proto` — thêm notification và thread messages
- **Modified service**: `services/messaging/` — thêm Kafka consumer, notification logic, thread support
- **Modified service**: `services/gateway/` — thêm asset routes
- **Database**: Bảng mới cho asset_types, asset_fields, assets, asset_transitions, asset_requests. Mở rộng messages table cho threads
- **Infrastructure**: Thêm asset service vào docker-compose.yml, Kafka topics mới
- **Frontend**: Thêm pages (AssetDashboard, AssetDetail, AssetTypeConfig, ApprovalQueue), components (AssetCard, LifecycleView, RequestForm), notification indicator
- **Dependencies**: JSON Schema validation library cho Go (có thể dùng `santhosh-tekuri/jsonschema`)
