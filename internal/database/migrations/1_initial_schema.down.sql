DROP VIEW IF EXISTS public.users_with_emails;

DROP TABLE IF EXISTS
    public.answers,
    public.auth,
    public.form_responses,
    public.forms,
    public.inbox_message,
    public.questions,
    public.refresh_tokens,
    public.tenants,
    public.unit_members,
    public.units,
    public.user_emails,
    public.user_inbox_messages,
    public.users CASCADE;

DROP TYPE IF EXISTS public.content_type;
DROP TYPE IF EXISTS public.db_strategy;
DROP TYPE IF EXISTS public.question_type;
DROP TYPE IF EXISTS public.status;
DROP TYPE IF EXISTS public.unit_type;

DROP EXTENSION IF EXISTS pgcrypto;
