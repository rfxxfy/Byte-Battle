ALTER TABLE games
    ADD COLUMN current_problem_index INTEGER NOT NULL DEFAULT 0 CHECK (current_problem_index >= 0);
