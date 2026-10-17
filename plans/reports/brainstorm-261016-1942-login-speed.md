---
type: brainstorm
date: 2026-10-16
slug: login-speed
status: accepted
---

# Brainstorm: đăng nhập và mở app chậm

Người dùng (2026-10-16): "login vào đang hơi chậm, load khá lâu". Chậm ở **cả bốn lúc**: chờ email OTP, nhập mã xong → vào
app, mở app khi đã đăng nhập (iPhone PWA + web), lần mở đầu sau bản mới. Muốn app **mở ngay từ dữ liệu lần trước** như
Google Chat rồi cập nhật nền.

## Bằng chứng

### Đo trên production (2026-10-16, `curl` từ máy dev)
| Tệp | Tải về | gzip -9 |
|---|---|---|
| `/assets/index-*.js` | **2 428 120 B, không nén** | 659 401 B |
| `/assets/livekit-client.esm-*.js` | 521 171 B, **modulepreload ngay lúc mở app** | — |
| `/assets/index-*.css` | 95 715 B, không nén | — |
- Không có `Content-Encoding` trong response; `deploy/nginx/default.conf` không bật `gzip`. `index.html` TTFB 53 ms.
- `Cache-Control: public, immutable` 30 ngày (đúng); mỗi deploy đổi hash → tải lại ~3 MB thô.
- `index.html` vẫn tải `fonts.googleapis.com/css2?family=Manrope…` (chặn render), app không dùng Manrope.

### Đọc code (agent Explore, chưa đo trên máy)
- Không chia bundle: router không `autoCodeSplitting`, không `manualChunks` (`frontend/vite.config.js:140,154`); tiptap,
  emoji-mart + dữ liệu, react-markdown, motion nạp ngay cả ở màn đăng nhập (`EmojiPicker.tsx:2` ← `ChatEditor.tsx:4`, `SpaceView.tsx:29`).
- Chuỗi chờ trước khi có shell: `POST /auth/refresh` (spinner toàn app, `__root.tsx:22-32`, `api/client.ts:89-97`) →
  `GET /workspaces` (spinner, `_workspace.tsx:38,67`) → có khi `POST /auth/switch-tenant`, và `resetTenantScopedState()`
  xoá cả cache kể cả khi tenant không đổi (`useSwitchWorkspace.ts:51`, `useTenantSync.ts:38-46`) → tải lại `/workspaces`.
- OTP: gửi email **đồng bộ trong request**, mở kết nối SMTP/TLS mới mỗi lần (`auth/domain/otp.go:101`, `pkg/mailer/mailer.go:168-190`; dial 10 s, send 30 s).
- Danh bạ tải tuần tự 50/trang, tới 100 trang (`useContacts.ts:58-66`); tên và avatar ở Trang chủ chờ nó.
- `GET /workspaces` quét mọi tenant rồi lọc (`workspace/internal/domain/service.go:257,273`) — rẻ hiện tại, tăng theo số tenant.
- OneSignal SDK nạp lúc khởi động (`main.tsx:35`, `lib/push.ts:195`).
- Đã loại trừ: refresh/`/me`/switch-tenant phía server rẻ (Redis, 1–2 query có index), danh sách kênh đã kiểm quyền theo lô,
  service worker `registerType: 'prompt'` không ép tải lại.

## Hợp đồng

**Outcome:** Mở app đã đăng nhập (PWA và web) thấy ngay màn làm việc từ dữ liệu lần trước, dữ liệu mới về nền. Lần đầu sau
bản mới và luồng nhập mã → Trang chủ nhanh rõ rệt. Bấm "Gửi mã" trả lời ngay, email đến không phụ thuộc thời gian request.

**Constraints:**
- Đo trước, đo sau, cùng cách đo: Lighthouse mobile (Moto G Power, 4G chậm) trên production + Safari Web Inspector trên
  iPhone PWA của người dùng; ghi số vào report. Không tối ưu chỗ chưa đo.
