# EXPLAIN ANALYZE — conversation activity queries (2026-10-13)

Database: dev Postgres (`nexus-enterprise-postgres-1`, port 5433), 401 messages, 115 channels, 94 users.
Unit under test: the per-channel newest-message lookups behind `GET /api/channels/activity`
(`store.LatestActivity`). One for the main stream (`parent_message_id IS NULL`), one for thread replies.

## Verdict

- The spec's `DISTINCT ON (channel_id)` form does **not** scale: on the existing indexes it reads every message of
  every channel (Bitmap Heap Scan on `idx_messages_channel`) and sorts them — **3.07 s** for 300k messages in 115
  channels, **0.52 s** for 50k replies. Adding the partial index does not change that plan: Postgres has no skip scan,
  so `DISTINCT ON` still reads all rows (section C, 3.05 s).
- Shipped: still one statement for all channels (no N+1), but `unnest(channel_ids) CROSS JOIN LATERAL (... ORDER BY
  created_at DESC, id DESC LIMIT 1)` + two partial indexes (`data/migrations/050_messages_latest_per_channel.sql`:
  `idx_messages_latest_main`, `idx_messages_latest_reply`). One ordered index probe per channel: **37 ms** cold for
  300k messages / 115 channels per query (D, E), 2.2 ms / 7.0 ms on the real dev data (F, G).
- `idx_messages_channel_time` alone is not enough for the LATERAL form: the `parent_message_id`/`deleted_at` filter would
  walk past replies and deleted rows; hence the partial indexes.
- Migration applied with `make db-migrate` after `pg_dump -Fc` backup; both indexes present
  (`select indexname from pg_indexes where indexname like 'idx_messages_latest%'`). Plain `CREATE INDEX` takes a brief
  write lock on `messages` — fine at current size; note for production deploy.

## A. DISTINCT ON, dev data as is (seq scan, tiny table — planner correctly ignores indexes at 401 rows)
```
                                                         QUERY PLAN                                                          
-----------------------------------------------------------------------------------------------------------------------------
 Unique  (cost=47.08..47.82 rows=16 width=200) (actual time=3.379..4.269 rows=16 loops=1)
   Buffers: shared hit=35
   InitPlan 1 (returns $0)
     ->  Seq Scan on channels  (cost=0.00..7.15 rows=115 width=36) (actual time=0.004..0.184 rows=115 loops=1)
           Buffers: shared hit=6
   ->  Sort  (cost=39.93..40.30 rows=149 width=200) (actual time=3.376..3.814 rows=314 loops=1)
         Sort Key: m.channel_id, m.created_at DESC, m.id DESC
         Sort Method: quicksort  Memory: 93kB
         Buffers: shared hit=35
         ->  Hash Left Join  (cost=5.12..34.55 rows=149 width=200) (actual time=0.696..2.598 rows=314 loops=1)
               Hash Cond: (m.sender_id = u.id)
               Buffers: shared hit=29
               ->  Seq Scan on messages m  (cost=0.00..29.02 rows=149 width=168) (actual time=0.377..1.305 rows=314 loops=1)
                     Filter: ((parent_message_id IS NULL) AND (deleted_at IS NULL) AND (channel_id = ANY ($0)))
                     Rows Removed by Filter: 87
                     Buffers: shared hit=26
               ->  Hash  (cost=3.94..3.94 rows=94 width=63) (actual time=0.301..0.305 rows=94 loops=1)
                     Buckets: 1024  Batches: 1  Memory Usage: 17kB
                     Buffers: shared hit=3
                     ->  Seq Scan on users u  (cost=0.00..3.94 rows=94 width=63) (actual time=0.006..0.147 rows=94 loops=1)
                           Buffers: shared hit=3
 Planning:
   Buffers: shared hit=519
 Planning Time: 3.437 ms
 Execution Time: 4.391 ms
(25 rows)
```

