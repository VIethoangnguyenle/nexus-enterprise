## Run 2026-11-06 23:08 — query set `current`

- messages | 50000000
- channels | 20191
- size | 20 GB
- server: 16.14, shared_buffers 128MB
- pgbench: 50 clients, 30 s per query

### latest-activity-current

```
 Nested Loop Left Join (actual time=0.163..3.059 rows=201 loops=1)
     ->  Function Scan on generate_series g (actual time=0.026..0.047 rows=201 loops=1)
   ->  Nested Loop (actual time=0.137..2.371 rows=201 loops=1)
         ->  Function Scan on unnest c (actual time=0.087..0.100 rows=201 loops=1)
         ->  Limit (actual time=0.011..0.011 rows=1 loops=201)
               ->  Index Scan using idx_messages_latest_main on messages x (actual time=0.011..0.011 rows=1 loops=201)
                     Index Cond: (channel_id = c.id)
   ->  Memoize (actual time=0.003..0.003 rows=1 loops=201)
         ->  Index Scan using users_pkey on users u (actual time=0.002..0.002 rows=1 loops=201)
               Index Cond: (id = x.sender_id)
 Planning Time: 4.121 ms
 Execution Time: 3.231 ms
```

- latency average = 23.478 ms
- tps = 2129.642220 (without initial connection time)
- latency p50 = 20.9 ms, p95 = 37.5 ms, p99 = 47.3 ms

### list-messages-current

```
 Limit (actual time=0.081..0.237 rows=51 loops=1)
   ->  Nested Loop Left Join (actual time=0.081..0.233 rows=51 loops=1)
         ->  Index Scan using idx_messages_channel_time on messages m (actual time=0.053..0.091 rows=51 loops=1)
               Index Cond: ((channel_id = 'c1_0'::text) AND (created_at < ('2026-01-01 00:00:00+00'::timestamp with time zone - '100 days'::interval)))
         ->  Memoize (actual time=0.002..0.002 rows=1 loops=51)
               ->  Index Scan using users_pkey on users u (actual time=0.002..0.002 rows=1 loops=51)
                     Index Cond: (id = m.sender_id)
 Planning Time: 3.975 ms
 Execution Time: 2.002 ms
```

- latency average = 26.417 ms
- tps = 1892.709551 (without initial connection time)
- latency p50 = 22.2 ms, p95 = 45.7 ms, p99 = 62.3 ms

### search-common-current

```
 Limit (actual time=6132.357..6132.364 rows=26 loops=1)
     ->  Function Scan on generate_series g (actual time=0.020..0.042 rows=201 loops=1)
   ->  Sort (actual time=6132.356..6132.359 rows=26 loops=1)
         ->  Nested Loop Left Join (actual time=930.342..5573.001 rows=1470545 loops=1)
               ->  Bitmap Heap Scan on messages m (actual time=930.301..2577.175 rows=1470545 loops=1)
                     ->  BitmapAnd (actual time=921.672..921.673 rows=0 loops=1)
                           ->  Bitmap Index Scan on idx_messages_channel (actual time=50.164..50.164 rows=1485399 loops=1)
                                 Index Cond: (channel_id = ANY ($0))
                           ->  Bitmap Index Scan on idx_messages_search (actual time=868.855..868.855 rows=3232432 loops=1)
                                 Index Cond: (search_vector @@ '''messa'':*'::tsquery)
               ->  Index Scan using users_pkey on users u (actual time=0.002..0.002 rows=1 loops=1470545)
                     Index Cond: (id = m.sender_id)
 Planning Time: 4.227 ms
 Execution Time: 6134.862 ms
```

- latency average = 49953.483 ms
- tps = 1.000931 (without initial connection time)
- latency p50 = 48115.5 ms, p95 = 49583.8 ms, p99 = 49947.6 ms

### search-rare-current

```
 Limit (actual time=58.151..58.153 rows=1 loops=1)
     ->  Function Scan on generate_series g (actual time=0.051..0.097 rows=201 loops=1)
   ->  Sort (actual time=58.148..58.150 rows=1 loops=1)
         ->  Nested Loop Left Join (actual time=58.054..58.057 rows=1 loops=1)
               ->  Bitmap Heap Scan on messages m (actual time=57.998..57.999 rows=1 loops=1)
                     ->  BitmapAnd (actual time=57.718..57.719 rows=0 loops=1)
                           ->  Bitmap Index Scan on idx_messages_channel (actual time=56.357..56.357 rows=1485399 loops=1)
                                 Index Cond: (channel_id = ANY ($0))
                           ->  Bitmap Index Scan on idx_messages_search (actual time=0.097..0.098 rows=112 loops=1)
                                 Index Cond: (search_vector @@ '''123456'':*'::tsquery)
               ->  Index Scan using users_pkey on users u (actual time=0.043..0.043 rows=1 loops=1)
                     Index Cond: (id = m.sender_id)
 Planning Time: 5.952 ms
 Execution Time: 58.946 ms
```