- Cache trên máy (IndexedDB qua TanStack Query persister): khoá theo **user + workspace**, chỉ dữ liệu đã có quyền đọc lúc
  lưu, `maxAge` 7 ngày, `buster` = phiên bản build; **xoá khi đăng xuất, đổi workspace, refresh token thất bại / 401**.
  Không lưu token, OTP, nội dung tệp. Mở từ cache vẫn kiểm quyền thật khi refetch; dữ liệu cũ hiện kèm trạng thái
  "Đang cập nhật" (theo DESIGN.md: thiết kế trạng thái trước khi code).
- OTP gửi nền: request trả ngay sau khi lưu mã; worker gửi với kết nối SMTP dùng lại, thử lại có giới hạn, log lỗi.
  Không lộ email có tồn tại hay không (giữ hành vi hiện tại). Giới hạn tần suất giữ nguyên.
- Không đổi mô hình NGAC, không đổi contract API công khai trừ khi ghi rõ.
- Thay đổi nginx đi qua `deploy/` + runbook `docs/deployment.md`; deploy vẫn qua cổng duyệt.

**Non-goals:**
- Không SSR, không đổi framework, không đổi cơ chế đăng nhập (OTP giữ nguyên), không offline đầy đủ (gửi tin khi mất mạng).
- Không tối ưu màn khác (Tài liệu, Phê duyệt) ngoài phần hưởng lợi chung từ chia bundle.
- Không tối ưu `GET /workspaces` quét toàn tenant (ghi nhận, để sau khi có số liệu).

**Acceptance criteria:**
1. Production trả JS/CSS nén (`Content-Encoding: gzip` hoặc `br`); JS tải về lúc mở màn đăng nhập ≤ 250 KB nén, màn
   Trang chủ ≤ 450 KB nén; livekit chỉ tải khi gọi.
2. Không còn Manrope; không stylesheet ngoài chặn render.
3. Mở app đã đăng nhập, có cache: Trang chủ (danh sách hội thoại) hiện trước khi request mạng đầu tiên xong — đo bằng
   Web Inspector, mục tiêu < 1 s trên iPhone PWA.
4. Không cache: từ lúc có token tới shell chỉ còn tối đa 2 request nối tiếp; không xoá cache khi tenant không đổi; không
   tải `/workspaces` hai lần.
5. `POST /api/auth/otp/request` p95 < 300 ms trên production (không tính thời gian SMTP); có test: mailer chậm/lỗi không làm
   request chậm/lỗi, email vẫn gửi (hoặc log lỗi + đếm).
6. Đăng xuất / đổi workspace / 401 xoá cache trên máy (test); người khác đăng nhập cùng máy không thấy dữ liệu người trước.
7. Lighthouse mobile production: LCP và TTI trước/sau ghi trong report; mọi cổng `make` + `npm` xanh.

## Hướng đã chọn

| # | Việc | Lúc chậm được sửa | Ước tính lợi |
|---|---|---|---|
| A | nginx bật `gzip` (+ `gzip_static` với file `.gz` build sẵn; `br` nếu module có) cho js/css/svg/json | mọi lúc tải lại, sau bản mới | 2,4 MB → ~0,66 MB |
| B | Bỏ Manrope; bỏ modulepreload livekit/call khỏi `index.html` (chỉ nạp khi gọi) | mọi lúc | −0,5 MB, bớt chặn render |
| C | Chia bundle: router `autoCodeSplitting`, lazy tiptap/emoji/markdown/motion-heavy, `manualChunks` cho vendor lớn | sau bản mới, nhập mã → app | màn đăng nhập nhỏ hẳn |
| D | Khởi động: không reset cache khi tenant không đổi, bỏ `/workspaces` lần hai, chạy refresh và tải chunk song song | nhập mã → app, mở app | −1–2 RTT |
| E | Persist TanStack Query cache (IndexedDB) theo user + workspace, hiện shell từ cache rồi refetch | mở app khi đã đăng nhập | màn hiện gần như tức thì |
| F | OTP: gửi email nền + kết nối SMTP dùng lại | chờ mã | request trả ngay |
| G | Danh bạ: tăng `limit` (vd 500) để bớt vòng tuần tự | nhập mã → app | ít RTT ở workspace lớn |

Đã cân nhắc:
- **OTP qua Redpanda queue** thay vì worker trong process: bền hơn khi service restart giữa chừng, nhưng thêm consumer +
  topic cho một email. Chọn worker trong process + log/đếm lỗi; người dùng có nút "Gửi lại mã". Đổi sang queue nếu log cho thấy mất email.
