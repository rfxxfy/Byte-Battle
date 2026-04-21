DROP INDEX IF EXISTS idx_solutions_problem_version_id;

ALTER TABLE solutions
    DROP COLUMN IF EXISTS problem_version_id;

DROP INDEX IF EXISTS idx_game_problems_problem_version_id;

ALTER TABLE game_problems
    DROP COLUMN IF EXISTS problem_version_id;

ALTER TABLE problems
    DROP CONSTRAINT IF EXISTS fk_problems_current_version;

DROP INDEX IF EXISTS idx_problem_versions_problem_id;
DROP TABLE IF EXISTS problem_versions;

DROP INDEX IF EXISTS idx_problems_status;
DROP INDEX IF EXISTS idx_problems_visibility;
DROP INDEX IF EXISTS idx_problems_owner_user_id;
DROP TABLE IF EXISTS problems;
