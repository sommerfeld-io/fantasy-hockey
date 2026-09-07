-- 0001_init: creates the participant and login_code tables.
--
-- Participant identity (name/email) is never seeded here - it is upserted at
-- application startup from PARTICIPANT_*_NAME/PARTICIPANT_*_EMAIL env vars so
-- personal data never enters git history. This migration only creates the
-- shape.

CREATE TABLE participant (
    id         UUID PRIMARY KEY,
    slot       SMALLINT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    email      TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE login_code (
    id             UUID PRIMARY KEY,
    participant_id UUID NOT NULL REFERENCES participant (id) ON DELETE CASCADE,
    code_hash      TEXT NOT NULL,
    issued_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    used_at        TIMESTAMPTZ
);

CREATE INDEX login_code_participant_id_idx ON login_code (participant_id);
