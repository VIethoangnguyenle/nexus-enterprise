-- Asset screens: how urgent a request is, and who a lifecycle step concerned.
--
-- urgency: the requester picks Thấp / Bình thường / Cao / Khẩn; the approver reads it in
-- the request list. Existing requests were all made without one and read as normal.
--
-- subject_user_id: the person an assignment step is about — the new holder on a hand-over,
-- the previous holder on a return. The history then says "giao cho X" / "thu hồi từ Y"
-- with a name, not just that a state changed. NULL for steps about no one (maintenance).
-- SET NULL so deleting a user never deletes audit history.
--
-- Idempotent.

ALTER TABLE asset_requests
    ADD COLUMN IF NOT EXISTS urgency TEXT NOT NULL DEFAULT 'normal';

ALTER TABLE asset_requests DROP CONSTRAINT IF EXISTS asset_requests_urgency_check;
ALTER TABLE asset_requests
    ADD CONSTRAINT asset_requests_urgency_check CHECK (urgency IN ('low', 'normal', 'high', 'urgent'));

ALTER TABLE asset_transitions
    ADD COLUMN IF NOT EXISTS subject_user_id TEXT REFERENCES users(id) ON DELETE SET NULL;

-- The dashboard's recent activity reads the newest steps across a workspace's assets.
CREATE INDEX IF NOT EXISTS idx_transitions_created ON asset_transitions(created_at DESC);
