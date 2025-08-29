CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID REFERENCES users(id) ON DELETE SET NULL,
    name VARCHAR(255),
    description VARCHAR(255),
    metadata JSONB,
    slug VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(slug)
    );

CREATE TABLE IF NOT EXISTS units (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255),
    description VARCHAR(255),
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS unit_members (
    unit_id UUID REFERENCES units(id) ON DELETE CASCADE,
    member_id UUID,
    PRIMARY KEY (unit_id, member_id)
);

CREATE TABLE IF NOT EXISTS org_members (
    org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    member_id UUID,
    PRIMARY KEY (org_id, member_id)
);

CREATE TABLE IF NOT EXISTS parent_child (
    parent_id UUID REFERENCES units(id) ON DELETE SET NULL,
    child_id UUID NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    PRIMARY KEY (child_id, org_id)
);