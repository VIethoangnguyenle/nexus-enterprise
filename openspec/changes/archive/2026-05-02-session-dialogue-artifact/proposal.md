## Why

Autopilot pipeline hiện tại thiếu audit trail cho inter-agent reasoning. `reviews.yaml` chỉ capture routing decisions (ai reject ai), nhưng KHÔNG capture: tại sao challenge xảy ra, evidence nào được dùng, confidence có cơ sở không. Kết quả: human không thể verify chất lượng reasoning, không detect được hallucination, và không biết agents có thực sự challenge lẫn nhau không.

Risk lớn nhất: Orchestrator vừa reasoning vừa tự báo cáo — không có independent verification. Session dialogue là self-reported, nhưng với evidence-based enforcement, human có thể detect sai bằng cách đọc.

## What Changes

- Thêm artifact mới `session-dialogue.md` cho mỗi change (per-change, không global)
- Update autopilot SKILL.md: thêm Session Dialogue Protocol với write triggers, evidence rules, red flag detection
- Mỗi decision phải có evidence cụ thể (file + location + observation) — không chấp nhận citation mơ hồ
- Mỗi deviation phải có validator xác nhận — validator phải kiểm tra evidence, không chỉ "agree"
- Decision status: VERIFIED (evidence + validator confirm) hoặc UNVERIFIED (chưa đủ)
- Confidence gắn với evidence strength: HIGH (nhiều evidence rõ + validated), MEDIUM (partial), LOW (assumption)
- Red flags tự động detect: thiếu evidence, confidence cao nhưng evidence yếu, validator không kiểm tra

## Capabilities

### New Capabilities
- `session-dialogue`: Evidence-based audit trail cho inter-agent reasoning — chỉ log decisions, conflicts, deviations (không log routine). Write-only cho Orchestrator, read-only cho human. Tách biệt hoàn toàn khỏi reviews.yaml.

### Modified Capabilities
- `autopilot-pipeline`: Thêm session-dialogue creation ở Phase 0, append triggers sau mỗi phase có conflict/deviation, summary ở Phase 10

## Impact

- **Files modified:** 1 file (`.agent/skills/autopilot/SKILL.md`)
- **Files created:** 0 (template nằm trong SKILL.md instructions, file được tạo runtime per-change)
- **No code changes** — tất cả thay đổi nằm trong `.agent/` configuration
- **No breaking changes** — pipeline behavior chỉ thêm write operations, không thay đổi flow
