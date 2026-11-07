-- Run once as database owner on the dedicated guestbook database.
-- Set LOGIN passwords interactively with psql \password; never commit them.
CREATE ROLE guestbook_migrator LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE guestbook_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE guestbook_moderator LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE, CREATE ON SCHEMA public TO guestbook_migrator;
-- Run all migrations as guestbook_migrator before running grants.sql.
