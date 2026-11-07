# Kết quả đo — ngac_bench (2026-11-07)

Dữ liệu: 2.000 tenant, 50.000.000 tin (1 triệu ở channel toàn công ty `c1_0`), 20.191 channel; người xem `u1_1`…`u1_1000`
đọc 201 channel, `u1_1`…`u1_100` ở mức `all` cho `c1_0`. Postgres 16.14 trong Docker local, `shared_buffers` 128 MB.
Chi tiết plan và EXPLAIN: `baseline-run.md` (trước 072), `after-run.md` (sau 072/073). Script: `make bench-messages`.

## Đếm chưa đọc (`GetUnreadCounts`, 201 channel)

| Đo | Trước | Sau | |
|---|---|---|---|
| 1 client, người dùng mức thường: p50 / p95 | 236 / 308 ms | **6,2 / 12,1 ms** | đạt < 20 ms p95 |
| 1 client, mọi người dùng: p50 / p95 | 238 / 414 ms | 6,6 / 156 ms | đuôi = mức `all`, 50k reply chưa đọc |
| 50 client, người dùng mức thường: lượt/giây | — | 926 | |
| 50 client, mọi người dùng: lượt/giây | 24 | 440 | ×18 |

Hai thay đổi đem lại kết quả: tin chính tính bằng phép trừ, và mention đi từ index GIN `mentions` (CTE
`MATERIALIZED`) thay cho `BitmapAnd` với index channel (1,48 triệu dòng mỗi lần).

## Các truy vấn khác (50 client)

| Truy vấn | Trước p95 | Sau p95 | Ghi chú |
|---|---|---|---|
| Trang lịch sử 50 tin | 45,7 ms | 28,8 ms | dùng `messages_channel_seq` |
| Xem trước 201 channel | 37,5 ms | 29,7 ms | không đổi truy vấn (nhiễu đo) |
| Tìm từ hiếm | 875 ms | 1.017 ms | ngoài phạm vi (hướng B) |
| Tìm từ phổ biến | 49,6 s | 44,5 s | ngoài phạm vi (hướng B) |

## Hub (Go benchmark, một instance)

Phát 1 tin tới 1.000 phiên đang mở: ~125 µs, 4 lần cấp phát. Không phải nút thắt.

## Migration trên 50 triệu dòng

072: 9.812 s (2 giờ 44 phút), giữ khoá độc quyền `messages` suốt thời gian đó; 073: 290 s; VACUUM: 770 s. Mỗi
`UPDATE` ghi lại cả dòng, kéo theo mọi index và cột sinh `search_vector`. Prod hiện nhỏ nên chạy trong vài giây;
một bảng lớn cần backfill theo lô ngoài transaction lớn.

## Acceptance của plan

| # | Tiêu chí | Kết quả |
|---|---|---|
| 1 | Đếm < 20 ms p95, không đổi khi 5M → 50M tin | Đạt 12 ms p95 ở 50M cho mức thường. Lần đo 5M chưa chạy (seed lại mất nhiều giờ); chi phí mỗi channel chỉ gồm tra theo khoá và đếm tin đã xoá, không phụ thuộc số tin |
| 2 | 0 request đếm lại khi tin tới channel đang mở; 1 publish / tin | Đạt (test Go + Vitest) |
| 3 | Badge < 1 s, e2e hai người | Chưa chạy e2e (cần cả stack); có test đơn vị cho cả hai phía |
| 4 | Đếm chính xác ở mọi ca | Đạt (test store, migration test) |
| 5 | Trang lịch sử < 10 ms, không mất/trùng ở ranh giới | Một lần gọi 2,9 ms; test ranh giới cùng thời điểm đạt |
| 6 | Người bị gỡ không nhận sự kiện | Đạt (test deny, kể cả đọc danh sách đang dở lúc gỡ) |
| 7 | make test, check-layering, check-ngac, npm test/typecheck/build | Đạt, trừ `AccountSettings.wide.test.tsx` (phụ thuộc môi trường push, không thuộc thay đổi này) |
