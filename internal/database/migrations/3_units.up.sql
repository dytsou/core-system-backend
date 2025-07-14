CREATE TYPE unit_type AS ENUM ('unit', 'organization');

CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id),
    name VARCHAR(255),
    description VARCHAR(255),
    metadata JSONB,
    type unit_type NOT NULL DEFAULT 'organization',
    slug VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(slug)
);

CREATE TABLE units (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    name VARCHAR(255),
    description VARCHAR(255),
    metadata JSONB,
    type unit_type NOT NULL DEFAULT 'unit',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE unit_members (
    unit_id UUID REFERENCES units(id) ON DELETE CASCADE,
    member_id UUID,
    PRIMARY KEY (unit_id, member_id)
);

CREATE TABLE org_members (
    org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    member_id UUID,
    PRIMARY KEY (org_id, member_id)
);

CREATE TABLE parent_child (
    parent_id UUID NOT NULL,
    child_id UUID NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    PRIMARY KEY (parent_id, child_id)
);