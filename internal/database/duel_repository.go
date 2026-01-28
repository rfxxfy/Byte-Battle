package database

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"bytebattle/internal/database/models"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
)

type DuelStatus string

const (
	DuelStatusPending  DuelStatus = "pending"
	DuelStatusActive   DuelStatus = "active"
	DuelStatusFinished DuelStatus = "finished"
)

type Player struct {
	ID int
}

type DuelRepository struct {
	db *sql.DB
}

func NewDuelRepository(db *sql.DB) *DuelRepository {
	return &DuelRepository{db: db}
}

func (r *DuelRepository) Create(ctx context.Context, players []Player, problemID int) (*models.Duel, error) {
	if len(players) != 2 {
		return nil, fmt.Errorf("exactly 2 players required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	duel := &models.Duel{
		Player1ID: players[0].ID,
		Player2ID: players[1].ID,
		ProblemID: problemID,
		Status:    string(DuelStatusPending),
	}

	err = duel.Insert(ctx, tx, boil.Infer())
	if err != nil {
		return nil, err
	}

	return duel, tx.Commit()
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

func (r *DuelRepository) Upsert(ctx context.Context, duel *models.Duel) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = duel.Upsert(ctx, tx, true, []string{"id"}, boil.Infer(), boil.Infer())
	if err != nil {
		return err
	}

	return tx.Commit()
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
	asPlayer1, err := models.Duels(
		qm.Where("player1_id = ?", playerID),
	).All(ctx, r.db)
	if err != nil {
		return nil, err
	}

	asPlayer2, err := models.Duels(
		qm.Where("player2_id = ?", playerID),
	).All(ctx, r.db)
	if err != nil {
		return nil, err
	}

	seen := make(map[int]struct{})
	result := make(models.DuelSlice, 0, len(asPlayer1)+len(asPlayer2))

	for _, d := range asPlayer1 {
		seen[d.ID] = struct{}{}
		result = append(result, d)
	}

	for _, d := range asPlayer2 {
		if _, exists := seen[d.ID]; !exists {
			result = append(result, d)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Time.After(result[j].CreatedAt.Time)
	})

	return result, nil
}

func (r *DuelRepository) GetByStatus(ctx context.Context, status DuelStatus) (models.DuelSlice, error) {
	return models.Duels(
		qm.Where("status = ?", string(status)),
		qm.OrderBy("created_at DESC"),
	).All(ctx, r.db)
}
