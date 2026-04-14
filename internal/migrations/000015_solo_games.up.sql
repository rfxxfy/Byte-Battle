ALTER TABLE games
    ADD COLUMN is_solo            BOOLEAN  NOT NULL DEFAULT false,
    ADD COLUMN time_limit_minutes SMALLINT NULL;
