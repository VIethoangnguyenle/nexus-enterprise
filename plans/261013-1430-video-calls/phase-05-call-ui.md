---
phase: 5
title: "Frontend lõi cuộc gọi: provider, kiểm tra trước, stage, điện thoại"
status: completed
priority: P1
effort: "3d"
dependencies: [2, 3]
---

# Phase 05: UI lõi cuộc gọi

## Overview
Dựng theo DESIGN.md §6 "Cuộc gọi video và thoại" và mockup §2, §4–§7. Dùng `livekit-client` + **hooks** của
`@livekit/components-react` (không dùng component có style, không `@livekit/components-styles`).

## Requirements
- Deps pin chính xác (`npm view` trước): `livekit-client`, `@livekit/components-react`. Token stage vào
  `frontend/src/index.css` (cùng giá trị tokens.css, không đổi theo theme) + lớp Tailwind tương ứng.
- **Trạng thái:** `stores/call.store.ts` (Zustand: cuộc gọi hiện tại, minimized, panel, lựa chọn thiết bị) — client
  state; dữ liệu cuộc gọi qua TanStack Query (`hooks/useCalls.ts`, keys trong `hooks/keys/`).
- **CallProvider** gắn trong `routes/_workspace.tsx`: giữ `Room` khi đổi màn; thu nhỏ = ô nổi (điện thoại) / PiP trình
  duyệt khi rời tab (máy tính, `documentPictureInPicture` nếu có, không thì `requestPictureInPicture` video người nói).
- **Route** `routes/_workspace/calls.$callId.tsx`: kiểm tra trước (nhóm) → stage; `?answer=video|audio` vào thẳng.
- **Kiểm tra trước:** xem trước camera, đo micro (AnalyserNode), Select thiết bị (`Room.getLocalDevices`, nhãn hệ điều
  hành), Thử loa, banner bị chặn quyền, đầy 20, nhớ lựa chọn (localStorage theo user, try/catch).
- **Stage máy tính:** bố cục tự động (1/2–4/5–9/10–20 + phân trang), Người đang nói / Lưới, ghim; ô người (video
  `VideoTrack`, avatar, tên, micro/sóng, mạng yếu theo `ConnectionQuality`, giơ tay); thanh điều khiển + phím tắt
  (Ctrl+D, Ctrl+E, Ctrl+Alt+H, Space giữ để nói); trình bày (`setScreenShareEnabled`, một người mỗi lúc, hỏi thay);
  panel Chat (tái dùng luồng của cuộc trò chuyện ở chế độ gọn + ô soạn) và Người tham gia (giơ tay theo thứ tự, trong
  cuộc gọi, chưa vào + Mời).
- **Giơ tay + cảm xúc:** `publishData` topic `nexus.hand` / `nexus.reaction` (JSON nhỏ, reliable cho tay, lossy cho cảm
  xúc), chỉ trong phiên; trạng thái tay cũng ghi vào participant attributes để người vào sau thấy.
- **Điện thoại:** 1–1 toàn màn + ô của mình kéo 4 góc; nhóm lưới 2 cột theo trang (vuốt); điều khiển tròn 56; sheet Thêm;
  tự ẩn điều khiển 4s; Đổi camera (`switchActiveDevice` facingMode); ẩn Trình bày; "Chạm để bật âm thanh"
  (`room.startAudio()` khi `AudioPlaybackStatusChanged`).
- **Trạng thái:** đang nối lại (`RoomEvent.Reconnecting`), không kết nối được + Thử lại, kết thúc + tổng kết, đang ở cuộc
  gọi khác ("Rời và vào"), bị đá vì mất quyền (`Disconnected` lý do `PARTICIPANT_REMOVED`) → banner.
- Không ID trên UI; mọi chữ tiếng Việt như mockup; giảm chuyển động theo §7.

## Related Code Files
- Create: `frontend/src/stores/call.store.ts`, `hooks/useCalls.ts`, `api/calls.ts`, `components/calls/` (`CallProvider`,
  `PreJoin`, `CallStage`, `ParticipantTile`, `CallControls`, `CallSidePanel`, `ReactionsLayer`, `PhoneCallView`,
  `MiniCall`, `CallStates`) + test mỗi file; `routes/_workspace/calls.$callId.tsx` (+ test)
- Modify: `frontend/package.json` (+ lock), `src/index.css` (token stage), `routes/_workspace.tsx`

## Implementation Steps
1. Test trước với `livekit-client` mock (Room giả phát sự kiện): bố cục theo số người, trạng thái micro/camera, phím tắt,
   giơ tay/cảm xúc nhận từ data, đá → banner, nối lại.
2. Dựng component; nối API vé (phase 03); chạy thật với LiveKit dev 2 tab.
3. Gate: `npm test`, lint, typecheck, build; ảnh Playwright (Chromium fake media `--use-fake-device-for-media-stream`)
   máy tính + 390px so mockup.

## Success Criteria
- [x] 2 tab trình duyệt gọi được nhau qua LiveKit dev, trình bày màn hình, giơ tay, cảm xúc, chat panel.
- [x] Thu nhỏ rồi đi màn khác không rớt cuộc gọi.
- [x] Gate frontend xanh; ảnh khớp mockup.

## Risk Assessment
- Bundle lớn (`livekit-client` ~ vài trăm KB) → lazy-load route cuộc gọi + provider chỉ nạp SDK khi có cuộc gọi.
