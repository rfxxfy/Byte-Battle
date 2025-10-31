package database

import (
	"context"
	"database/sql"
	"fmt"

	"bytebattle/internal/database/models"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	. "github.com/aarondl/sqlboiler/v4/queries/qm"
)

// CreateUser создает нового пользователя в базе данных
func (c *Client) CreateUser(ctx context.Context, username, email, passwordHash string) (*models.User, error) {
	user := &models.User{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
	}

	err := user.Insert(ctx, c.DB, boil.Infer())
	if err != nil {
		return nil, fmt.Errorf("не удалось создать пользователя: %w", err)
	}

	return user, nil
}

// GetUserByUsername получает пользователя по его имени пользователя
func (c *Client) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	user, err := models.Users(
		Where("username = ?", username),
	).One(ctx, c.DB)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить пользователя по имени: %w", err)
	}

	return user, nil
}

// CreateProblem создает новую задачу в базе данных
func (c *Client) CreateProblem(ctx context.Context, title, description, difficulty string, timeLimit, memoryLimit int) (*models.Problem, error) {
	problem := &models.Problem{
		Title:       title,
		Description: description,
		Difficulty:  difficulty,
		TimeLimit:   timeLimit,
		MemoryLimit: memoryLimit,
	}

	err := problem.Insert(ctx, c.DB, boil.Infer())
	if err != nil {
		return nil, fmt.Errorf("не удалось создать задачу: %w", err)
	}

	return problem, nil
}

// GetProblems получает все задачи из базы данных
func (c *Client) GetProblems(ctx context.Context) (models.ProblemSlice, error) {
	problems, err := models.Problems().All(ctx, c.DB)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить задачи: %w", err)
	}

	return problems, nil
}

// CreateDuel создает новую дуэль в базе данных
func (c *Client) CreateDuel(ctx context.Context, player1ID, player2ID, problemID int) (*models.Duel, error) {
	duel := &models.Duel{
		Player1ID: player1ID,
		Player2ID: player2ID,
		ProblemID: problemID,
		Status:    "pending",
	}

	err := duel.Insert(ctx, c.DB, boil.Infer())
	if err != nil {
		return nil, fmt.Errorf("не удалось создать дуэль: %w", err)
	}

	return duel, nil
}

// UpdateDuelStatus обновляет статус дуэли
func (c *Client) UpdateDuelStatus(ctx context.Context, duelID int, status string) error {
	duel, err := models.FindDuel(ctx, c.DB, duelID)
	if err != nil {
		return fmt.Errorf("не удалось найти дуэль: %w", err)
	}

	duel.Status = status
	_, err = duel.Update(ctx, c.DB, boil.Infer())
	if err != nil {
		return fmt.Errorf("не удалось обновить статус дуэли: %w", err)
	}

	return nil
}

// CreateSolution создает новое решение в базе данных
func (c *Client) CreateSolution(ctx context.Context, userID, problemID int, code, language string) (*models.Solution, error) {
	solution := &models.Solution{
		UserID:    userID,
		ProblemID: problemID,
		Code:      code,
		Language:  language,
		Status:    "pending",
	}

	err := solution.Insert(ctx, c.DB, boil.Infer())
	if err != nil {
		return nil, fmt.Errorf("не удалось создать решение: %w", err)
	}

	return solution, nil
}

// UpdateSolutionStatus обновляет статус решения
func (c *Client) UpdateSolutionStatus(ctx context.Context, solutionID int, status string, executionTime, memoryUsed *int) error {
	solution, err := models.FindSolution(ctx, c.DB, solutionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("решение не найдено: %w", err)
		}
		return fmt.Errorf("не удалось найти решение: %w", err)
	}

	solution.Status = status
	if executionTime != nil {
		solution.ExecutionTime = null.IntFrom(*executionTime)
	}
	if memoryUsed != nil {
		solution.MemoryUsed = null.IntFrom(*memoryUsed)
	}

	_, err = solution.Update(ctx, c.DB, boil.Infer())
	if err != nil {
		return fmt.Errorf("не удалось обновить статус решения: %w", err)
	}

	return nil
}
