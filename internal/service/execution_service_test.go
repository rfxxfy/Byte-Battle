package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"bytebattle/internal/apierr"
	"bytebattle/internal/executor"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// stubExecutor satisfies executor.Executor without running real code.
type stubExecutor struct{}

func (stubExecutor) Run(_ context.Context, _ executor.ExecutionRequest) (executor.ExecutionResult, error) {
	return executor.ExecutionResult{Stdout: "ok"}, nil
}

func newTestService(r rate.Limit, burst int) *ExecutionService {
	return NewExecutionService(stubExecutor{}, RateLimitConfig{Rate: r, Burst: burst})
}

// --- CheckRateLimit ---

func TestExecutionService_CheckRateLimit_AllowsWithinBurst(t *testing.T) {
	svc := newTestService(rate.Every(time.Hour), 3)
	userID := uuid.New()

	for i := range 3 {
		require.NoError(t, svc.CheckRateLimit(userID), "request %d should be allowed", i+1)
	}
}

func TestExecutionService_CheckRateLimit_BlocksAfterBurst(t *testing.T) {
	svc := newTestService(rate.Every(time.Hour), 2)
	userID := uuid.New()

	require.NoError(t, svc.CheckRateLimit(userID))
	require.NoError(t, svc.CheckRateLimit(userID))

	err := svc.CheckRateLimit(userID)
	require.Error(t, err)
	var appErr *apierr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apierr.ErrExecutionRateLimited, appErr.ErrorCode)
}

func TestExecutionService_CheckRateLimit_IndependentPerUser(t *testing.T) {
	svc := newTestService(rate.Every(time.Hour), 1)
	user1, user2 := uuid.New(), uuid.New()

	require.NoError(t, svc.CheckRateLimit(user1))
	require.Error(t, svc.CheckRateLimit(user1))
	require.NoError(t, svc.CheckRateLimit(user2), "user2 should have its own bucket")
}

// --- TryAcquireSlot / ReleaseSlot ---

func TestExecutionService_TryAcquireSlot_OnlyOneAtATime(t *testing.T) {
	svc := newTestService(rate.Inf, 100)
	userID := uuid.New()

	assert.True(t, svc.TryAcquireSlot(userID))
	assert.False(t, svc.TryAcquireSlot(userID), "second acquire should fail while slot is held")

	svc.ReleaseSlot(userID)
	assert.True(t, svc.TryAcquireSlot(userID), "acquire should succeed after release")
}

func TestExecutionService_TryAcquireSlot_IndependentPerUser(t *testing.T) {
	svc := newTestService(rate.Inf, 100)
	user1, user2 := uuid.New(), uuid.New()

	assert.True(t, svc.TryAcquireSlot(user1))
	assert.True(t, svc.TryAcquireSlot(user2), "user2 slot should be independent of user1")
	assert.False(t, svc.TryAcquireSlot(user1))
}

func TestExecutionService_ReleaseSlot_NoopWhenNotHeld(t *testing.T) {
	svc := newTestService(rate.Inf, 100)
	// should not panic or block when no slot was ever acquired
	svc.ReleaseSlot(uuid.New())
}

// TestExecutionService_SlotConcurrency verifies the core invariant: at most one
// goroutine holds the slot for a given user at any point in time.
func TestExecutionService_SlotConcurrency(t *testing.T) {
	svc := newTestService(rate.Inf, 1000)
	userID := uuid.New()

	const goroutines = 50
	var concurrent atomic.Int32 // current concurrent holders
	var maxConcurrent atomic.Int32
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			if !svc.TryAcquireSlot(userID) {
				return
			}
			cur := concurrent.Add(1)
			// record peak
			for {
				old := maxConcurrent.Load()
				if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			concurrent.Add(-1)
			svc.ReleaseSlot(userID)
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), maxConcurrent.Load(), "at most one goroutine must hold the slot at any time")
}

// --- Execute ---

func TestExecutionService_Execute(t *testing.T) {
	svc := newTestService(rate.Inf, 1)
	result, err := svc.Execute(context.Background(), executor.ExecutionRequest{
		Code:     "print('hi')",
		Language: executor.Language("python"),
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", result.Stdout)
}

// --- runCleanup ---

func TestExecutionService_Cleanup_RemovesIdleEntriesWhenLimiterReplenished(t *testing.T) {
	svc := &ExecutionService{
		executor: stubExecutor{},
		rl:       RateLimitConfig{Rate: rate.Every(time.Millisecond), Burst: 1},
	}
	userID := uuid.New()

	_ = svc.limiter(userID) // create limiter entry
	svc.TryAcquireSlot(userID)
	svc.ReleaseSlot(userID)

	time.Sleep(5 * time.Millisecond) // let limiter replenish
	svc.runCleanup()

	_, limiterExists := svc.limiters.Load(userID)
	_, slotExists := svc.slots.Load(userID)
	assert.False(t, limiterExists, "replenished limiter should be removed")
	assert.False(t, slotExists, "idle slot should be removed")
}

// TestExecutionService_Cleanup_PreservesSlotWhenExecutionInFlight checks that
// a slot held during cleanup is not deleted, even when the limiter is replenished.
func TestExecutionService_Cleanup_PreservesSlotWhenExecutionInFlight(t *testing.T) {
	svc := &ExecutionService{
		executor: stubExecutor{},
		rl:       RateLimitConfig{Rate: rate.Every(time.Millisecond), Burst: 1},
	}
	userID := uuid.New()

	_ = svc.limiter(userID)
	svc.TryAcquireSlot(userID) // slot is now held (not released)

	time.Sleep(5 * time.Millisecond) // limiter replenishes
	svc.runCleanup()

	_, slotExists := svc.slots.Load(userID)
	assert.True(t, slotExists, "slot must be preserved while execution is in flight")
}

// TestExecutionService_Cleanup_RemovesOrphanedSlot covers the bug scenario:
// limiter was removed in a prior cleanup while slot was in flight;
// after execution finishes the slot becomes orphaned — next cleanup must collect it.
func TestExecutionService_Cleanup_RemovesOrphanedSlot(t *testing.T) {
	svc := &ExecutionService{
		executor: stubExecutor{},
		rl:       RateLimitConfig{Rate: rate.Every(time.Millisecond), Burst: 1},
	}
	userID := uuid.New()

	_ = svc.limiter(userID)
	svc.TryAcquireSlot(userID) // slot held during first cleanup

	time.Sleep(5 * time.Millisecond) // limiter replenishes
	svc.runCleanup()                 // limiter deleted, slot preserved (still in flight)

	_, limiterExists := svc.limiters.Load(userID)
	_, slotExists := svc.slots.Load(userID)
	require.False(t, limiterExists, "limiter should be gone after first cleanup")
	require.True(t, slotExists, "slot should survive while in flight")

	svc.ReleaseSlot(userID) // execution finishes
	svc.runCleanup()        // second cleanup: orphaned slot should now be removed

	_, slotExists = svc.slots.Load(userID)
	assert.False(t, slotExists, "orphaned slot must be removed on second cleanup")
}
