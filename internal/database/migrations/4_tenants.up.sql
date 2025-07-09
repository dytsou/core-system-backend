CREATE TYPE db_strategy AS ENUM ('shared', 'isolated');

CREATE TABLE IF NOT EXISTS tenants
(
    id          UUID PRIMARY KEY REFERENCES organizations (id) ON DELETE CASCADE,
    db_strategy db_strategy NOT NULL
);