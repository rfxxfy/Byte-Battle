package database

import (
	"context"
	"database/sql"

	"bytebattle/internal/database/models"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
)

type DuelRepository struct {
	db *sql.DB
}

func NewDuelRepository(db *sql.DB) *DuelRepository {
	return &DuelRepository{db: db}
}

func (r *DuelRepository) Create(ctx context.Context, player1ID, player2ID, problemID int) (*models.Duel, error) {
	duel := &models.Duel{
		Player1ID: player1ID,
		Player2ID: player2ID,
		ProblemID: problemID,
		Status:    "pending",
	}

	err := duel.Insert(ctx, r.db, boil.Infer())
	if err != nil {
		return nil, err
	}

	return duel, nil
}

func (r *DuelRepository) GetByID(ctx context.Context, id int) (*models.Duel, error) {
	return models.FindDuel(ctx, r.db, id)
}

func (r *DuelRepository) GetAll(ctx context.Context, limit, offset int) (models.DuelSlice, error) {
	return models.Duels(
		qm.Limit(limit),
		qm.Offset(offset),
		qm.OrderBy("created_at DESC"),
	).All(ctx, r.db)
}

func (r *DuelRepository) Update(ctx context.Context, duel *models.Duel) error {
	_, err := duel.Update(ctx, r.db, boil.Infer())
	return err
}

func (r *DuelRepository) Delete(ctx context.Context, id int) error {
	duel, err := models.FindDuel(ctx, r.db, id)
	if err != nil {
		return err
	}

	_, err = duel.Delete(ctx, r.db)
	return err
}

func (r *DuelRepository) GetByPlayerID(ctx context.Context, playerID int) (models.DuelSlice, error) {
	return models.Duels(
		qm.Where("player1_id = ? OR player2_id = ?", playerID, playerID),
		qm.OrderBy("created_at DESC"),
	).All(ctx, r.db)
}

func (r *DuelRepository) GetByStatus(ctx context.Context, status string) (models.DuelSlice, error) {
	return models.Duels(
		qm.Where("status = ?", status),
		qm.OrderBy("created_at DESC"),
	).All(ctx, r.db)
}
