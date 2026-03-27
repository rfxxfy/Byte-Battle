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
	Rate:  rate.Every(10 * time.Second),
	Burst: 3,
}

type ExecutionService struct {
	executor executor.Executor
	rl       RateLimitConfig
	limiters sync.Map // uuid.UUID -> *rate.Limiter
	slots    sync.Map // uuid.UUID -> chan struct{} (per-user concurrency slot)
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

// cleanupLoop periodically removes stale per-user entries from limiters and slots maps.
// An entry is considered stale when the limiter is fully replenished (user has been
// idle long enough for all tokens to recover) and no execution slot is held.
func (s *ExecutionService) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		s.limiters.Range(func(k, v any) bool {
			if v.(*rate.Limiter).Tokens() >= float64(s.rl.Burst) {
				s.limiters.Delete(k)
				// Only delete the slot if no execution is in progress.
				if ch, ok := s.slots.Load(k); ok && len(ch.(chan struct{})) == 0 {
					s.slots.Delete(k)
				}
			}
			return true
		})
	}
}

func (s *ExecutionService) CheckRateLimit(userID uuid.UUID) error {
	if !s.limiter(userID).Allow() {
		return apierr.New(apierr.ErrExecutionRateLimited, "execution rate limit exceeded")
	}
	return nil
}

func (s *ExecutionService) Execute(ctx context.Context, req executor.ExecutionRequest) (executor.ExecutionResult, error) {
	return s.executor.Run(ctx, req)
}

func (s *ExecutionService) TryAcquireSlot(userID uuid.UUID) bool {
	ch, _ := s.slots.LoadOrStore(userID, make(chan struct{}, 1))
	select {
	case ch.(chan struct{}) <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *ExecutionService) ReleaseSlot(userID uuid.UUID) {
	if ch, ok := s.slots.Load(userID); ok {
		<-ch.(chan struct{})
	}
}

func (s *ExecutionService) limiter(userID uuid.UUID) *rate.Limiter {
	v, _ := s.limiters.LoadOrStore(userID, rate.NewLimiter(s.rl.Rate, s.rl.Burst))
	return v.(*rate.Limiter)
}
