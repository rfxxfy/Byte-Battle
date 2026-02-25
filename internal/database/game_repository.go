package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"bytebattle/internal/database/models"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
)

const (
	GameStatusPending   = "pending"
	GameStatusActive    = "active"
	GameStatusFinished  = "finished"
	GameStatusCancelled = "cancelled"
)

type Player struct {
	ID int
}

type IGameRepo interface {
	Create(ctx context.Context, players []Player, problemID int) (*models.Game, error)
	GetByID(ctx context.Context, id int) (*models.Game, error)
	GetAll(ctx context.Context, limit, offset int) (models.GameSlice, error)
	Count(ctx context.Context) (int64, error)
	StartGame(ctx context.Context, id int) (*models.Game, error)
	CompleteGame(ctx context.Context, id, winnerID int) (*models.Game, error)
	CancelGame(ctx context.Context, id int) (*models.Game, error)
	Delete(ctx context.Context, id int) error
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
		Status:    GameStatusPending,
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

func (r *gameRepo) Count(ctx context.Context) (int64, error) {
	return models.Games().Count(ctx, r.db)
}

func (r *gameRepo) StartGame(ctx context.Context, id int) (*models.Game, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	game, err := models.Games(qm.Where("id = ?", id), qm.For("UPDATE")).One(ctx, tx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if game.Status != GameStatusPending {
		return nil, ErrGameNotPending
	}

	game.Status = GameStatusActive
	game.StartedAt = null.TimeFrom(time.Now())
	game.UpdatedAt = null.TimeFrom(time.Now())

	if _, err = game.Update(ctx, tx, boil.Infer()); err != nil {
		return nil, err
	}

	return game, tx.Commit()
}

func (r *gameRepo) CompleteGame(ctx context.Context, id, winnerID int) (*models.Game, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	game, err := models.Games(qm.Where("id = ?", id), qm.For("UPDATE")).One(ctx, tx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if game.Status != GameStatusActive {
		return nil, ErrGameNotActive
	}

	ok, err := models.GameParticipants(
		qm.Where("game_id = ? AND user_id = ?", id, winnerID),
	).Exists(ctx, tx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotParticipant
	}

	game.Status = GameStatusFinished
	game.WinnerID = null.IntFrom(winnerID)
	game.CompletedAt = null.TimeFrom(time.Now())
	game.UpdatedAt = null.TimeFrom(time.Now())

	if _, err = game.Update(ctx, tx, boil.Infer()); err != nil {
		return nil, err
	}

	return game, tx.Commit()
}

func (r *gameRepo) CancelGame(ctx context.Context, id int) (*models.Game, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	game, err := models.Games(qm.Where("id = ?", id), qm.For("UPDATE")).One(ctx, tx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if game.Status == GameStatusFinished {
		return nil, ErrGameFinished
	}
	if game.Status == GameStatusCancelled {
		return nil, ErrGameAlreadyCancelled
	}

	game.Status = GameStatusCancelled
	game.UpdatedAt = null.TimeFrom(time.Now())

	if _, err = game.Update(ctx, tx, boil.Infer()); err != nil {
		return nil, err
	}

	return game, tx.Commit()
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
