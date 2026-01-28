ALTER TYPE question_type ADD VALUE IF NOT EXISTS 'dropdown';
ALTER TYPE question_type ADD VALUE IF NOT EXISTS 'detailed_multiple_choice';
ALTER TYPE question_type ADD VALUE IF NOT EXISTS 'linear_scale';
ALTER TYPE question_type ADD VALUE IF NOT EXISTS 'rating';
ALTER TYPE question_type ADD VALUE IF NOT EXISTS 'upload_file';
ALTER TYPE question_type ADD VALUE IF NOT EXISTS 'oauth_connect';
ALTER TYPE question_type ADD VALUE IF NOT EXISTS 'ranking';

-- Drop old form_id column and add section_id column
ALTER TABLE questions
    DROP COLUMN IF EXISTS form_id;
    
ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS section_id UUID NOT NULL REFERENCES sections(id) ON DELETE CASCADE;

-- Add source_id column (nullable)
ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS source_id UUID;