---
type: brainstorm
date: 2026-10-17
topic: Danh bạ redesign theo Lark
status: accepted
---

# Danh bạ kiểu Lark

## Quyết định (Zane, 2026-10-17)
- Bố cục **A: 3 cột** kiểu Lark.
- Hồ sơ có **Nhắn tin + Gọi + Email** (gọi đã có: `frontend/src/hooks/useCalls.ts`).
- Sắp xếp người theo **tên (chữ cuối)**, rồi cả họ tên (khác code hiện tại `sortContacts` xếp theo cả họ tên).
- **Gọi** luôn bấm được, kể cả khi người kia ngoại tuyến (đổ chuông thiết bị của họ).
- Thêm mục **Vai trò** ở nav (workspace doanh nghiệp): vai trò tự tạo + nhóm "Quản trị viên" (owners), không liệt kê "Thành viên". Hồ sơ có dòng Vai trò. Backend: contacts response thêm vai trò hiển thị được của mỗi người (hiện role→members chỉ admin đọc được qua `GetRoleDetail`, cần `OpManage`). Đây là mở rộng quyền xem → plan phải nêu rõ, có test deny cho workspace khác.
- **Màn chính kiểu Lark: chỉ có danh mục, bấm mới tải** (đã thử trên điện thoại, mockup §7). Tổ chức: Phòng ban · Vai trò · Nhóm của tôi · [Bên ngoài · Danh bạ chia sẻ = đợt 2] + lối tắt "Phòng của tôi". Bỏ "Tất cả thành viên" ở màn chính (tìm kiếm phủ cả workspace; Phòng ban › Toàn NovaPay để xem cả công ty). Mỗi danh mục gọi API riêng khi mở; thành viên nhóm tải khi mở nhóm đó → giải luôn câu hỏi "N lần gọi". `useContacts` vẫn tải sẵn toàn app (people.ts, sidebar, gợi ý tìm kiếm đều dùng) nên không lazy phần người.
- **Phân biệt workspace cá nhân và workspace tổ chức**: cá nhân = Thành viên (liệt kê thẳng, ít người) · Nhóm của tôi · [Liên hệ = đợt 2]; không Phòng ban, Vai trò, Danh bạ chia sẻ, Phòng của tôi; hồ sơ không có Phòng ban, Vai trò, Cùng phòng.
- **Desktop + tablet giống điện thoại** (Zane, 2026-10-17; mục đích: một màn không tải và không hiện quá nhiều): cột trái (tablet: chip) chỉ có danh mục; cây phòng ban, danh sách vai trò, nhóm duyệt ở cột giữa có đường dẫn (Vai trò › X, Nhóm của tôi › X); mỗi danh mục tải khi bấm.
- **Cây phòng ban nhiều cấp** (mockup §1b): một cấp mỗi màn, phòng có cấp con có dòng phụ "N phòng, nhóm bên dưới", đường dẫn đủ mọi cấp, không giới hạn độ sâu.
- **Phạm vi hiển thị danh bạ làm luôn trong đợt 1** (Zane, 2026-10-17; mockup §1c). Hiện trạng: mọi thành viên thấy tất cả (`ListDepartments` chỉ authorizeMember; `ListContacts` chỉ kiểm `tenant_users`, **không qua NGAC**). Quy tắc quản trị viên đặt: (1) ẩn một phòng (cả cây con) → chỉ người trong phòng + quản trị viên thấy; (2) một vai trò hoặc phòng chỉ thấy phòng của mình. Quản trị viên luôn thấy tất cả, phòng ẩn có nhãn. Ẩn = không trong cây, không tìm ra, không đếm, không trong danh sách vai trò. Quyết bằng NGAC ở API (không lọc ở UI) → thay đổi policy model: dùng skill `ngac-policy-change`, spec + test vector + test deny. PDP có prohibition (subject, ops, target OAs, intersection) nhưng **không có phần bù**, nên "chỉ thấy phòng mình" khó làm bằng prohibition; hướng nghiêng về association theo OA danh bạ từng phòng (chốt ở plan).
- Rà soát lỗi vượt quyền hiện có: report riêng `plans/reports/security-261017-1630-authz-overprivilege-audit.md` (F1–F10). Quyết định (Zane, 2026-10-17):
  - Nhóm chat **riêng tư**, chỉ thành viên nhóm đọc/ghi (F1 = lỗi, sửa; thành viên workspace chỉ giữ `create_channel` trên Channels OA).
  - DM / gọi **chỉ giữa người chung workspace** (F4); xuyên tổ chức để đợt 2 (Bên ngoài).
  - Xoá thành viên = **đánh dấu removed + cắt phiên** (F3, F5).
  - F6: chưa hỏi; mặc định xoá người có `manage` cần `manage`.
  - **Plan bảo mật riêng, làm trước**; plan Danh bạ đợt 1 `blockedBy` plan bảo mật (F2, F3, F7 là nền cho phạm vi hiển thị).
