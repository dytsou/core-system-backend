CREATE TYPE unit_type AS ENUM ('organization', 'unit');

CREATE TABLE units (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES units(id),
    type unit_type NOT NULL DEFAULT 'unit',
    name VARCHAR(255),
    description VARCHAR(255),
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE unit_members (
    unit_id UUID REFERENCES units(id) ON DELETE CASCADE,
    member_id UUID,
    PRIMARY KEY (unit_id, member_id)
);

CREATE TABLE parent_child (
    parent_id UUID REFERENCES units(id) ON DELETE SET NULL,
    child_id UUID NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    org_id UUID NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    PRIMARY KEY (child_id, org_id)
);