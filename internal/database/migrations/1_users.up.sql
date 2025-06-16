CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    username VARCHAR(255) NOT NULL,
    avatar_url VARCHAR(512) NOT NULL,
    role VARCHAR(255) NOT NULL DEFAULT 'user',
    oauth_provider VARCHAR(50) NOT NULL,
    oauth_user_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(oauth_provider, oauth_user_id)
);
