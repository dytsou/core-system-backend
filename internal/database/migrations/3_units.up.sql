CREATE TYPE unit_type AS ENUM ('organization', 'unit');

CREATE TABLE IF NOT EXISTS units (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES units(id),
    parent_id UUID REFERENCES units(id) ON DELETE SET NULL,
    type unit_type NOT NULL DEFAULT 'unit',
    name VARCHAR(255),
    description VARCHAR(255),
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_units_parent_id ON units(parent_id);

CREATE TABLE IF NOT EXISTS unit_members (
    unit_id UUID REFERENCES units(id) ON DELETE CASCADE,
    member_id UUID,
    PRIMARY KEY (unit_id, member_id)
);
