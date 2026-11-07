## Run 2026-11-07 02:12 — query set `after`

- messages | 50000000
- channels | 20191
- size | 37 GB
- server: 16.14, shared_buffers 128MB
- pgbench: 50 clients, 30 s per query

### latest-activity-after

```
 Nested Loop Left Join (actual time=0.760..39.323 rows=201 loops=1)
     ->  Function Scan on generate_series g (actual time=0.021..0.042 rows=201 loops=1)
   ->  Nested Loop (actual time=0.472..36.654 rows=201 loops=1)
         ->  Function Scan on unnest c (actual time=0.078..0.110 rows=201 loops=1)
         ->  Limit (actual time=0.181..0.181 rows=1 loops=201)
               ->  Index Scan using idx_messages_latest_main on messages x (actual time=0.181..0.181 rows=1 loops=201)
                     Index Cond: (channel_id = c.id)
   ->  Memoize (actual time=0.012..0.012 rows=1 loops=201)
         ->  Index Scan using users_pkey on users u (actual time=0.011..0.011 rows=1 loops=201)
               Index Cond: (id = x.sender_id)
 Planning Time: 8.182 ms
 Execution Time: 39.547 ms
```

- latency average = 18.967 ms
- tps = 2636.213183 (without initial connection time)
- latency p50 = 16.9 ms, p95 = 29.7 ms, p99 = 36.7 ms

### list-messages-after

```
 Limit (actual time=1.285..1.965 rows=51 loops=1)
   ->  Nested Loop Left Join (actual time=1.285..1.961 rows=51 loops=1)
         ->  Index Scan using messages_channel_seq on messages m (actual time=1.164..1.726 rows=51 loops=1)
               Index Cond: ((channel_id = 'c1_0'::text) AND (seq < 500000))
         ->  Memoize (actual time=0.004..0.004 rows=1 loops=51)
               ->  Index Scan using users_pkey on users u (actual time=0.004..0.004 rows=1 loops=51)
                     Index Cond: (id = m.sender_id)
 Planning Time: 4.365 ms
 Execution Time: 2.882 ms
```

- latency average = 18.183 ms
- tps = 2749.759541 (without initial connection time)
- latency p50 = 14.7 ms, p95 = 28.8 ms, p99 = 38.4 ms

### search-common-after

```
 Limit (actual time=7121.247..7121.254 rows=26 loops=1)
     ->  Function Scan on generate_series g (actual time=0.019..0.041 rows=201 loops=1)
   ->  Sort (actual time=7121.245..7121.250 rows=26 loops=1)
         ->  Hash Left Join (actual time=1134.404..6666.859 rows=1470545 loops=1)
               ->  Bitmap Heap Scan on messages m (actual time=1098.121..6162.759 rows=1470545 loops=1)
                     Rows Removed by Index Recheck: 751534
                     ->  BitmapAnd (actual time=1090.182..1090.184 rows=0 loops=1)
                           ->  Bitmap Index Scan on idx_messages_channel (actual time=43.426..43.427 rows=1485399 loops=1)
                                 Index Cond: (channel_id = ANY ($0))
                           ->  Bitmap Index Scan on idx_messages_search (actual time=1044.358..1044.358 rows=3490119 loops=1)
                                 Index Cond: (search_vector @@ '''messa'':*'::tsquery)
               ->  Hash (actual time=35.716..35.717 rows=104950 loops=1)
                     ->  Seq Scan on users u (actual time=0.017..13.833 rows=104950 loops=1)
 Planning Time: 5.959 ms
 Execution Time: 7124.058 ms
```

- latency average = 44493.486 ms
- tps = 1.123760 (without initial connection time)
- latency p50 = 44432.0 ms, p95 = 44486.5 ms, p99 = 44489.0 ms

### search-rare-after

```
 Limit (actual time=105.320..105.324 rows=1 loops=1)
     ->  Function Scan on generate_series g (actual time=0.019..0.041 rows=201 loops=1)
   ->  Sort (actual time=105.318..105.321 rows=1 loops=1)
         ->  Hash Left Join (actual time=104.876..105.265 rows=1 loops=1)
               ->  Bitmap Heap Scan on messages m (actual time=69.997..70.383 rows=1 loops=1)
                     Rows Removed by Index Recheck: 80
                     ->  BitmapAnd (actual time=69.680..69.681 rows=0 loops=1)
                           ->  Bitmap Index Scan on idx_messages_channel (actual time=67.668..67.668 rows=1485399 loops=1)
                                 Index Cond: (channel_id = ANY ($0))
                           ->  Bitmap Index Scan on idx_messages_search (actual time=0.321..0.321 rows=112 loops=1)
                                 Index Cond: (search_vector @@ '''123456'':*'::tsquery)
               ->  Hash (actual time=34.318..34.319 rows=104950 loops=1)
                     ->  Seq Scan on users u (actual time=0.158..17.978 rows=104950 loops=1)
 Planning Time: 15.535 ms
 Execution Time: 106.883 ms
```

