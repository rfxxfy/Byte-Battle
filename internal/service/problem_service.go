package service

import "bytebattle/internal/problems"

type ProblemService struct {
	loader *problems.Loader
}

func NewProblemService(loader *problems.Loader) *ProblemService {
	return &ProblemService{loader: loader}
}

func (s *ProblemService) ListProblems() []*problems.Problem {
	return s.loader.List()
}

func (s *ProblemService) GetProblem(id string) (*problems.Problem, error) {
	return s.loader.Get(id)
}
