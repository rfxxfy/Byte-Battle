package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"bytebattle/internal/database/models"

	"github.com/aarondl/sqlboiler/v4/queries/qm"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"),
	)
	db, err := sql.Open("postgres", connStr)
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	t.Cleanup(func() { db.Close() })
	return db
}

func TestGameDeleteCascadesParticipants(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	insertUser := func(username, email string) int {
		var id int
		err := db.QueryRowContext(ctx,
			`INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id`,
			username, email, "testhash",
		).Scan(&id)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
		})
		return id
	}
	user1 := insertUser("cascade_test_user1", "cascade_test1@example.com")
	user2 := insertUser("cascade_test_user2", "cascade_test2@example.com")

	var problemID int
	err := db.QueryRowContext(ctx,
		`INSERT INTO problems (title, description, difficulty, time_limit, memory_limit) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		"Cascade Test Problem", "desc", "easy", 1, 256,
	).Scan(&problemID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM problems WHERE id = $1", problemID)
	})

	repo := NewGameRepository(db)
	game, err := repo.Create(ctx, []Player{{ID: user1}, {ID: user2}}, problemID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM games WHERE id = $1", game.ID)
	})

	count, err := models.GameParticipants(qm.Where("game_id = ?", game.ID)).Count(ctx, db)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count, "expected 2 participants before delete")

	err = repo.Delete(ctx, game.ID)
	require.NoError(t, err)

	count, err = models.GameParticipants(qm.Where("game_id = ?", game.ID)).Count(ctx, db)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "participants should be cascade-deleted with the game")

	game2, err := repo.Create(ctx, []Player{{ID: user1}, {ID: user2}}, problemID)
	require.NoError(t, err, "should be able to create a new game with the same players after deletion")
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM games WHERE id = $1", game2.ID)
	})
}
