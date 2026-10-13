# Search API migration — 2026-10-13

Migration: `data/migrations/051_message_search_simple.sql` (`messages.search_vector` -> `simple` config over tag-stripped text, GIN rebuilt).

## Backup (before applying to dev DB)
- `/home/zane/Desktop/projects/nexus-enterprise/.dev-backups/ngac-pre-051-20261013-092230.dump` (pg_dump -Fc of db `ngac`, 1.19 MB, `pg_restore -l` readable)
- `.dev-backups/` was not git-ignored; `.dev-backups/` added to `.gitignore`.

## Applied
- `make db-migrate` exit 0; run twice (second run is a no-op: migration checks the generated expression for `simple`).
- `\d messages`: `search_vector tsvector generated always as (to_tsvector('simple'::regconfig, regexp_replace(COALESCE(content_text, content), '<[^>]+>'::text, ' '::text, 'g'::text))) stored`; index `idx_messages_search gin (search_vector)`.
- Expression uses `COALESCE(content_text, content)` (content_text already holds entity-decoded text of html messages) instead of bare `content`; tags still stripped.

## Verification (in a rolled-back transaction on the dev DB)
- `Báo cáo đối soát tháng 9` -> `'9':6 'báo':1 'cáo':2 'soát':4 'tháng':5 'đối':3`; query `plainto_tsquery('simple','báo') && to_tsquery('simple', '''đối'':*')` finds it.
- `<p>xem <strong>kế hoạch</strong></p>` -> `'hoạch':3 'kế':2 'xem':1`; `strong` matches 0.
- EXPLAIN (with `enable_seqscan=off`, since dev has only 401 rows): `Bitmap Index Scan on idx_messages_search`, Index Cond `search_vector @@ '''báo'' & ''cáo'' & ''đối'':*'::tsquery`.
- Rolled back; 0 leftover rows.

## Rollback
```sql
DROP INDEX IF EXISTS idx_messages_search;
ALTER TABLE messages DROP COLUMN search_vector;
ALTER TABLE messages ADD COLUMN search_vector tsvector
    GENERATED ALWAYS AS (to_tsvector('english', COALESCE(content_text, content))) STORED;
CREATE INDEX idx_messages_search ON messages USING gin (search_vector);
```
(Re-running `make db-migrate` after the rollback re-applies 051; remove/rename it first, or restore from the dump: `pg_restore -U ngac -d ngac --clean --if-exists <dump>`.)
Also revert the Go code: `Store.SearchMessages` now queries with `simple`.

## Known limit
No unaccented search (`doi soat` does not find `đối soát`); recorded in `docs/specs/chat-search/spec.md`.
