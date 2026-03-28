DROP INDEX IF EXISTS idx_games_problem_id;

ALTER TABLE games
    DROP COLUMN IF EXISTS problem_id;