## B. DISTINCT ON, synthetic load (inserted inside a transaction that was rolled back: 300000 messages + 50000 replies)
```
--- main stream
                                                                         QUERY PLAN                                                                         
------------------------------------------------------------------------------------------------------------------------------------------------------------
 Unique  (cost=15411.22..15537.01 rows=115 width=152) (actual time=2159.782..3062.326 rows=115 loops=1)
   Buffers: shared hit=9546 read=1630 written=903, temp read=5860 written=5874
   InitPlan 1 (returns $0)
     ->  Seq Scan on channels  (cost=0.00..7.15 rows=115 width=36) (actual time=0.007..0.189 rows=115 loops=1)
           Buffers: shared hit=6
   ->  Sort  (cost=15404.07..15466.96 rows=25157 width=152) (actual time=2159.776..2637.616 rows=300314 loops=1)
         Sort Key: m.channel_id, m.created_at DESC, m.id DESC
         Sort Method: external merge  Disk: 46880kB
         Buffers: shared hit=9546 read=1630 written=903, temp read=5860 written=5874
         ->  Hash Left Join  (cost=351.38..11671.76 rows=25157 width=152) (actual time=13.123..1374.027 rows=300314 loops=1)
               Hash Cond: (m.sender_id = u.id)
               Buffers: shared hit=9546 read=1630 written=903
               ->  Bitmap Heap Scan on messages m  (cost=346.26..11597.63 rows=25157 width=120) (actual time=12.758..507.779 rows=300314 loops=1)
                     Recheck Cond: (channel_id = ANY ($0))
                     Filter: ((parent_message_id IS NULL) AND (deleted_at IS NULL))
                     Rows Removed by Filter: 50087
                     Heap Blocks: exact=10592
                     Buffers: shared hit=9545 read=1628 written=901
                     ->  Bitmap Index Scan on idx_messages_channel  (cost=0.00..339.98 rows=29305 width=0) (actual time=11.493..11.495 rows=350401 loops=1)
                           Index Cond: (channel_id = ANY ($0))
                           Buffers: shared hit=351 read=230 written=106
               ->  Hash  (cost=3.94..3.94 rows=94 width=63) (actual time=0.346..0.351 rows=94 loops=1)
                     Buckets: 1024  Batches: 1  Memory Usage: 17kB
                     Buffers: shared hit=1 read=2 written=2
                     ->  Seq Scan on users u  (cost=0.00..3.94 rows=94 width=63) (actual time=0.013..0.187 rows=94 loops=1)
                           Buffers: shared hit=1 read=2 written=2
 Planning:
   Buffers: shared hit=92 read=16
 Planning Time: 2.769 ms
 Execution Time: 3069.226 ms
(30 rows)

--- thread replies
                                                                             QUERY PLAN                                                                             
--------------------------------------------------------------------------------------------------------------------------------------------------------------------
 Unique  (cost=10385.13..10405.87 rows=115 width=152) (actual time=372.972..522.677 rows=115 loops=1)
   Buffers: shared hit=2336 read=206, temp read=993 written=995
   InitPlan 1 (returns $0)
     ->  Seq Scan on channels  (cost=0.00..7.15 rows=115 width=36) (actual time=0.006..0.191 rows=115 loops=1)
           Buffers: shared hit=6
   ->  Sort  (cost=10377.98..10388.35 rows=4148 width=152) (actual time=372.967..451.522 rows=50087 loops=1)
         Sort Key: m.channel_id, m.created_at DESC, m.id DESC
         Sort Method: external merge  Disk: 7944kB
         Buffers: shared hit=2336 read=206, temp read=993 written=995
         ->  Hash Left Join  (cost=2103.79..10128.72 rows=4148 width=152) (actual time=13.295..238.807 rows=50087 loops=1)
               Hash Cond: (m.sender_id = u.id)
               Buffers: shared hit=2336 read=206
               ->  Bitmap Heap Scan on messages m  (cost=2098.68..10112.23 rows=4148 width=120) (actual time=12.962..93.808 rows=50087 loops=1)
                     Recheck Cond: ((channel_id = ANY ($0)) AND (parent_message_id IS NOT NULL))
                     Filter: (deleted_at IS NULL)
                     Heap Blocks: exact=1584
                     Buffers: shared hit=2333 read=206
                     ->  BitmapAnd  (cost=2098.68..2098.68 rows=4148 width=0) (actual time=12.789..12.799 rows=0 loops=1)
                           Buffers: shared hit=749 read=206
                           ->  Bitmap Index Scan on idx_messages_channel  (cost=0.00..339.98 rows=29305 width=0) (actual time=10.483..10.484 rows=350401 loops=1)
                                 Index Cond: (channel_id = ANY ($0))
                                 Buffers: shared hit=581
                           ->  Bitmap Index Scan on idx_messages_thread_time  (cost=0.00..1756.38 rows=49593 width=0) (actual time=1.943..1.945 rows=50089 loops=1)
                                 Buffers: shared hit=168 read=206
               ->  Hash  (cost=3.94..3.94 rows=94 width=63) (actual time=0.317..0.321 rows=94 loops=1)
                     Buckets: 1024  Batches: 1  Memory Usage: 17kB
                     Buffers: shared hit=3
                     ->  Seq Scan on users u  (cost=0.00..3.94 rows=94 width=63) (actual time=0.007..0.148 rows=94 loops=1)
                           Buffers: shared hit=3
 Planning Time: 0.224 ms
 Execution Time: 524.598 ms
(31 rows)
```

