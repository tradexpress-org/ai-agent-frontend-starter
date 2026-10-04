CREATE TABLE datasets (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    records JSONB NOT NULL CHECK (jsonb_typeof(records) = 'array'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
