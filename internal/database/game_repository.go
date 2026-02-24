package database

import (
	"context"
	"database/sql"
	"errors"

	"bytebattle/internal/database/models"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
)

type GameStatus string

const (
	GameStatusPending   GameStatus = "pending"
	GameStatusActive    GameStatus = "active"
	GameStatusFinished  GameStatus = "finished"
	GameStatusCancelled GameStatus = "cancelled"
)

type Player struct {
	ID int
}

type IGameRepo interface {
	Create(ctx context.Context, players []Player, problemID int) (*models.Game, error)
	GetByID(ctx context.Context, id int) (*models.Game, error)
	GetAll(ctx context.Context, limit, offset int) (models.GameSlice, error)
	Upsert(ctx context.Context, game *models.Game) error
	Delete(ctx context.Context, id int) error
	IsParticipant(ctx context.Context, gameID, userID int) (bool, error)
}

type gameRepo struct {
	db *sql.DB
}

func NewGameRepository(db *sql.DB) IGameRepo {
	return &gameRepo{db: db}
}

func (r *gameRepo) Create(ctx context.Context, players []Player, problemID int) (*models.Game, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	game := &models.Game{
		ProblemID: problemID,
		Status:    string(GameStatusPending),
	}

	err = game.Insert(ctx, tx, boil.Infer())
	if err != nil {
		return nil, err
	}

	for _, p := range players {
		participant := &models.GameParticipant{
			GameID: game.ID,
			UserID: p.ID,
		}
		err = participant.Insert(ctx, tx, boil.Infer())
		if err != nil {
			return nil, err
		}
	}

	return game, tx.Commit()
}

func (r *gameRepo) GetByID(ctx context.Context, id int) (*models.Game, error) {
	game, err := models.FindGame(ctx, r.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return game, err
}

func (r *gameRepo) GetAll(ctx context.Context, limit, offset int) (models.GameSlice, error) {
	return models.Games(
		qm.Limit(limit),
		qm.Offset(offset),
		qm.OrderBy("created_at DESC"),
	).All(ctx, r.db)
}

func (r *gameRepo) Upsert(ctx context.Context, game *models.Game) error {
	return game.Upsert(ctx, r.db, true, []string{"id"}, boil.Infer(), boil.Infer())
}

func (r *gameRepo) Delete(ctx context.Context, id int) error {
	rowsAff, err := models.Games(qm.Where("id = ?", id)).DeleteAll(ctx, r.db)
	if err != nil {
		return err
	}
	if rowsAff == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *gameRepo) IsParticipant(ctx context.Context, gameID, userID int) (bool, error) {
	return models.GameParticipants(
		qm.Where("game_id = ? AND user_id = ?", gameID, userID),
	).Exists(ctx, r.db)
}
