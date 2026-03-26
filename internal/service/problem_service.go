package service

import (
	"context"
	"log"

	sqlcdb "bytebattle/internal/db/sqlc"
	"bytebattle/internal/problems"
)

type ProblemService struct {
	store *problems.Store
	q     sqlcdb.Querier
}

func NewProblemService(store *problems.Store, q sqlcdb.Querier) *ProblemService {
	return &ProblemService{store: store, q: q}
}

func (s *ProblemService) ListProblems(ctx context.Context) ([]*problems.Problem, error) {
	rows, err := s.q.ListPublishedPublicProblemsWithArtifact(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*problems.Problem, 0, len(rows))
	for i := range rows {
		p, err := s.store.GetByPath(rows[i].ArtifactPath)
		if err != nil {
			log.Printf("warn: problem %q in DB but files missing (artifact=%s): %v", rows[i].Slug, rows[i].ArtifactPath, err)
			continue
		}
		result = append(result, p)
	}
	return result, nil
}

func (s *ProblemService) GetProblem(ctx context.Context, slug string) (*problems.Problem, error) {
	row, err := s.q.GetProblemWithArtifactBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	return s.store.GetByPath(row.ArtifactPath)
}
