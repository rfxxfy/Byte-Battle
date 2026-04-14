ALTER TABLE solutions
    ADD CONSTRAINT solutions_user_game_problem_unique
    UNIQUE (user_id, game_id, problem_id);