- **Mockup trước** (`design/mockups/contacts-lark.html`), duyệt xong mới `ak:plan` → `ak:cook --tdd`.

## Contract
- **Outcome:** Danh bạ 3 cột: nav trái (Tổ chức · Tất cả thành viên · Nhóm của tôi), cột giữa duyệt cây tổ chức có breadcrumb (phòng con + số người trước, thành viên sau), thẻ hồ sơ lớn (avatar, tên, chức danh, đường dẫn phòng ban đầy đủ, Nhắn tin / Gọi / Email, cùng phòng). Tablet/mobile = drill-down xếp tầng.
- **Constraints:** design-first (DESIGN.md + mockup trước code). Không id/UUID trên màn. Chỉ mục có dữ liệu thật. Workspace cá nhân không có cây tổ chức (chỉ Tất cả thành viên). Route mới phải vào vite proxy. Tìm kiếm vẫn toàn workspace.
- **Non-goals:** liên hệ bên ngoài (đợt 2, xem dưới), gắn sao, quản lý trực tiếp, số điện thoại (không có trong schema), sửa phòng ban từ Danh bạ (thuộc Quản trị).
- **Acceptance:** mockup duyệt; duyệt cây tới đúng người kể cả hai phòng trùng tên khác nhánh; Nhắn tin + Gọi chạy thật; vitest + Go test cho `department_id` xanh; spec `docs/specs/contacts-documents-settings-screens/spec.md` cập nhật (bỏ "no call", thêm cây tổ chức); DESIGN.md §5 cập nhật.

## Evidence
- Hiện tại: bảng phẳng + chip phòng ban + `SidePanel` hồ sơ (`frontend/src/components/contacts/*`), mockup cũ §1 `design/mockups/contacts-documents-settings.html` đã bỏ nav kiểu Lark vì mục giả.
- Thành viên thường đọc được `GET /workspaces/:id/departments` (`backend/services/workspace/internal/domain/department.go:127` chỉ `authorizeMember`) → cây có sẵn, `parent_id` + `member_count`.
- `Contact` chỉ có **tên** phòng ban (`frontend/src/hooks/useContacts.ts:15`) → cần thêm `department_id` vào response contacts (backend nhỏ, không đổi NGAC model).
- DESIGN.md §5: "list panel chỉ ở Tin nhắn và Tài liệu" → redesign này cần sửa quy tắc đó.

## Đợt 2: Danh bạ bên ngoài (Zane, 2026-10-17)
- Tách thành feature riêng, có brainstorm + plan + NGAC spec riêng. Đợt 1 chỉ chừa vị trí mục "Bên ngoài" trong nav (mockup §8), **không ship mục rỗng**.
- Phải phủ cả hai loại:
  1. Người dùng Nexus ở tổ chức khác: mời kết nối, chat/gọi xuyên tổ chức, nhãn "Bên ngoài", quản trị bật/tắt.
  2. Người không dùng Nexus: thẻ liên hệ tự lưu (tên, công ty, email, SĐT, ghi chú), chỉ email/gọi điện thoại; có thể mời vào Nexus → thành loại 1.
- Hiện trạng: chưa có khái niệm external/guest ở đâu cả; channel gắn `workspace_id`; NGAC chỉ cho phép khi cùng PC → chat xuyên tổ chức cần thay đổi policy model (spec + test vector + deny case bắt buộc). `users.union_id` (migration 005) đã nhận diện một người qua nhiều tenant.

## Rủi ro / câu hỏi mở
- **Chốt (Zane, 2026-10-17):** người bị ẩn vẫn hiện tên + avatar ở tin nhắn, phê duyệt, tài liệu; bấm vào → thẻ nhỏ "Hồ sơ này không còn hiển thị với bạn" (không chức danh/phòng/email, không Nhắn tin/Gọi; trả lời trong cuộc trò chuyện đang có vẫn được). Hệ quả: `useContacts` đang vừa là danh bạ vừa là nguồn tên toàn app → phải tách "duyệt danh bạ" (lọc theo phạm vi) khỏi "tra tên người" (tên + avatar), nếu không tin nhắn của người bị ẩn hiện "Thành viên".
- "Nhóm của tôi" lấy từ kênh chat người dùng là thành viên: cần xác nhận khi plan rằng endpoint kênh đủ trả danh sách thành viên mà không gọi N lần.
- Workspace lớn (vài nghìn người): cột giữa dùng danh sách đã tải (`useContacts` tải hết), không thêm API phân trang mới.
