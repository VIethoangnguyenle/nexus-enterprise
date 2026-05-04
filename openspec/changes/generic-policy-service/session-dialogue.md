# Session Dialogue: Generic Policy Service

> Evidence-based audit trail. Write-only: Orchestrator. Read-only: Human.
> KHÔNG narrative. KHÔNG roleplay. CHỈ structured reasoning.

---

## Phase 1: CEO

### D-001: Scope — Prohibitions IN, Obligations OUT

Quyết định:
- Prohibitions đưa vào generic service (NGAC core spec, pure graph logic)
- Obligations để consumer xử lý (business workflow, consumer owns event-response)
- Bằng chứng: file: `access.go`, vị trí: toàn file, quan sát: CheckAccess chỉ có BFS + association match, không có deny override logic — thiếu Prohibition
- Bằng chứng: file: `producer.go`, vị trí: line 22-30, quan sát: GraphMutatedEvent đã publish ra Kafka — consumer có thể subscribe và tự implement obligation logic
- Xác nhận: đã kiểm tra NGAC spec — Prohibitions là core component, Obligations là optional extension
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

### D-002: Size = L, Risk = Medium

Quyết định:
- Size L: refactor toàn bộ module path + thêm Prohibition feature + dynamic operations + new RPCs
- Risk Medium: core engine đã stable (cache 3 tầng đã verify), risk nằm ở Prohibition integration + backward compat
- Bằng chứng: file: `go.mod`, vị trí: line 1, quan sát: module path `ngac-platform/services/policy` cần đổi
- Bằng chứng: file: `models.go`, vị trí: line 14-24, quan sát: 8 hardcoded operation constants cần xóa
- Xác nhận: đã scan 27 files import `proto/policy` across 7 services — migration path cần cẩn thận
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Chuyển giao:
- CEO → BA: proposal.md đã tạo, scope rõ ràng, evidence đầy đủ

---

## Phase 2: BA

### D-003: Prohibition + Cache 3 tầng — Critical path identified

Quyết định:
- User feedback: prohibition tạo/xóa phải invalidate cache 3 tầng, nếu không sẽ trả ALLOW sai
- CreateProhibition/RemoveProhibition PHẢI chạy cùng flow invalidation như assignment/association changes
- L3 compute PHẢI check prohibition trước khi populate L2/L1 cache
- L2/L1 lưu decision CUỐI CÙNG (đã tính prohibition), KHÔNG chỉ kết quả BFS
- Bằng chứng: `materialized.go` lưu `decision bool` — phải là kết quả cuối cùng
- Bằng chứng: `cache_invalidator.go` dùng `InvalidateForNodes()` — prohibition cũng phải trigger cùng flow
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

---


