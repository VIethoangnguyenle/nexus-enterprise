# Video call load test — 2026-10-13

## Setup (local, not the VPS)
- `livekit/livekit-server:v1.13.7` in Docker (dev compose, mem limit 512 MB), host 16 cores / 22 GB.
- `lk` v2.18.8 `load-test --video-publishers 10 --subscribers 10 --duration 60s --video-resolution medium`
  (20 participants, the room's limit; a real 20-person call has everyone publishing *and* subscribing, which this
  approximates as half and half).
- Stats sampled every 4 s with `docker stats` after a 15 s ramp.

## Results
| Metric | Value |
|---|---|
| LiveKit CPU | ~21–22 % of one core, steady |
| LiveKit memory | 96 → 110 MiB over 60 s |
| Inbound (publishers → SFU) | ~1.0 MB/s ≈ 8 Mbps |
| Outbound (SFU → subscribers) | ~2.4 MB/s ≈ 19 Mbps |
| Per subscriber | 1.2–1.7 Mbps, 0 % packet loss |
| Tracks subscribed | 60 / 100 (each subscriber held 6 of the 10 video tracks) |

## Reading
- Per received video stream at "medium" with simulcast: ~150–250 kbps (LiveKit picks the layer).
- Extrapolated worst case, 20 people all camera-on, each receiving 19 streams: ~20 × 19 × 0.2 Mbps ≈ **76 Mbps out**,
  ~20 × 0.8 Mbps ≈ **16 Mbps in**; CPU scales roughly with forwarded packets → about 1–1.5 cores.
- The 60/100 tracks: the load tester's subscribers did not all subscribe to every publisher within the run; no
  errors or loss were reported. Not investigated further locally.

## Still to do
- Repeat on the VPS after deploy (`ufw`/provider firewall, real public IP, `use_external_ip`) and record CPU,
  RAM and NIC throughput there. If the VPS uplink cannot carry ~80 Mbps, cap cameras on at once (e.g. 9) or move to
  LiveKit Cloud (change `LIVEKIT_URL` + keys, no code change).