## C-E. Partial indexes created in the same kind of rolled-back transaction: DISTINCT ON (C) vs LATERAL (D main, E replies)
```
=== C. DISTINCT ON with the partial index ===
                                                                         QUERY PLAN                                                                         
------------------------------------------------------------------------------------------------------------------------------------------------------------
 Unique  (cost=25446.10..25572.30 rows=115 width=152) (actual time=2146.869..3046.514 rows=115 loops=1)
   Buffers: shared hit=11047 read=151, temp read=5860 written=5874
   InitPlan 1 (returns $0)
     ->  Seq Scan on channels  (cost=0.00..7.15 rows=115 width=36) (actual time=0.006..0.190 rows=115 loops=1)
           Buffers: shared hit=6
   ->  Sort  (cost=25438.95..25502.05 rows=25240 width=152) (actual time=2146.864..2622.769 rows=300314 loops=1)
         Sort Key: m.channel_id, m.created_at DESC, m.id DESC
         Sort Method: external merge  Disk: 46880kB
         Buffers: shared hit=11047 read=151, temp read=5860 written=5874
         ->  Hash Left Join  (cost=427.40..21692.97 rows=25240 width=152) (actual time=12.807..1366.969 rows=300314 loops=1)
               Hash Cond: (m.sender_id = u.id)
               Buffers: shared hit=11047 read=151
               ->  Bitmap Heap Scan on messages m  (cost=422.29..21618.61 rows=25240 width=120) (actual time=12.465..502.707 rows=300314 loops=1)
                     Recheck Cond: (channel_id = ANY ($0))
                     Filter: ((parent_message_id IS NULL) AND (deleted_at IS NULL))
                     Rows Removed by Filter: 50087
                     Heap Blocks: exact=10592
                     Buffers: shared hit=11046 read=149
                     ->  Bitmap Index Scan on idx_messages_channel  (cost=0.00..415.98 rows=29305 width=0) (actual time=11.287..11.288 rows=350401 loops=1)
                           Index Cond: (channel_id = ANY ($0))
                           Buffers: shared hit=466 read=137
               ->  Hash  (cost=3.94..3.94 rows=94 width=63) (actual time=0.309..0.316 rows=94 loops=1)
                     Buckets: 1024  Batches: 1  Memory Usage: 17kB
                     Buffers: shared hit=1 read=2
                     ->  Seq Scan on users u  (cost=0.00..3.94 rows=94 width=63) (actual time=0.007..0.152 rows=94 loops=1)
                           Buffers: shared hit=1 read=2
 Planning:
   Buffers: shared hit=127 read=16
 Planning Time: 3.838 ms
 Execution Time: 3053.216 ms
(30 rows)

=== D. One row per channel through LATERAL, main stream ===
                                                                                 QUERY PLAN                                                                                  
-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------
 Nested Loop  (cost=11.41..54.63 rows=10 width=152) (actual time=0.495..37.281 rows=115 loops=1)
   Buffers: shared hit=662 read=149
   InitPlan 1 (returns $0)
     ->  Seq Scan on channels  (cost=0.00..7.15 rows=115 width=36) (actual time=0.006..0.186 rows=115 loops=1)
           Buffers: shared hit=6
   ->  Function Scan on unnest c  (cost=0.00..0.10 rows=10 width=32) (actual time=0.397..0.563 rows=115 loops=1)
         Buffers: shared hit=6
   ->  Hash Right Join  (cost=4.25..8.56 rows=1 width=147) (actual time=0.039..0.308 rows=1 loops=115)
         Hash Cond: (u.id = m.sender_id)
         Buffers: shared hit=656 read=149
         ->  Seq Scan on users u  (cost=0.00..3.94 rows=94 width=63) (actual time=0.002..0.134 rows=94 loops=115)
               Buffers: shared hit=345
         ->  Hash  (cost=4.24..4.24 rows=1 width=120) (actual time=0.032..0.032 rows=1 loops=115)
               Buckets: 1024  Batches: 1  Memory Usage: 9kB
               Buffers: shared hit=311 read=149
               ->  Subquery Scan on m  (cost=0.42..4.24 rows=1 width=120) (actual time=0.017..0.021 rows=1 loops=115)
                     Buffers: shared hit=311 read=149
                     ->  Limit  (cost=0.42..4.23 rows=1 width=468) (actual time=0.014..0.015 rows=1 loops=115)
                           Buffers: shared hit=311 read=149
                           ->  Index Scan using idx_messages_latest_main on messages x  (cost=0.42..9997.70 rows=2624 width=468) (actual time=0.011..0.011 rows=1 loops=115)
                                 Index Cond: (channel_id = c.id)
                                 Buffers: shared hit=311 read=149
 Planning Time: 0.317 ms
 Execution Time: 37.533 ms
(24 rows)

=== E. One row per channel through LATERAL, thread replies ===
                                                                                 QUERY PLAN                                                                                  
-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------
 Nested Loop  (cost=11.63..54.85 rows=10 width=152) (actual time=0.470..36.942 rows=115 loops=1)
   Buffers: shared hit=689 read=122
   InitPlan 1 (returns $0)
     ->  Seq Scan on channels  (cost=0.00..7.15 rows=115 width=36) (actual time=0.006..0.177 rows=115 loops=1)
           Buffers: shared hit=6
   ->  Function Scan on unnest c  (cost=0.00..0.10 rows=10 width=32) (actual time=0.392..0.564 rows=115 loops=1)
         Buffers: shared hit=6
   ->  Hash Right Join  (cost=4.47..8.78 rows=1 width=147) (actual time=0.036..0.305 rows=1 loops=115)
         Hash Cond: (u.id = m.sender_id)
         Buffers: shared hit=683 read=122
         ->  Seq Scan on users u  (cost=0.00..3.94 rows=94 width=63) (actual time=0.002..0.134 rows=94 loops=115)
               Buffers: shared hit=345
         ->  Hash  (cost=4.46..4.46 rows=1 width=120) (actual time=0.029..0.029 rows=1 loops=115)
               Buckets: 1024  Batches: 1  Memory Usage: 9kB
               Buffers: shared hit=338 read=122
               ->  Subquery Scan on m  (cost=0.41..4.46 rows=1 width=120) (actual time=0.014..0.018 rows=1 loops=115)
                     Buffers: shared hit=338 read=122
                     ->  Limit  (cost=0.41..4.45 rows=1 width=468) (actual time=0.011..0.013 rows=1 loops=115)
                           Buffers: shared hit=338 read=122
                           ->  Index Scan using idx_messages_latest_reply on messages x  (cost=0.41..1707.71 rows=423 width=468) (actual time=0.008..0.008 rows=1 loops=115)
                                 Index Cond: (channel_id = c.id)
                                 Buffers: shared hit=338 read=122
 Planning:
   Buffers: shared hit=2 read=1
 Planning Time: 0.321 ms
 Execution Time: 37.186 ms
(26 rows)

ROLLBACK
```

