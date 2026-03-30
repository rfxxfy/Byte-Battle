package service

import (
	"context"
	"sync"
	"time"

	"bytebattle/internal/apierr"
	"bytebattle/internal/executor"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

type RateLimitConfig struct {
	Rate  rate.Limit
	Burst int
}

var DefaultRateLimitConfig = RateLimitConfig{
	Rate:  rate.Every(5 * time.Second),
	Burst: 10,
}

type slotKey struct {
	userID uuid.UUID
	kind   string // execute/submit
}

type ExecutionService struct {
	executor executor.Executor
	rl       RateLimitConfig
	limiters sync.Map // uuid.UUID -> *rate.Limiter
	slots    sync.Map // slotKey -> chan struct{} (per-user per-kind concurrency slot)
}

func NewExecutionService(exec executor.Executor, cfg ...RateLimitConfig) *ExecutionService {
	rl := DefaultRateLimitConfig
	if len(cfg) > 0 {
		rl = cfg[0]
	}
	svc := &ExecutionService{executor: exec, rl: rl}
	go svc.cleanupLoop()
	return svc
}

func (s *ExecutionService) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		s.runCleanup()
	}
}

func (s *ExecutionService) runCleanup() {
	s.limiters.Range(func(k, v any) bool {
		if v.(*rate.Limiter).Tokens() >= float64(s.rl.Burst) {
			s.limiters.Delete(k)
		}
		return true
	})
	s.slots.Range(func(k, v any) bool {
		userID := k.(slotKey).userID
		if _, hasLimiter := s.limiters.Load(userID); !hasLimiter {
			if len(v.(chan struct{})) == 0 {
				s.slots.Delete(k)
			}
		}
		return true
	})
}

func (s *ExecutionService) CheckRateLimit(userID uuid.UUID) error {
	if !s.limiter(userID).Allow() {
		return apierr.New(apierr.ErrExecutionRateLimited, "execution rate limit exceeded")
	}
	return nil
}

func (s *ExecutionService) IsReady() bool {
	return s.executor.IsReady()
}

func (s *ExecutionService) Executor() executor.Executor {
	return s.executor
}

func (s *ExecutionService) Execute(ctx context.Context, req executor.ExecutionRequest) (executor.ExecutionResult, error) {
	return s.executor.Run(ctx, req)
}

func (s *ExecutionService) TryAcquireSlot(userID uuid.UUID, kind string) bool {
	k := slotKey{userID, kind}
	ch, _ := s.slots.LoadOrStore(k, make(chan struct{}, 1))
	select {
	case ch.(chan struct{}) <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *ExecutionService) ReleaseSlot(userID uuid.UUID, kind string) {
	if ch, ok := s.slots.Load(slotKey{userID, kind}); ok {
		<-ch.(chan struct{})
	}
}

func (s *ExecutionService) limiter(userID uuid.UUID) *rate.Limiter {
	v, _ := s.limiters.LoadOrStore(userID, rate.NewLimiter(s.rl.Rate, s.rl.Burst))
	return v.(*rate.Limiter)
}
