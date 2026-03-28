ALTER TABLE games
    ADD COLUMN IF NOT EXISTS problem_id TEXT NOT NULL DEFAULT '';

UPDATE games g
SET problem_id = COALESCE(gp.problem_id, '')
FROM game_problems gp
WHERE gp.game_id = g.id
  AND gp.problem_index = g.current_problem_index;

CREATE INDEX IF NOT EXISTS idx_games_problem_id ON games(problem_id);

