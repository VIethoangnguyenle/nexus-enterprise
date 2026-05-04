## Why

Hệ thống AI orchestration hiện tại (autopilot pipeline) thiếu các cơ chế enforcement thực tế: thinking priority không nhất quán, DEV không có quality gate bắt buộc, QA không verify knowledge violations, và không có knowledge layer để hệ thống học từ experience. Sau nhiều lần chạy, system sẽ lặp lại cùng lỗi, không cải thiện chất lượng code, và không có cách nào đảm bảo consistency giữa các agent.

## What Changes

- Rewrite 7 core agent skills (CEO, BA, SA, UX, DEV, QC, Autopilot) với unified thinking priority 7 tầng, pre-flight checklists, và boundary protection
- DEV agent: thêm 8-step pre-flight checklist (merge artifact check + knowledge enforcement) + 7-point code quality gate bắt buộc
- QA agent: thêm knowledge violation check + regression check từ knowledge/bugs/
- Autopilot: upgrade từ 5-phase → 9-phase pipeline (CEO → BA → User → SA → UX → DEV → SA verify → QA → POLISH → DONE → LEARN)
- Thêm LEARN phase: conditional knowledge extraction từ QA reports + reviews
- Tạo Knowledge Layer (.agent/knowledge/) với hypothesis-based lifecycle (NEW → VALIDATED → DEPRECATED)
- Bootstrap 3 seed knowledge items (permission-before-mutation, no-unused-imports, explicit-error-handling)
- Deprecate 3 redundant skills (stitch-taste-design, stitch-design-md, stitch-loop)
- Update supporting files (autopilot workflow, AI-PRODUCT-OS-GUIDE, qa-memory)

## Capabilities

### New Capabilities
- `knowledge-layer`: Hypothesis-based knowledge system — index.yaml với lifecycle, scoring, applicability rules, usage_mode (strict/advisory), tier (standard/protected), bootstrap seeds
- `dev-quality-gate`: Mandatory 8-step pre-flight + 7-point code quality checklist cho DEV agent trước khi output code
- `learn-phase`: Post-pipeline knowledge extraction — conditional triggers, deduplication, pruning, reviews compaction

### Modified Capabilities
- `agent-skills-rewrite`: Rewrite 7 core skills với unified thinking priority, pre-flight checklists, boundary rules
- `pipeline-upgrade`: 5-phase → 9-phase pipeline với SA verify, POLISH, LEARN, User checkpoint

## Impact

- **Files modified:** 7 skill files (.agent/skills/agent-*/SKILL.md, autopilot/SKILL.md)
- **Files updated:** 3 supporting files (autopilot.md workflow, AI-PRODUCT-OS-GUIDE.md, qa-memory.md)
- **Files deprecated:** 3 skill files (stitch-taste-design, stitch-design-md, stitch-loop)
- **Files created:** 4 items (knowledge/ directories + index.yaml)
- **No code changes** — all changes are in .agent/ configuration files
- **No breaking changes** — existing features continue to work, pipeline behavior improves
