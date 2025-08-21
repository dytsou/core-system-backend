CREATE TYPE content_type AS ENUM(
    'text',
    'form'
);

CREATE TABLE IF NOT EXISTS inbox_message(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    posted_by UUID NOT NULL references units(id),
    title varchar(255) NOT NULL,
    subtitle varchar(255),
    type content_type NOT NULL,
    content_id UUID,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_inbox_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID references users(id),
    message_id UUID references inbox_message(id),
    is_read boolean DEFAULT false,
    is_starred boolean DEFAULT false,
    is_archived boolean DEFAULT false
);