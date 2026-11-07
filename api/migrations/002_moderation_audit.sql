CREATE TABLE moderation_audit (
 id bigserial PRIMARY KEY,
 entry_id bigint NOT NULL,
 action text NOT NULL CHECK (action IN ('approve','reject','delete')),
 actor text NOT NULL CHECK (char_length(actor) BETWEEN 1 AND 80),
 created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- Deliberately no foreign key: audit survives deletion of the guestbook row.
