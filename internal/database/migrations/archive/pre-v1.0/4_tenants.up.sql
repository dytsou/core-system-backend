 CREATE TYPE db_strategy AS ENUM ('shared', 'isolated');

 CREATE TABLE IF NOT EXISTS tenants
 (
     id UUID PRIMARY KEY REFERENCES units(id) ON DELETE CASCADE,
     db_strategy db_strategy NOT NULL,
     owner_id UUID REFERENCES users(id) ON DELETE SET NULL
 );

 CREATE TABLE IF NOT EXISTS slug_history
 (
     id SERIAL PRIMARY KEY,
     slug TEXT NOT NULL,
     org_id UUID REFERENCES units(id) ON DELETE CASCADE,
     created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
     ended_at TIMESTAMPTZ DEFAULT null
 );