- **Service worker trả shell + API cache** thay cho persister: phức tạp, khó kiểm quyền. Persister của TanStack Query đủ và xoá được theo user.

Thứ tự: A + B (rẻ, lợi lớn nhất, deploy riêng được ngay) → F → D → C → E → G. Đo lại sau mỗi bước.

## Bổ sung (người dùng, 2026-10-16): tối ưu backend, caching, học kiến trúc Telegram

### Bằng chứng backend (log dev `.dev-logs/*.log`, ~3 000 request, dữ liệu nhỏ)
- Endpoint khởi động đều nhanh: `GET /channels/unread` p95 17 ms, `/channels/activity` 15 ms, `/threads/unread` 15 ms,
  `/workspaces/:id/contacts` 11 ms, `/me` 10 ms, `/chat-sections` 9 ms, `/me/workspaces` 7 ms. `POST /auth/otp/verify` có lần 1 166 ms.
- Vậy chi phí nằm ở **số request và số vòng**, cộng với **tải lại toàn bộ mỗi lần mở / kết nối lại**:
  spec `realtime-event-delivery` (dòng 79–90, 186): sự kiện chỉ là gợi ý tải lại; workspace có số thứ tự (`INCR` Redis),
  sự kiện kênh và người dùng mang số 0 (ngoài luồng); lỗ hổng hay kết nối lại → refetch mọi nhóm query.
- Chưa có số production (VPS dùng chung, dữ liệu thật lớn hơn). Bước đầu của plan: đo p50/p95 từ access log production.

### Phát hiện lớn nhất: đợt chậm 8 s trên production (đo 2026-10-16 qua SSH chỉ đọc)
- Log 72 h của auth/workspace/messaging/drive/approval (848 request): p50 5–30 ms, nhưng **cả loạt request cùng mất
  8,0–8,5 s theo đợt** (23:29, 01:34, 01:46, 12:38 …), `GET /channels/:id/members` 16,5 s (= 2 lời gọi nối tiếp).
  Đợt rơi vào lần mở app sau một khoảng nghỉ — đúng cảm giác "login / mở app load lâu".
- Chỉ endpoint **có gọi policy** bị chậm (`/workspaces`, `/dms`, `/channels/*`, `/approval/pending`, `/drive/.../download`);
  endpoint không gọi policy không bao giờ chậm (`/me` max 62 ms, `/notifications/unread-count` 2–4 ms).
- Policy **không chậm**: histogram `ngac_check_access_duration_seconds` 299/299 quyết định ≤ 10 ms; log policy không có một
  `grpc call failed` nào, trong khi messaging ghi `policy check failed; denying … DeadlineExceeded` lúc 12:38:36 → treo ở
  phía client trước khi tới policy.
- `grpcauth.Dial` (`backend/pkg/grpcauth/interceptors.go:81`) dùng `grpc.NewClient`: resolver `dns`, kênh vào IDLE sau 30 phút
  không gọi, thức dậy thì phân giải lại; không keepalive, không tắt tra TXT service config.
- DNS trong container: `nameserver 127.0.0.11`, `search tail5d68a5.ts.net`, upstream Tailscale MagicDNS (`100.100.100.100` +
  IPv6 `fd7a:…`). `nslookup policy` / `redis` trong `nexus-messaging-1`: **4,10 s mỗi lần, lặp ổn định** (đi qua
  `policy.tail5d68a5.ts.net` → NXDOMAIN từ Tailscale sau ~4 s). Hai lần = ~8,2 s.
- ~~DNS qua Tailscale~~ — **đã loại (ak:debug, cùng ngày)**: từng truy vấn mà Go/gRPC phát ra (`A`/`AAAA policy.`,
  `TXT _grpc_config.policy.` và bản có hậu tố search) đều 0,10 s = chi phí `docker exec`; `wget http://policy:9090` 0,10 s
  như dùng IP. 4,1 s lúc đầu là cách busybox `nslookup` không `-type` đi danh sách search, không phải đường của Go.
  Conntrack cũng loại: `nf_conntrack_tcp_timeout_established = 432000`, không có `br_netfilter`.

