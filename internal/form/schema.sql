CREATE TYPE status AS ENUM(
    'draft',
    'published'
);

CREATE TABLE IF NOT EXISTS forms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    description TEXT,
    status status NOT NULL DEFAULT 'draft',
    unit_id UUID REFERENCES units(id) ON DELETE CASCADE,
    last_editor UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

