-- Original GORM table: retain all IDs, text and timestamps.
CREATE TABLE IF NOT EXISTS guestbooks (
 id bigserial PRIMARY KEY,
 name text,
 message text,
 created_at timestamptz
);
ALTER TABLE guestbooks ADD COLUMN status text NOT NULL DEFAULT 'pending';
ALTER TABLE guestbooks ADD CONSTRAINT guestbooks_status CHECK (status IN ('pending','approved','rejected'));
-- NOT VALID preserves historical oversized/empty rows, but constrains new writes.
ALTER TABLE guestbooks ADD CONSTRAINT guestbooks_input CHECK (
 name IS NOT NULL AND message IS NOT NULL AND
 char_length(btrim(name)) BETWEEN 1 AND 80 AND char_length(btrim(message)) BETWEEN 1 AND 2000
) NOT VALID;
ALTER TABLE guestbooks ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP;
CREATE INDEX guestbooks_public_page ON guestbooks (created_at DESC,id DESC) WHERE status='approved';
