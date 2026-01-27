ALTER TYPE question_type DROP VALUE IF NOT EXISTS 'dropdown';
ALTER TYPE question_type DROP VALUE IF NOT EXISTS 'detailed_multiple_choice';
ALTER TYPE question_type DROP VALUE IF NOT EXISTS 'linear_scale';
ALTER TYPE question_type DROP VALUE IF NOT EXISTS 'rating';
ALTER TYPE question_type DROP VALUE IF NOT EXISTS 'upload_file';
ALTER TYPE question_type DROP VALUE IF NOT EXISTS 'oauth_connect';
ALTER TYPE question_type DROP VALUE IF NOT EXISTS 'ranking';

ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS form_id UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE;
ALTER TABLE questions
    DROP COLUMN IF EXISTS section_id;
ALTER TABLE questions
    DROP COLUMN IF EXISTS source_id;