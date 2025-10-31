-- +goose Up
-- SQL в разделе 'Up' выполняется при применении этой миграции

CREATE TABLE problems (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    difficulty VARCHAR(20) NOT NULL CHECK (difficulty IN ('easy', 'medium', 'hard')),
    time_limit INTEGER NOT NULL, -- в секундах
    memory_limit INTEGER NOT NULL, -- в МБ
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_problems_difficulty ON problems(difficulty);