CREATE TABLE problems (
    id BIGSERIAL PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    owner_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    visibility TEXT NOT NULL DEFAULT 'public'
        CHECK (visibility IN ('public', 'unlisted', 'private')),
    status TEXT NOT NULL DEFAULT 'published'
        CHECK (status IN ('published', 'archived')),
    title TEXT NOT NULL,
    current_version_id BIGINT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_problems_owner_user_id ON problems(owner_user_id);
CREATE INDEX idx_problems_visibility ON problems(visibility);
CREATE INDEX idx_problems_status ON problems(status);

CREATE TABLE problem_versions (
    id BIGSERIAL PRIMARY KEY,
    problem_id BIGINT NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    version INTEGER NOT NULL CHECK (version > 0),
    artifact_path TEXT NOT NULL,
    artifact_sha256 TEXT NOT NULL,
    limits_time_ms INTEGER NOT NULL CHECK (limits_time_ms > 0),
    limits_memory_kb INTEGER NOT NULL CHECK (limits_memory_kb > 0),
    checker_type TEXT NOT NULL CHECK (checker_type IN ('diff', 'custom')),
    reference_language TEXT NOT NULL CHECK (reference_language IN ('python', 'go', 'cpp', 'java')),
    created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (problem_id, version)
);

CREATE INDEX idx_problem_versions_problem_id ON problem_versions(problem_id);

ALTER TABLE problems
    ADD CONSTRAINT fk_problems_current_version
    FOREIGN KEY (current_version_id)
    REFERENCES problem_versions(id)
    ON DELETE SET NULL;

ALTER TABLE game_problems
    ADD COLUMN problem_version_id BIGINT NOT NULL REFERENCES problem_versions(id) ON DELETE RESTRICT;

CREATE INDEX idx_game_problems_problem_version_id ON game_problems(problem_version_id);

ALTER TABLE solutions
    ADD COLUMN problem_version_id BIGINT NOT NULL REFERENCES problem_versions(id) ON DELETE RESTRICT;

CREATE INDEX idx_solutions_problem_version_id ON solutions(problem_version_id);
