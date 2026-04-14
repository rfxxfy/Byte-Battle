ALTER TABLE games
    ADD COLUMN is_public    BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN invite_token UUID    NOT NULL DEFAULT gen_random_uuid();

CREATE UNIQUE INDEX games_invite_token_idx ON games(invite_token);