### Chẩn đoán (ak:debug 2026-10-16) — kênh gRPC tới policy thoát IDLE chậm ~8 s
- **Kích hoạt đã chứng minh:** một lời gọi policy chậm ~8 s **khi và chỉ khi** tiến trình gọi đã không gọi policy > 30 phút
  (mặc định idle của `grpc.NewClient` trong `grpcauth.Dial`, `backend/pkg/grpcauth/interceptors.go:81-87`).
  72 h log workspace/messaging/approval: idle > 30 phút → chậm **13/13**, nhanh **0**; 27 request "đang bận mà chậm" đều là
  request song song chờ cùng kênh trong đợt đó. Mỗi tiến trình bị riêng: 12:38 workspace 22,17→30,64 rồi messaging +
  approval + drive 30,9→38,98; policy tạo kết nối Redis lúc 12:38:38 = lúc request mới tới policy.
- **Đã loại:** policy chậm (histogram ≤ 10 ms, không log lỗi), DNS (trên), conntrack (trên), DB phía client
  (`/notifications/unread-count` cùng service, cùng lúc: 4 ms).
- **Chưa xác định:** bước nào *bên trong* lúc thoát IDLE tốn 8 s (dựng resolver/tra TXT service config qua resolver Go,
  hay kết nối). Không quan sát được nếu không có log trong tiến trình.
- **Hướng sửa gắn với nguyên nhân:** `grpcauth.Dial` thêm `grpc.WithIdleTimeout(0)` (kênh không vào IDLE, giữ kết nối) +
  ghi log chuyển trạng thái kênh (WARN khi CONNECTING/IDLE→READY > 1 s, kèm thời lượng) để lần kết nối lại sau này (vd policy
  restart) lộ ra nếu vẫn tốn 8 s. Không thêm keepalive ping trong lần này: server grpc mặc định `MinTime` 5 phút +
  `PermitWithoutStream=false` → client ping dày bị GOAWAY `too_many_pings`; nếu cần thì phải sửa cả `ServerOptions`.
- Kiểm sau deploy: cùng phép đếm 72 h — idle > 30 phút & chậm phải về 0; log chuyển trạng thái không có WARN.

### Telegram làm gì, áp vào đây thế nào
| Cơ chế Telegram | Ý nghĩa | Áp vào Nexus |
|---|---|---|
| Client giữ CSDL cục bộ, mở app hiện ngay từ đó | không chờ mạng để vẽ | E (persist cache, đã chọn) |
| `updates.getState` + `getDifference(pts)`: server giữ nhật ký thay đổi theo người dùng, client hỏi "từ số X đã có gì đổi" | mở lại / kết nối lại chỉ tải phần thay đổi | **H**: nhật ký sự kiện có số thứ tự, phát lại khi đăng ký với `since` |
| Tham số `hash` trong `getDialogs`, `getContacts`…: server trả "không đổi" nếu hash khớp | danh sách lớn không gửi lại | **I**: ETag / `If-None-Match` → 304 cho danh sách |
| Một lần lấy trạng thái hội thoại (dialogs + unread + top message) | ít vòng | **J**: một endpoint trạng thái chat thay 6 request |
| Ảnh thu nhỏ "stripped" vài trăm byte gửi kèm metadata | khung ảnh hiện ngay, không chờ | **K**: placeholder nhỏ (vd blurhash) kèm metadata đính kèm/thumbnail (nối với plan Drive) |
| Server cache nóng trong bộ nhớ, CDN cho media | đọc rẻ | **L**: cache Redis có phiên bản cho dữ liệu đọc nhiều (danh bạ workspace, unread) |

### Hướng backend
- **H — Đồng bộ phần thay đổi (getDifference):** hub ghi mỗi sự kiện đã cấp số vào Redis Stream theo workspace
  (`XADD … MAXLEN ~ 10000`, giữ ~24 h). Đăng ký kèm `since=<seq>`: còn trong cửa sổ → phát lại các sự kiện thiếu (vẫn lọc
  tenant + audience + quyền như hiện tại), client chỉ làm cũ các query bị chạm; quá cửa sổ → "quá xa", tải lại như hiện nay.
  Mở rộng số thứ tự cho sự kiện mức người dùng (luồng riêng theo user, như `pts` của Telegram) để unread, mention, DM không
  nằm ngoài luồng. Sự kiện vẫn chỉ là gợi ý + id, không chứa dữ liệu — giữ nguyên ranh giới quyền.
