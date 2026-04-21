package service

import (
	"context"
	"errors"
	"fmt"

	"bytebattle/internal/apierr"
	sqlcdb "bytebattle/internal/db/sqlc"
	"bytebattle/internal/executor"
	"bytebattle/internal/problems"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProblemService struct {
	store *problems.Store
	q     sqlcdb.Querier
	pool  *pgxpool.Pool
	exec  executor.Executor
}

func NewProblemService(store *problems.Store, q sqlcdb.Querier, pool *pgxpool.Pool, exec executor.Executor) *ProblemService {
	return &ProblemService{store: store, q: q, pool: pool, exec: exec}
}

func (s *ProblemService) ListProblems(ctx context.Context, q string, limit, offset int) ([]*problems.ProblemMeta, int64, error) {
	total, err := s.q.CountPublicProblems(ctx, q)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.q.ListPublicProblemsSearch(ctx, sqlcdb.ListPublicProblemsSearchParams{
		Limit:   int32(limit),
		Offset:  int32(offset),
		Column3: q,
	})
	if err != nil {
		return nil, 0, err
	}

	result := make([]*problems.ProblemMeta, 0, len(rows))
	for i := range rows {
		result = append(result, &problems.ProblemMeta{
			Slug:          rows[i].Slug,
			Title:         rows[i].Title,
			Difficulty:    rows[i].Difficulty,
			TimeLimitMs:   int(rows[i].LimitsTimeMs),
			MemoryLimitMb: int(rows[i].LimitsMemoryKb) / 1024,
			TestCaseCount: int(rows[i].TestCaseCount),
		})
	}
	return result, total, nil
}

func (s *ProblemService) GetProblem(ctx context.Context, slug string, requesterID uuid.UUID) (*problems.Problem, error) {
	catalog, err := s.q.GetProblemCatalogBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apierr.New(apierr.ErrProblemNotFound, "problem not found")
	}
	if err != nil {
		return nil, err
	}

	if catalog.Status != "published" {
		return nil, apierr.New(apierr.ErrProblemNotFound, "problem not found")
	}

	if catalog.Visibility == "private" {
		if !catalog.OwnerUserID.Valid || catalog.OwnerUserID.UUID != requesterID {
			return nil, apierr.New(apierr.ErrProblemNotFound, "problem not found")
		}
	}

	if !catalog.CurrentVersionID.Valid {
		return nil, apierr.New(apierr.ErrProblemNotFound, "problem has no published version")
	}

	row, err := s.q.GetProblemWithArtifactBySlug(ctx, slug)
	if err != nil {
		return nil, apierr.New(apierr.ErrProblemNotFound, "problem not found")
	}
	return s.store.GetByPath(row.ArtifactPath)
}

type MyProblemRow struct {
	Slug       string
	Title      string
	Visibility string
	Status     string
	Version    *int32
}

func (s *ProblemService) ListMyProblems(ctx context.Context, ownerID uuid.UUID, q string) ([]MyProblemRow, error) {
	rows, err := s.q.ListMyProblems(ctx, sqlcdb.ListMyProblemsParams{
		OwnerUserID: uuid.NullUUID{UUID: ownerID, Valid: true},
		Column2:     q,
	})
	if err != nil {
		return nil, err
	}

	result := make([]MyProblemRow, len(rows))
	for i := range rows {
		result[i] = MyProblemRow{
			Slug:       rows[i].Slug,
			Title:      rows[i].Title,
			Visibility: rows[i].Visibility,
			Status:     rows[i].Status,
		}
		if rows[i].Version.Valid {
			v := rows[i].Version.Int32
			result[i].Version = &v
		}
	}
	return result, nil
}

func (s *ProblemService) UpdateVisibility(ctx context.Context, slug string, requesterID uuid.UUID, visibility string) error {
	catalog, err := s.q.GetProblemCatalogBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return apierr.New(apierr.ErrProblemNotFound, "problem not found")
	}
	if err != nil {
		return err
	}

	if !catalog.OwnerUserID.Valid || catalog.OwnerUserID.UUID != requesterID {
		return apierr.New(apierr.ErrNotProblemOwner, "not the owner of this problem")
	}

	return s.q.UpdateProblemVisibility(ctx, sqlcdb.UpdateProblemVisibilityParams{
		ID:         catalog.ID,
		Visibility: visibility,
	})
}

func (s *ProblemService) ValidateUploadBatch(ctx context.Context, validated []*problems.ValidatedProblem, ownerID uuid.UUID) error {
	newProblemCount := 0
	for _, vp := range validated {
		slug := vp.Manifest.Slug
		if slug == "" {
			newProblemCount++
			continue
		}
		catalog, err := s.q.GetProblemCatalogBySlug(ctx, slug)
		if errors.Is(err, pgx.ErrNoRows) {
			return apierr.New(apierr.ErrProblemNotFound, "problem \""+slug+"\" not found")
		}
		if err != nil {
			return err
		}
		if !catalog.OwnerUserID.Valid {
			return apierr.New(apierr.ErrNotProblemOwner, "built-in problem \""+slug+"\" cannot be updated via upload")
		}
		if catalog.OwnerUserID.UUID != ownerID {
			return apierr.New(apierr.ErrNotProblemOwner, "not the owner of problem \""+slug+"\"")
		}
		cnt, err := s.q.CountProblemVersions(ctx, catalog.ID)
		if err != nil {
			return err
		}
		if cnt >= problems.MaxVersionsPerProblem {
			return apierr.New(apierr.ErrVersionLimitReached, "problem \""+slug+"\" has reached the version limit")
		}
	}

	if newProblemCount > 0 {
		existing, err := s.q.CountUserProblems(ctx, uuid.NullUUID{UUID: ownerID, Valid: true})
		if err != nil {
			return err
		}
		if existing+int64(newProblemCount) > problems.MaxProblemsPerUser {
			return apierr.New(apierr.ErrProblemLimitReached, "upload would exceed the maximum of "+fmt.Sprintf("%d", problems.MaxProblemsPerUser)+" problems per user")
		}
	}
	return nil
}

func (s *ProblemService) UploadProblem(ctx context.Context, validated *problems.ValidatedProblem, ownerID uuid.UUID, visibility string) (*problems.UploadResult, error) {
	return problems.UploadProblem(ctx, s.pool, s.store, validated, ownerID, visibility)
}

func (s *ProblemService) UploadNewVersion(ctx context.Context, validated *problems.ValidatedProblem, slug string, ownerID uuid.UUID) (*problems.UploadResult, error) {
	catalog, err := s.q.GetProblemCatalogBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apierr.New(apierr.ErrProblemNotFound, "problem not found")
	}
	if err != nil {
		return nil, err
	}

	if !catalog.OwnerUserID.Valid {
		return nil, apierr.New(apierr.ErrNotProblemOwner, "built-in problems cannot be updated via upload")
	}

	if catalog.OwnerUserID.UUID != ownerID {
		return nil, apierr.New(apierr.ErrNotProblemOwner, "not the owner of this problem")
	}

	return problems.UploadNewVersion(ctx, s.pool, s.store, validated, slug, ownerID)
}

func (s *ProblemService) Executor() executor.Executor {
	return s.exec
}
