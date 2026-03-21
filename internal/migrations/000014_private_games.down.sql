DROP INDEX IF EXISTS games_invite_token_idx;
ALTER TABLE games DROP COLUMN IF EXISTS invite_token, DROP COLUMN IF EXISTS is_public;