- **I — Kiểm tra lại rẻ (ETag/304):** danh sách kênh, chat-sections, danh bạ, workspaces, `/me` trả `ETag` (phiên bản lưu
  trong Redis, tăng khi có sự kiện đổi tương ứng, hoặc băm nội dung). Client gửi `If-None-Match` → 304, không truy vấn DB.
- **J — Một request trạng thái chat:** `GET /api/workspaces/:id/chat-state` (messaging) trả kênh + DM + unread + activity +
  sections + số mention trong một lần, kiểm quyền theo lô như hiện nay. Giữ các endpoint cũ cho chỗ khác dùng.
- **L — Cache Redis có phiên bản:** danh bạ workspace (thường đọc, ít đổi) và unread theo user; khoá kèm phiên bản, xoá bằng
  sự kiện đã có. Quyết định quyền vẫn đi `policyclient` (đã có cache + invalidation qua EPP), không cache quyền ở service.
- **F (đã có) — OTP nền**, kèm điều tra `otp/verify` 1,2 s (đo từng bước: Redis, DB, ký token, switch tenant).
- Không đổi mô hình NGAC. Mọi cache phía server khoá theo tenant; trả từ cache vẫn phải đúng audience như truy vấn gốc.

### Thứ tự đề xuất (cập nhật)
0. Đo: access log production (p50/p95 theo route, số request lúc khởi động), Lighthouse + Web Inspector — làm mốc.
0b. **Đợt chậm 8 s sau idle (DNS / kênh gRPC)** — chứng minh rồi sửa trước tiên (ak:debug → ak:fix), deploy riêng.
1. A + B (nén, bỏ Manrope, bỏ preload livekit) — deploy riêng ngay.
2. F (OTP nền) + D (bớt vòng khởi động).
3. J + I (một request trạng thái, ETag/304).
4. C (chia bundle) + E (cache trên máy).
5. H (phát lại sự kiện theo `since`, luồng theo user) — lớn nhất, làm khi 0–4 đã đo lại.
6. L, G, K theo số đo.

## Rủi ro
- Phát lại sự kiện (H) phải lọc quyền tại thời điểm phát lại, không phải lúc ghi: người bị gỡ khỏi kênh giữa chừng không được
  nhận sự kiện cũ của kênh đó. Test deny bắt buộc.
- ETag theo phiên bản: quên tăng phiên bản ở một đường ghi → client giữ dữ liệu cũ. Gắn tăng phiên bản vào đúng chỗ phát sự kiện.
- Cache trên máy chứa nội dung doanh nghiệp: máy dùng chung có thể lộ nếu người trước không đăng xuất. Giảm bằng `maxAge`,
  xoá khi 401, khoá theo user; ghi rõ trong spec. Nếu cần chặt hơn (vd chỉ lưu danh sách hội thoại, không lưu nội dung tin) → hỏi.
- Chia bundle có thể làm chậm lần mở màn chưa nạp (Tài liệu, Phê duyệt) → prefetch chunk khi rảnh sau khi shell hiện.
- Gửi email nền mất email khi container restart giữa lúc gửi → chấp nhận, có "Gửi lại mã".

## Capability trong `docs/specs/`
- Sửa: `sign-in-screens` (trạng thái "Đang cập nhật"), `tenant-auth-flow` (OTP gửi nền, khởi động), `pwa-install` (cache trên máy).

## Quyết định của người dùng (2026-10-16)
- Cache trên máy **lưu cả nội dung tin**: danh sách hội thoại, danh bạ, trang tin mới nhất của 20 hội thoại gần nhất; xoá khi
  đăng xuất / đổi workspace / 401; tối đa 7 ngày.
- Đồng bộ phần thay đổi (H) **gồm cả luồng theo người dùng** (như `pts` của Telegram), không chỉ workspace.
- Cho phép **đọc access log production qua SSH, chỉ đọc**, chỉ lấy số liệu theo route.

## Câu hỏi còn mở
- Không còn.
