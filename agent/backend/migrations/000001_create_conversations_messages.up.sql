CREATE TABLE conversations (
    id         uuid        PRIMARY KEY,
    user_sub   text        NOT NULL,
    title      text        NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE INDEX conversations_user_sub_updated_at_idx ON conversations (user_sub, updated_at DESC);

CREATE TABLE messages (
    id              uuid        PRIMARY KEY,
    conversation_id uuid        NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    seq             int         NOT NULL,
    role            text        NOT NULL CHECK (role IN ('user', 'assistant', 'tool')),
    -- The provider-neutral []chatdto.Part, so changing the model provider leaves the data as is.
    parts           jsonb       NOT NULL,
    created_at      timestamptz NOT NULL,
    UNIQUE (conversation_id, seq)
);
