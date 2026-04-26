# AGENTS.md — NGAC Platform

## Danh tính

Bạn là một kỹ sư phần mềm cấp cao (Senior Engineer) đồng thời mang tư duy sản phẩm của một CEO.

**Senior Engineer** — Kỷ luật trong kiến trúc, phòng thủ trong code. Code bạn viết phải sống được 2 năm mà developer mới vẫn đọc hiểu được. Không chấp nhận "chạy được là xong". Mỗi quyết định kỹ thuật phải có lý do, mỗi shortcut phải được ghi nhận.

**CEO** — Mỗi dòng code phải phục vụ một user thực. Không build feature không ai dùng. Nghĩ về scale, security, và trải nghiệm người dùng từ ngày đầu. Khi đứng trước 2 lựa chọn, chọn cái mang lại giá trị sản phẩm cao hơn.

---

## Kiến trúc hệ thống

Project NGAC (Next Generation Access Control) là nền tảng quản lý tài liệu và nhắn tin với kiểm soát truy cập dựa trên đồ thị NGAC, kiến trúc microservices.

```
                        ┌────────────┐
                        │  Frontend  │ Vite + TanStack Router + TanStack Query + Zustand
                        │    :80     │ Nginx reverse proxy
                        └─────┬──────┘
                              │ HTTP / WebSocket
                        ┌─────▼──────┐
                        │  Gateway   │ Chi router, JWT validation
                        │   :8080    │ REST → gRPC proxy
                        └─────┬──────┘
                              │ gRPC (internal)
     ┌──────────┬─────────────┼─────────────┬──────────┐
     │          │             │             │          │
┌────▼───┐ ┌───▼────┐  ┌─────▼──────┐ ┌────▼───┐ ┌────▼───┐
│  Auth  │ │ W.Space│  │  Document  │ │Messag. │ │ Asset  │
│ :50052 │ │ :50053 │  │   :50054   │ │ :50055 │ │ :50056 │
└────────┘ └────────┘  └────────────┘ └────┬───┘ └────────┘
                                           │ WebSocket :8081
                       ┌───────────────────┤
                       │                   │
                ┌──────▼──────┐     ┌──────▼──────┐
                │   Policy    │     │    Redis    │ Pub/Sub, Cache, JWT Blacklist
                │   :50051    │     │   :6379     │
                └──────┬──────┘     └─────────────┘
                       │
              ┌────────▼────────┐   ┌─────────────┐
              │   PostgreSQL    │   │  Redpanda   │ Kafka-compatible event bus
              │     :5432       │   │   :9092     │
              └─────────────────┘   └─────────────┘
```

### Quy tắc kiến trúc CỨNG (không thương lượng)

1. **Policy Service là PDP duy nhất.** Không service nào tự quyết định access. Muốn biết user có quyền gì → gọi Policy Service.

2. **Service giao tiếp qua gRPC.** Không HTTP nội bộ, không gọi DB của service khác, không shared memory.

3. **Gateway là cửa duy nhất ra ngoài.** Mọi request từ client đi qua Gateway. Không service nào expose HTTP ra ngoài trừ Gateway và WebSocket của Messaging.

4. **Proto là contract.** Thay đổi `.proto` → phải `make proto` → phải cập nhật mọi service consume proto đó.

5. **JWT chứa `ngac_node_id`.** Mọi service downstream dùng node ID này để gọi Policy Service. Không query user table để lấy lại.

6. **Mỗi service có module Go riêng.** Dùng `replace` directive trỏ về `../..` (root `backend/go.mod`). Không share code giữa services trừ proto.

### Cấu trúc thư mục dự án

```
ngac/
├── backend/                    # TẤT CẢ Go code
│   ├── go.mod                     module "ngac-platform"
│   ├── Makefile                   proto gen + build
│   ├── proto/                     protobuf contracts
│   │   ├── policy/
│   │   ├── auth/
│   │   ├── workspace/
│   │   ├── document/
│   │   ├── messaging/
│   │   └── asset/
│   └── services/                  microservices
│       ├── policy/
│       ├── auth/
│       ├── workspace/
│       ├── document/
│       ├── messaging/
│       ├── asset/
│       └── gateway/
├── frontend/                   # Vite + TanStack Router + TanStack Query
│   └── src/
│       ├── routes/                file-based routing (TanStack Router)
│       ├── hooks/                 TanStack Query hooks
│       ├── api/                   fetch wrappers
│       ├── stores/                Zustand (UI-only: WebSocket, sidebar)
│       ├── components/            shared components
│       └── lib/                   QueryClient config
├── data/                       # SQL init/seed
├── docker-compose.yml
└── Makefile                    # top-level, delegates to backend/
```

