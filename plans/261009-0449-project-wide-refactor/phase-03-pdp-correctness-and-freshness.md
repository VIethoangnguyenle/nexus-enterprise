---
phase: 3
title: "PDP correctness and freshness"
status: pending
priority: P0
effort: 2-3d
dependencies: [1]
---

# Phase 03 — PDP correctness and freshness

## Overview
PDP có thể trả ALLOW sai và giữ quyết định cũ vô thời hạn. Sửa decision engine, EPP và cache.
Đây là thay đổi policy model → spec + test vector cùng commit (CLAUDE.md §5).

## Key insights
- **Fail-open:** `pdp_decision_engine.go:129-132` — query prohibition lỗi → log warn, trả "không
  có prohibition" → ALLOW. Batch path (`pdp_decision_batch.go:126`) thì fail-closed. Không nhất quán.
- **Singleflight dùng ctx của caller đầu** (`pdp_evaluator.go:74`): caller đó cancel → query
  prohibition lỗi → ALLOW → được cache vào Redis + materialized cache.
- **policy-read không có EPP:** `cmd/policy-read/main.go` chỉ `LoadGraph` lúc start, không consume
  `ngac.graph.mutated`. Trong docker-compose drive check qua policy-read.
- **Redis key mismatch:** invalidation pattern `ngac:access:<user>:*`
  (`epp_cache_invalidator.go:131`) không khớp key dạng `ngac:access:<ws>:<user>:…`
  (`pdp_decision_cache.go:154`).
- `WriteServer.DeleteNode` (`write_server.go:92`) resolve shard *sau* khi xoá node → không shard
  nào bị invalidate. `RemoveAssociationByUAOA` (`pap_store.go:154`) sửa graph trong RAM trước DB.
- Scope cache key bỏ qua workspace (`read_server.go:186`) — cùng loại với divergence đang mở của
  `drive-permission-engine`.
- Prohibition chạy DB query mỗi lần ALLOW — trái nguyên tắc "runtime check không chạm DB".
- Test PDP: mọi test `NewDecisionEngine` truyền nil cho CTE evaluator và prohibition store;
  `TestPH01` (`pdp_vnpay_scenarios_test.go:339`) không assert gì.

## Requirements
- Lỗi khi đánh giá prohibition → DENY, và quyết định do lỗi **không được cache**.
- Singleflight chạy với ctx tách khỏi caller (timeout riêng).
- Prohibitions load vào graph trong RAM, invalidate qua EPP như assignment/association.
- Mọi replica PDP (policy, policy-read) nhận EPP; key cache và pattern invalidate dùng chung một
  builder trong `backend/ngac`.
- Thứ tự PAP: DB trước, RAM sau, cho mọi mutation.

## Related files
- modify `backend/services/policy/internal/ngac/{pdp_decision_engine,pdp_evaluator,pdp_decision_cache,epp_cache_invalidator,pap_store,pip_store}.go`
- modify `backend/services/policy/internal/grpc/{write_server,read_server}.go`
- modify `backend/services/policy/cmd/policy-read/main.go`
- create test vectors trong `backend/services/policy/internal/ngac/` (prohibition deny, CTE deny,
  fail-closed, batch prohibition, stale-after-revoke)
- create `docs/specs/policy-decision-freshness/spec.md`; modify `docs/specs/batch-access-check/spec.md`,
  `docs/specs/drive-permission-engine/spec.md` (đóng Status divergence)

## Implementation steps
1. Test đỏ: prohibition store trả lỗi → mong DENY. Caller cancel giữa singleflight → không cache.
2. Test đỏ: revoke association trên policy → policy-read trả DENY trong ≤ N giây.
3. Sửa engine; thêm EPP consumer cho policy-read; gom key builder.
4. Sửa thứ tự DeleteNode / RemoveAssociationByUAOA, có test.
5. Thay `TestPH01` bằng assertion thật.

## Success criteria
- [ ] Không còn đường nào ra ALLOW khi có lỗi.
- [ ] Revoke có hiệu lực trên mọi replica mà không cần restart.
- [ ] Decision engine test với CTE + prohibition thật, có nhánh deny.

## Risks
- Load prohibitions vào RAM tăng bộ nhớ — đo số prohibition hiện có trước.
