# Hướng dẫn sử dụng AI Product OS

## Flow duy nhất

```
/opsx-explore "ý tưởng"        ← Bạn tham gia brainstorm
/autopilot "feature name"      ← Bấm 1 lần, ngồi chờ
/opsx-archive <name>           ← Đóng khi xong
```

**3 lệnh. Bạn chỉ can thiệp ở ý tưởng + intent checkpoint.**

---

## Chuyện gì xảy ra khi chạy `/autopilot`?

```
/autopilot "Approval module frontend"

  [1/9] ✅ CEO — Size: M, Risk: Low
  [2/9] ✅ BA — 5 stories, 14 AC. Spec locked.
  [3/9] ⏸️ User Checkpoint — "Proceed? (Y/feedback)"
  [4/9] ✅ SA — Architecture verified.
  [5/9] ✅ UX — 4 screens. Stitch done.
  [6/9] ✅ Dev — 12 files. Quality gate: ALL PASS.
  [7/9] ✅ SA verify — Architecture compliant.
  [8/9] ✅ QA — All pass. Knowledge: 0 violations.
  [9/9] ✅ POLISH — 2 fixes applied.

  ✅ Feature complete!
  📚 LEARN: 1 new pattern extracted.
  Run /opsx-archive to close.
```

Tất cả chạy tự động trong **1 conversation**. Bạn chỉ thấy progress.

---

## Khi nào bạn cần làm gì thêm?

### Trường hợp 1: Mọi thứ ổn (90% cases)
```
Bạn: /autopilot "feature"
AI:  ...chạy 9 phase...
AI:  [3/9] ⏸️ Proceed? Y
Bạn: Y
AI:  ...tiếp tục...
AI:  ✅ Feature complete!
Bạn: /opsx-archive <name>
```

### Trường hợp 2: Context overflow (hiếm, feature lớn)
```
Bạn: /autopilot "feature lớn"
AI:  [1/9] ✅ CEO...
AI:  [6/9] Dev... task 7/10...
AI:  ⚠️ Context limit. Run "/autopilot <name>" to continue.
Bạn: /autopilot <name>      ← conversation mới
AI:  Resuming Dev task 8/10...
AI:  ✅ Feature complete!
```

### Trường hợp 3: User Checkpoint reject (hiếm)
```
AI:  [3/9] ⏸️ User Checkpoint — scope review
Bạn: Cần thêm bulk actions
AI:  → CEO revise scope → BA update specs → continue...
```

---

## Cơ chế bên trong (bạn không cần biết, nhưng nó hoạt động)

| Cơ chế | Tác dụng | Bạn thấy gì |
|--------|---------|-------------|
| **Spec Lock** | BA lock spec sau khi xong. Không ai sửa được. | Không thấy gì |
| **QA Memory** | QC nhớ lỗi cũ, test regression tự động | Không thấy gì |
| **Checkpoint** | Dev ghi progress, resume nếu crash | Chỉ thấy khi context overflow |
| **Review Routing** | Agent reject nhau qua reviews.yaml | Không thấy gì |
| **Role Enforcement** | Mỗi agent chỉ làm việc của mình | Không thấy gì |
| **Quality Gate** | DEV tự kiểm 7 điểm trước khi output code | Không thấy gì |
| **Knowledge Layer** | System học từ experience, tránh lặp lỗi | 📚 LEARN summary |
| **SA Verify** | SA kiểm code vs architecture sau DEV | Không thấy gì |
| **User Checkpoint** | Validate intent trước khi build | ⏸️ Pause tại Phase 3 |

---

## So sánh trước vs sau

| | v2 | v3 |
|---|---|---|
| **Phases** | 5 phases | 9 phases |
| **User checkpoint** | Không | Sau BA (Phase 3) |
| **Architecture** | Implicit | SA + SA verify |
| **Quality gate** | Optional | Mandatory 8+7 checklist |
| **Knowledge** | Không có | Hypothesis-based, auto-learn |
| **POLISH** | Manual in QA loop | Dedicated phase, max 2 rounds |
| **Learning** | Không | LEARN phase, conditional triggers |

---

## Commands

| Command | Mục đích |
|---------|---------|
| `/opsx-explore "ý tưởng"` | Brainstorm, khám phá, quyết định |
| `/autopilot "feature"` | Chạy toàn bộ pipeline 9-phase tự động |
| `/autopilot <name>` | Resume nếu bị gián đoạn |
| `/opsx-archive <name>` | Đóng change khi hoàn thành |
