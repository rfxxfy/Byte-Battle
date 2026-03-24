ALTER TABLE games
    DROP COLUMN IF EXISTS time_limit_minutes,
    DROP COLUMN IF EXISTS is_solo;
