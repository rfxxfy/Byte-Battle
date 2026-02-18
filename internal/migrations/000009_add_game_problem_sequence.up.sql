ALTER TABLE games
    ADD COLUMN current_problem_index INTEGER NOT NULL DEFAULT 0 CHECK (current_problem_index >= 0);

CREATE TABLE game_problems (
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    problem_index INTEGER NOT NULL CHECK (problem_index >= 0 AND problem_index < 20), -- sync with maxGameProblems in game_service.go
    problem_id TEXT NOT NULL,
    PRIMARY KEY (game_id, problem_index)
);

CREATE INDEX idx_game_problems_problem_id ON game_problems(problem_id);

INSERT INTO game_problems (game_id, problem_index, problem_id)
SELECT id, 0, problem_id
FROM games;