- latency average = 435.707 ms
- tps = 114.756146 (without initial connection time)
- latency p50 = 406.7 ms, p95 = 875.2 ms, p99 = 1068.3 ms

### unread-counts-current

```
 HashAggregate (actual time=318.734..318.806 rows=201 loops=1)
     ->  Append (actual time=0.041..1.504 rows=220 loops=1)
           ->  Bitmap Heap Scan on thread_follows tf (actual time=0.041..0.166 rows=20 loops=1)
                 ->  Bitmap Index Scan on thread_follows_pkey (actual time=0.030..0.030 rows=20 loops=1)
                       Index Cond: (user_id = 'u1_500'::text)
           ->  Hash Anti Join (actual time=0.129..1.317 rows=200 loops=1)
                 ->  Bitmap Heap Scan on messages m_2 (actual time=0.063..1.206 rows=200 loops=1)
                       ->  BitmapOr (actual time=0.044..0.045 rows=0 loops=1)
                             ->  Bitmap Index Scan on idx_messages_thread_starter (actual time=0.035..0.035 rows=200 loops=1)
                                   Index Cond: (sender_id = 'u1_500'::text)
                             ->  Bitmap Index Scan on idx_messages_thread_mentions (actual time=0.009..0.009 rows=0 loops=1)
                                   Index Cond: (mentions @> '{u1_500}'::text[])
                 ->  Hash (actual time=0.043..0.044 rows=20 loops=1)
                       ->  Index Only Scan using thread_follows_pkey on thread_follows tf_1 (actual time=0.031..0.033 rows=20 loops=1)
                             Index Cond: (user_id = 'u1_500'::text)
     ->  Seq Scan on channel_notification_levels (actual time=0.017..0.017 rows=0 loops=1)
   ->  Nested Loop Left Join (actual time=53.256..307.005 rows=69675 loops=1)
         ->  Merge Left Join (actual time=53.191..55.003 rows=201 loops=1)
               ->  Merge Left Join (actual time=53.172..54.951 rows=201 loops=1)
                     ->  Merge Left Join (actual time=6.621..8.359 rows=201 loops=1)
                           ->  Merge Left Join (actual time=6.580..8.281 rows=201 loops=1)
                                 ->  Nested Loop Left Join (actual time=0.211..1.867 rows=201 loops=1)
                                       ->  Unique (actual time=0.168..0.263 rows=201 loops=1)
                                               ->  Function Scan on generate_series g_1 (actual time=0.022..0.046 rows=201 loops=1)
                                             ->  Sort (actual time=0.168..0.192 rows=201 loops=1)
                                                   ->  ProjectSet (actual time=0.070..0.080 rows=201 loops=1)
                                                         ->  Result (actual time=0.001..0.001 rows=1 loops=1)
                                       ->  Index Scan using read_receipts_pkey on read_receipts rr (actual time=0.007..0.007 rows=1 loops=201)
                                             Index Cond: ((user_id = 'u1_500'::text) AND (channel_id = (unnest($3))))
                                 ->  GroupAggregate (actual time=6.367..6.370 rows=1 loops=1)
                                       ->  Sort (actual time=6.339..6.349 rows=200 loops=1)
                                             ->  Nested Loop (actual time=0.421..6.280 rows=200 loops=1)
                                                   ->  Nested Loop (actual time=0.396..4.325 rows=200 loops=1)
                                                         ->  CTE Scan on followed f (actual time=0.042..1.565 rows=220 loops=1)
                                                         ->  Index Scan using idx_messages_thread_time on messages r (actual time=0.012..0.012 rows=1 loops=220)
                                                               Index Cond: ((parent_message_id = f.id) AND (created_at > f.last_read_at))
                                                   ->  Index Scan using messages_pkey on messages root (actual time=0.009..0.009 rows=1 loops=200)
                                                         Index Cond: (id = r.parent_message_id)
                           ->  GroupAggregate (actual time=0.040..0.043 rows=0 loops=1)
                                 ->  Sort (actual time=0.039..0.042 rows=0 loops=1)
```

- latency average = 2084.122 ms
- tps = 23.990920 (without initial connection time)
- latency p50 = 1916.0 ms, p95 = 3428.1 ms, p99 = 4085.3 ms

