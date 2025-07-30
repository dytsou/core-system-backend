CREATE TABLE org_unit_ids (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid()
);

CREATE TABLE organizations (
    id UUID PRIMARY KEY REFERENCES org_unit_ids(id) ON DELETE CASCADE,
    owner_id UUID NOT NULL REFERENCES users(id),
    name VARCHAR(255),
    description VARCHAR(255),
    metadata JSONB,
    slug VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(slug)
);

CREATE TABLE units (
    id UUID PRIMARY KEY REFERENCES org_unit_ids(id) ON DELETE CASCADE,
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
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

CREATE TABLE org_members (
    org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    member_id UUID,
    PRIMARY KEY (org_id, member_id)
);

CREATE TABLE parent_child (
    parent_id UUID NOT NULL REFERENCES org_unit_ids(id) ON DELETE CASCADE,
    child_id UUID NOT NULL REFERENCES org_unit_ids(id) ON DELETE CASCADE,
    PRIMARY KEY (parent_id, child_id)
);