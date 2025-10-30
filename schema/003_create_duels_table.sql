-- +goose Up
-- SQL в разделе 'Up' выполняется при применении этой миграции

CREATE TABLE duels (
    id SERIAL PRIMARY KEY,
    player1_id INTEGER NOT NULL REFERENCES users(id),
    player2_id INTEGER NOT NULL REFERENCES users(id),
    problem_id INTEGER NOT NULL REFERENCES problems(id),
    winner_id INTEGER REFERENCES users(id),
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'active', 'completed', 'cancelled')),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_duels_player1_id ON duels(player1_id);
CREATE INDEX idx_duels_player2_id ON duels(player2_id);
CREATE INDEX idx_duels_problem_id ON duels(problem_id);
CREATE INDEX idx_duels_status ON duels(status);