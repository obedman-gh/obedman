-- Run as owner after migration, in a database dedicated to this app.
REVOKE ALL ON guestbooks, moderation_audit, schema_migrations FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO guestbook_runtime, guestbook_moderator;
GRANT SELECT ON guestbooks, schema_migrations TO guestbook_runtime;
GRANT INSERT (name,message) ON guestbooks TO guestbook_runtime;
GRANT USAGE ON SEQUENCE guestbooks_id_seq TO guestbook_runtime;
GRANT SELECT, UPDATE (status,created_at), DELETE ON guestbooks TO guestbook_moderator;
GRANT INSERT (entry_id,action,actor) ON moderation_audit TO guestbook_moderator;
GRANT USAGE ON SEQUENCE moderation_audit_id_seq TO guestbook_moderator;
-- Runtime cannot UPDATE, DELETE, write audit rows, or execute DDL.
