ALTER TABLE problem_versions
    ADD COLUMN test_case_count INTEGER NOT NULL DEFAULT 0 CHECK (test_case_count >= 0),
    ADD COLUMN difficulty       TEXT    NOT NULL DEFAULT '' CHECK (difficulty IN ('', 'easy', 'medium', 'hard'));
