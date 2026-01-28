-- Create new temporary enum with only original values
CREATE TYPE question_type_old AS ENUM(
    'short_text',
    'long_text',
    'single_choice',
    'multiple_choice',
    'date'
);

-- Change columns to use the old enum
ALTER TABLE questions 
    ALTER COLUMN type TYPE question_type_old USING type::text::question_type_old;
ALTER TABLE answers 
    ALTER COLUMN type TYPE question_type_old USING type::text::question_type_old;

-- Drop the new enum
DROP TYPE IF EXISTS question_type;

-- Rename the old enum back
ALTER TYPE question_type_old RENAME TO question_type;

-- Drop section-related columns
ALTER TABLE questions
    DROP COLUMN IF EXISTS section_id;
ALTER TABLE questions
    DROP COLUMN IF EXISTS source_id;

-- Add form_id column back
ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS form_id UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE;