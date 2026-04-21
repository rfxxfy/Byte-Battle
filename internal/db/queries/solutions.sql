-- name: InsertSolution :exec
INSERT INTO solutions (user_id, problem_id, problem_version_id, game_id, code, language, status)
VALUES (@user_id, @problem_id, @problem_version_id, @game_id, @code, @language, 'passed')
ON CONFLICT (user_id, game_id, problem_id) DO NOTHING;

-- name: GetGameSolutions :many
SELECT
    s.user_id,
    s.problem_id,
    s.code,
    s.language,
    s.created_at,
    u.username,
    u.name
FROM solutions s
JOIN users u ON u.id = s.user_id
WHERE s.game_id = @game_id AND s.status = 'passed'
ORDER BY s.created_at;