## F-G. Shipped queries on the real dev data, after the migration
```
=== F. Shipped query on dev data, main stream ===
                                                                            QUERY PLAN                                                                             
-------------------------------------------------------------------------------------------------------------------------------------------------------------------
 Hash Right Join  (cost=37.69..42.31 rows=10 width=200) (actual time=1.740..2.027 rows=16 loops=1)
   Hash Cond: (u.id = x.sender_id)
   Buffers: shared hit=253 read=2
   InitPlan 1 (returns $0)
     ->  Seq Scan on channels  (cost=0.00..7.15 rows=115 width=36) (actual time=0.020..0.190 rows=115 loops=1)
           Buffers: shared hit=6
   ->  Seq Scan on users u  (cost=0.00..3.94 rows=94 width=63) (actual time=0.012..0.155 rows=94 loops=1)
         Buffers: shared hit=3
   ->  Hash  (cost=30.41..30.41 rows=10 width=168) (actual time=1.670..1.682 rows=16 loops=1)
         Buckets: 1024  Batches: 1  Memory Usage: 11kB
         Buffers: shared hit=250 read=2
         ->  Nested Loop  (cost=0.28..30.41 rows=10 width=168) (actual time=0.456..1.643 rows=16 loops=1)
               Buffers: shared hit=250 read=2
               ->  Function Scan on unnest c  (cost=0.00..0.10 rows=10 width=32) (actual time=0.397..0.553 rows=115 loops=1)
                     Buffers: shared hit=6
               ->  Limit  (cost=0.27..3.01 rows=1 width=516) (actual time=0.006..0.006 rows=0 loops=115)
                     Buffers: shared hit=244 read=2
                     ->  Index Scan using idx_messages_latest_main on messages x  (cost=0.27..55.04 rows=20 width=516) (actual time=0.003..0.003 rows=0 loops=115)
                           Index Cond: (channel_id = c.id)
                           Buffers: shared hit=244 read=2
 Planning:
   Buffers: shared hit=550
 Planning Time: 3.695 ms
 Execution Time: 2.168 ms
(24 rows)

=== G. Shipped query on dev data, thread replies ===
                                                                               QUERY PLAN                                                                                
-------------------------------------------------------------------------------------------------------------------------------------------------------------------------
 Nested Loop  (cost=11.92..55.14 rows=10 width=200) (actual time=0.457..6.923 rows=9 loops=1)
   Buffers: shared hit=271 read=1
   InitPlan 1 (returns $0)
     ->  Seq Scan on channels  (cost=0.00..7.15 rows=115 width=36) (actual time=0.005..0.170 rows=115 loops=1)
           Buffers: shared hit=6
   ->  Function Scan on unnest c  (cost=0.00..0.10 rows=10 width=32) (actual time=0.364..0.528 rows=115 loops=1)
         Buffers: shared hit=6
   ->  Hash Right Join  (cost=4.76..9.07 rows=1 width=195) (actual time=0.038..0.047 rows=0 loops=115)
         Hash Cond: (u.id = m.sender_id)
         Buffers: shared hit=265 read=1
         ->  Seq Scan on users u  (cost=0.00..3.94 rows=94 width=63) (actual time=0.002..0.133 rows=94 loops=9)
               Buffers: shared hit=27
         ->  Hash  (cost=4.75..4.75 rows=1 width=168) (actual time=0.013..0.013 rows=0 loops=115)
               Buckets: 1024  Batches: 1  Memory Usage: 9kB
               Buffers: shared hit=238 read=1
               ->  Subquery Scan on m  (cost=0.27..4.75 rows=1 width=168) (actual time=0.009..0.009 rows=0 loops=115)
                     Buffers: shared hit=238 read=1
                     ->  Limit  (cost=0.27..4.74 rows=1 width=516) (actual time=0.006..0.006 rows=0 loops=115)
                           Buffers: shared hit=238 read=1
                           ->  Index Scan using idx_messages_latest_reply on messages x  (cost=0.27..22.64 rows=5 width=516) (actual time=0.003..0.003 rows=0 loops=115)
                                 Index Cond: (channel_id = c.id)
                                 Buffers: shared hit=238 read=1
 Planning:
   Buffers: shared hit=6
 Planning Time: 0.277 ms
 Execution Time: 7.019 ms
(26 rows)

```
