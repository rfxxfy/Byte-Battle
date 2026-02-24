CREATE TABLE games (
    id SERIAL PRIMARY KEY,
    problem_id INTEGER NOT NULL REFERENCES problems(id),
    winner_id INTEGER REFERENCES users(id),
    status VARCHAR(20) NOT NULL
        CHECK (status IN ('pending', 'active', 'finished', 'cancelled')),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_games_problem_id ON games(problem_id);
CREATE INDEX idx_games_winner_id ON games(winner_id);
CREATE INDEX idx_games_status ON games(status);
