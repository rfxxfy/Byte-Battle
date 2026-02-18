DROP INDEX IF EXISTS idx_game_problems_problem_id;
DROP TABLE IF EXISTS game_problems;

ALTER TABLE games
    DROP COLUMN IF EXISTS current_problem_index;
