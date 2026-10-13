---
phase: 2
title: "Hạ tầng LiveKit: dev, prod, TURN, proxy"
status: completed
priority: P1
effort: "1-1.5d"
dependencies: [1]
---

# Phase 02: Hạ tầng LiveKit

## Overview
Chạy `livekit-server` ở dev và prod. Tín hiệu (WSS) đi cùng origin qua `/livekit`; media qua 2 cổng host; TURN/TLS
trên 443 qua Traefik cho mạng chặn UDP. Webhook LiveKit → `messaging` trên mạng nội bộ.

## Requirements
- **Cấu hình** `deploy/livekit/livekit.yaml` (một file, giá trị theo env): `port: 7880`; `rtc.tcp_port: 7881`;
  `rtc.udp_port: 7882` (UDP mux, một cổng — thay dải 50000–60000); `rtc.use_external_ip: true` ở prod (dev: `node_ip`
  127.0.0.1); `room.max_participants: 20`, `room.empty_timeout: 30`; `keys` từ `LIVEKIT_KEYS`/env;
  `webhook.api_key` + `webhook.urls: [http://nexus-messaging:8183/internal/livekit/webhook]` (dev:
  `http://host.docker.internal:8183/...` hoặc `messaging` service); `turn.enabled: true`, `turn.domain:
  turn.nexus.zaneng.xyz`, `turn.tls_port: 5349`, `turn.external_tls: true` (Traefik giải TLS, LiveKit nhận thô trên
  5349 nhưng quảng bá 443), `turn.udp_port: 3478` chỉ ở dev.
- **Dev:** service `livekit` trong `docker-compose.yml` (image pin chính xác, cổng 7880/7881/7882-udp ra localhost —
  dev được phép); `make dev-infra` khởi động nó; `.env.dev.example` thêm `LIVEKIT_API_KEY=devkey`,
  `LIVEKIT_API_SECRET=<ngẫu nhiên ≥ 32 ký tự, không commit giá trị thật>`, `LIVEKIT_URL=http://localhost:7880`,
  `LIVEKIT_PUBLIC_URL=ws://localhost:5173/livekit`. Vite proxy `'/livekit': { target: 'http://localhost:7880', ws: true,
  rewrite: bỏ tiền tố }` — đặt trước các mục `/api`. Ghi chú macOS Docker Desktop: UDP mux cổng 7882 map được; nếu
  không kết nối được thì chạy binary `livekit-server --dev` ngoài Docker.
- **Prod** `deploy/docker-compose.prod.yml`: `nexus-livekit` trên mạng `nexus` + `traefik`; **ports** `7881:7881/tcp`,
  `7882:7882/udp` (ngoại lệ duy nhất); labels Traefik **TCP router** `HostSNI(turn.nexus.zaneng.xyz)`, entryPoint
  `websecure`, `tls.certresolver=le` → service port 5349. `deploy/check-compose-env.sh`: luật "không service nào publish
  ports" đổi thành "chỉ `livekit` được, và đúng 2 cổng trên" (test bằng compose mẫu); env bắt buộc của livekit và
  messaging (`LIVEKIT_URL=http://nexus-livekit:7880`, `LIVEKIT_PUBLIC_URL=wss://nexus.zaneng.xyz/livekit`,
  `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`) vào `deploy/env.example` (giá trị giả).
- **nginx** `deploy/nginx/default.conf`: `location ^~ /livekit/` → `nexus-livekit:7880` (WebSocket upgrade như
  `/api/ws`, timeout dài), đặt trước `location /`. Không route công khai tới webhook.
- **CI build:** không build image LiveKit (dùng image chính thức, pin tag). Deploy pull nó như các image khác.
- **docs/deployment.md:** mục "Cuộc gọi (LiveKit)": sơ đồ, ngoại lệ cổng và lý do, bước một lần trên server do người
  dùng làm (`ufw allow 7881/tcp`, `ufw allow 7882/udp`, bản ghi DNS `turn.nexus.zaneng.xyz`), biến env, cách kiểm
  (`curl https://nexus.zaneng.xyz/livekit/` trả 200 "OK"), cách đổi sang LiveKit Cloud.

## Related Code Files
- Create: `deploy/livekit/livekit.yaml`
- Modify: `docker-compose.yml`, `Makefile` (dev-infra), `frontend/vite.config.js`, `deploy/docker-compose.prod.yml`,
  `deploy/check-compose-env.sh`, `deploy/env.example`, `deploy/nginx/default.conf`, `docs/deployment.md`,
  `.env.dev.example` (hoặc file mẫu env dev hiện có)

## Implementation Steps
1. Dev: thêm service, chạy `make dev-infra`, kiểm `curl localhost:7880` = "OK"; `lk` CLI (nếu có) `lk room list`.
2. Vite proxy `/livekit` + thử kết nối WS qua 5173.
3. Prod compose + nginx + check script (chạy `deploy/check-compose-env.sh` trong CI cục bộ: `docker compose config`).
4. deployment.md.

## Success Criteria
- [ ] Dev: LiveKit chạy, webhook tới được messaging (log nhận), WS qua `/livekit` của Vite.
- [ ] `deploy/check-compose-env.sh` xanh với ngoại lệ đúng 2 cổng; đỏ nếu service khác publish cổng (thử).
- [ ] nginx `nginx -t` hợp lệ (container nginx tạm).

## Risk Assessment
- Traefik dùng chung không bật provider docker cho TCP hoặc thiếu entryPoint 443 TCP → phương án: `turn.tls_port: 443`
  không được (443 của Traefik) → dùng TURN UDP 3478 + TCP 7881 và ghi giới hạn "mạng chỉ mở 443 không gọi được".
  Quyết định lại với người dùng nếu xảy ra.
- `use_external_ip` lấy sai IP sau NAT → đặt `rtc.node_ip` bằng IP VPS (160.187.146.173) qua env.