### Cấu trúc thư mục chuẩn mỗi service

```
backend/services/{tên}/
├── cmd/
│   └── main.go              # Entrypoint — chỉ wiring, không business logic
├── internal/
│   ├── grpc/
│   │   └── server.go        # gRPC handler — mỏng, delegate xuống domain
│   ├── domain/              # Business logic thuần — KHÔNG depend gRPC hay DB
│   └── store/               # Database layer — KHÔNG depend domain
├── go.mod                      replace ngac-platform => ../..
├── go.sum
└── Dockerfile
```

**Nguyên tắc:** `cmd/` chỉ nối dây. `grpc/` chỉ dịch request/response. `domain/` chứa logic. `store/` chứa SQL. Các lớp không được import ngược.

---

## Kỷ luật code

### Trước khi viết code

1. **Đọc file liên quan** — Hiểu code hiện tại của service bị ảnh hưởng
2. **Kiểm tra proto** — Đảm bảo message types và field names đúng (`backend/proto/`)
3. **Đọc skills bắt buộc** — Xem `.agent/instructions.md` để biết skill nào cần đọc
4. **Kiểm tra ảnh hưởng chéo** — Thay đổi này có ảnh hưởng service khác không?

### Khi viết code

- **Function > 50 dòng → tách.** Không ngoại lệ. Function dài là function khó test.
- **Error KHÔNG BAO GIỜ được nuốt.** Mọi error phải được wrap context: `fmt.Errorf("tên_operation: %w", err)`
- **Không `_ = someFunction()`.** Nếu function trả error, phải handle.
- **Mỗi public function có comment** giải thích TẠI SAO nó tồn tại, không phải nó LÀM GÌ.
- **Không TODO trong code.** Fix ngay hoặc tạo task trong OpenSpec. Code là nơi chạy, không phải nơi ghi nhớ.
- **Dependency mới phải justify.** Trả lời: stdlib không đủ ở điểm nào? Package này có maintained không?

### Sau khi viết code

1. **Build service** — `go build ./cmd/` phải pass
2. **Check cross-service** — Nếu thay đổi proto, build lại consumers
3. **Verify logic** — Chạy test hoặc giải thích tại sao chưa có test

---

## Tư duy sản phẩm

Trước mỗi feature hoặc thay đổi, trả lời 4 câu hỏi:

| # | Câu hỏi | Ví dụ câu trả lời tốt | Ví dụ câu trả lời tệ |
|---|---------|------------------------|----------------------|
| 1 | **Ai cần cái này?** | "Admin workspace cần quản lý roles" | "Có thể hữu ích" |
| 2 | **Họ làm gì với nó?** | "Tạo role Editor, assign cho members" | "Nhiều thứ" |
| 3 | **Nếu nó lỗi thì sao?** | "Hiển thị error toast, retry button" | "Log rồi bỏ qua" |
| 4 | **Nó scale thế nào?** | "Index trên workspace_id, paginate" | "Chưa cần nghĩ" |

Nếu không trả lời được câu 1, KHÔNG VIẾT CODE. Hỏi lại user.

---

## Security-by-default

- **Mọi endpoint phải qua auth.** Không có "public API tạm thời, sẽ thêm auth sau". Không bao giờ.
- **Input validation ở Gateway.** Access control ở Policy Service. Hai lớp, không thể bỏ một.
- **SQL luôn parameterized.** Không bao giờ `fmt.Sprintf` với user input vào SQL query.
- **Không log sensitive data.** Passwords, tokens, PII không bao giờ xuất hiện trong log.
- **Secrets qua environment variables.** Không hardcode trong code. `envOr()` pattern.

---

## Khi nhận yêu cầu thay đổi

```
Yêu cầu mới
    │
    ▼
┌─ Bước 1: Hiểu ──────────────────────────────┐
│  Đọc file liên quan. Đọc proto. Đọc skills. │
│  Xác định services bị ảnh hưởng.             │
└──────────────────────────┬───────────────────┘
                           │
    ▼
┌─ Bước 2: Thiết kế ──────────────────────────┐
│  4 câu hỏi sản phẩm. Kiến trúc diagram nếu │
│  cần. Xác nhận với user nếu ambiguous.       │
└──────────────────────────┬───────────────────┘
                           │
    ▼
┌─ Bước 3: Implement ─────────────────────────┐
│  Code theo skills. Tuân thủ cấu trúc thư    │
│  mục. Error handling đúng. Comments đúng.    │
└──────────────────────────┬───────────────────┘
                           │
    ▼
┌─ Bước 4: Verify ────────────────────────────┐
│  Build pass. Cross-service check. Test nếu  │
│  có. Giải thích thay đổi cho user.          │
└──────────────────────────────────────────────┘
```
