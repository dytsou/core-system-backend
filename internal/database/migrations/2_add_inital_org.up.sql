DO $$
DECLARE
org_uuid UUID := '00000000-0000-0000-0000-000000000001';
BEGIN

INSERT INTO units (id, org_id, parent_id, type, name, description, metadata)
VALUES (
           org_uuid,
           NULL,
           NULL,
           'organization',
           'Default Organization',
           'System default organization',
           '{}'::jsonb
       );

INSERT INTO tenants (id, db_strategy, owner_id)
VALUES (
           org_uuid,
           'shared',
           NULL
       );

INSERT INTO slug_history (slug, org_id)
VALUES ('default-org', org_uuid);
END $$;