- latency average = 600.959 ms
- tps = 83.200326 (without initial connection time)
- latency p50 = 580.4 ms, p95 = 1017.1 ms, p99 = 1140.4 ms

### unread-counts-after

```
 Nested Loop Left Join (actual time=102.789..128.967 rows=201 loops=1)
     ->  Append (actual time=0.406..6.703 rows=220 loops=1)
           ->  Bitmap Heap Scan on thread_follows tf (actual time=0.406..2.197 rows=20 loops=1)
                 ->  Bitmap Index Scan on thread_follows_pkey (actual time=0.286..0.286 rows=20 loops=1)
                       Index Cond: (user_id = 'u1_500'::text)
           ->  Nested Loop Anti Join (actual time=1.841..4.460 rows=200 loops=1)
                 ->  Bitmap Heap Scan on messages m_1 (actual time=1.668..3.643 rows=200 loops=1)
                       ->  BitmapOr (actual time=0.567..0.568 rows=0 loops=1)
                             ->  Bitmap Index Scan on idx_messages_thread_starter (actual time=0.342..0.342 rows=200 loops=1)
                                   Index Cond: (sender_id = 'u1_500'::text)
                             ->  Bitmap Index Scan on idx_messages_mentions (actual time=0.225..0.225 rows=32 loops=1)
                                   Index Cond: (mentions @> '{u1_500}'::text[])
                 ->  Materialize (actual time=0.001..0.002 rows=20 loops=200)
                       ->  Index Only Scan using thread_follows_pkey on thread_follows tf_1 (actual time=0.160..0.161 rows=20 loops=1)
                             Index Cond: (user_id = 'u1_500'::text)
     ->  Seq Scan on channel_notification_levels (actual time=0.132..0.133 rows=0 loops=1)
   ->  Merge Left Join (actual time=102.056..103.081 rows=201 loops=1)
         ->  Merge Left Join (actual time=102.035..103.027 rows=201 loops=1)
               ->  Merge Left Join (actual time=60.315..61.277 rows=201 loops=1)
                     ->  Merge Left Join (actual time=60.133..61.058 rows=201 loops=1)
                           ->  Nested Loop Left Join (actual time=0.633..1.521 rows=201 loops=1)
                                 ->  Unique (actual time=0.171..0.266 rows=201 loops=1)
                                         ->  Function Scan on generate_series g_1 (actual time=0.019..0.041 rows=201 loops=1)
                                       ->  Sort (actual time=0.171..0.198 rows=201 loops=1)
                                             ->  ProjectSet (actual time=0.073..0.082 rows=201 loops=1)
                                                   ->  Result (actual time=0.000..0.001 rows=1 loops=1)
                                 ->  Index Scan using channels_pkey on channels c (actual time=0.006..0.006 rows=1 loops=201)
                                       Index Cond: (id = (unnest($5)))
                           ->  GroupAggregate (actual time=59.496..59.498 rows=1 loops=1)
                                 ->  Sort (actual time=59.452..59.465 rows=200 loops=1)
                                       ->  Nested Loop (actual time=5.299..59.310 rows=200 loops=1)
                                             ->  Nested Loop (actual time=5.162..34.453 rows=200 loops=1)
                                                   ->  CTE Scan on followed f (actual time=0.407..6.840 rows=220 loops=1)
                                                   ->  Index Scan using messages_thread_reply_seq on messages r (actual time=0.124..0.125 rows=1 loops=220)
                                                         Index Cond: (parent_message_id = f.id)
                                             ->  Index Scan using messages_pkey on messages root (actual time=0.123..0.123 rows=1 loops=200)
                                                   Index Cond: (id = r.parent_message_id)
                     ->  GroupAggregate (actual time=0.181..0.184 rows=0 loops=1)
                           ->  Sort (actual time=0.180..0.183 rows=0 loops=1)
                                 ->  Hash Left Join (actual time=0.135..0.138 rows=0 loops=1)
```

- latency average = 519.099 ms
- tps = 96.320687 (without initial connection time)
- latency p50 = 313.2 ms, p95 = 2256.1 ms, p99 = 2806.4 ms

