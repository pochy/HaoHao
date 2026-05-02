CREATE INDEX notifications_recipient_created_idx
    ON notifications(recipient_user_id, created_at DESC, id DESC);

CREATE INDEX notifications_recipient_unread_created_idx
    ON notifications(recipient_user_id, created_at DESC, id DESC)
    WHERE read_at IS NULL;

CREATE INDEX notifications_search_idx
    ON notifications
    USING gin (to_tsvector('simple', subject || ' ' || body || ' ' || template));
