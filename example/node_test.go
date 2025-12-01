package group

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	. "github.com/oatcatx/group"
)

func TestGroupGoNodeFFAndBlock(t *testing.T) {
	t.Parallel()

	t.Run("non-fast-fail", func(t *testing.T) {
		t.Parallel()
		ctx, c, s := context.Background(), new(exampleCtx), time.Now()

		err := NewGroup().
			AddRunner(c.F).Key("f"). // non-fast-fail by default
			AddRunner(c.C).Key("c"). // C will execute for 2 seconds
			AddRunner(c.X).Dep("c"). // X will be executed
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, float64(3), time.Since(s).Truncate(time.Second).Seconds())
		assert.Equal(t, 1, c.x)
	})

	t.Run("fast-fail", func(t *testing.T) {
		t.Parallel()
		ctx, c, s := context.Background(), new(exampleCtx), time.Now()

		err := NewGroup().
			AddRunner(c.F).Key("f").FastFail(). // fast-fail
			AddRunner(c.C).Key("c").            // C will execute for 2 seconds
			AddRunner(c.X).Dep("c").            // X will be blocked since fast-fail error occurs
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, float64(2), time.Since(s).Truncate(time.Second).Seconds())
		assert.Equal(t, 0, c.x)
	})
}

func TestGroupGoNodeRetry(t *testing.T) {
	t.Parallel()

	t.Run("retry success after failures", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		var attempts int
		failTwice := func() error {
			attempts++
			if attempts < 3 {
				return fmt.Errorf("attempt %d failed", attempts)
			}
			return nil
		}

		err := NewGroup().
			AddRunner(failTwice).Key("retry").WithRetry(2).
			Go(ctx)

		assert.Nil(t, err)
		assert.Equal(t, 3, attempts) // failed twice, succeeded on third attempt
	})

	t.Run("retry exhausted", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		var attempts int
		alwaysFail := func() error {
			attempts++
			return fmt.Errorf("attempt %d failed", attempts)
		}

		err := NewGroup().
			AddRunner(alwaysFail).Key("retry").WithRetry(2).
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, err.Error(), "attempt 3 failed")
		assert.Equal(t, 3, attempts) // initial attempt + 2 retries = 3 total
	})

	t.Run("retry with dependency", func(t *testing.T) {
		t.Parallel()
		ctx, c, s := context.Background(), new(exampleCtx), time.Now()

		var attempts int
		failOnce := func() error {
			attempts++
			if attempts == 1 {
				return errors.New("first attempt failed")
			}
			return c.B()
		}

		err := NewGroup().
			AddRunner(c.A).Key("a").
			AddRunner(failOnce).Key("b").Dep("a").WithRetry(1).
			AddRunner(c.D).Key("d").Dep("b").
			Go(ctx)

		assert.Nil(t, err)
		assert.Equal(t, 2, attempts) // failed once, succeeded on retry
		assert.Equal(t, 2, c.b)
		assert.Equal(t, float64(3), time.Since(s).Truncate(time.Second).Seconds()) // elapsed = A(1s) + B retry(1s+1s) = 3s
	})

	t.Run("retry blocks downstream until success", func(t *testing.T) {
		t.Parallel()
		ctx, c, s := context.Background(), new(exampleCtx), time.Now()

		var attempts int
		retryFunc := func() error {
			attempts++
			time.Sleep(500 * time.Millisecond)
			if attempts < 3 {
				return fmt.Errorf("attempt %d failed", attempts)
			}
			c.c = c.a + 1
			return nil
		}

		err := NewGroup().
			AddRunner(c.A).Key("a").
			AddRunner(retryFunc).Key("c").Dep("a").WithRetry(3).
			AddRunner(c.D).Key("d").Dep("c").
			Go(ctx)

		assert.Nil(t, err)
		assert.Equal(t, 3, attempts)
		assert.Equal(t, 2, c.c)
		assert.Equal(t, float64(3), time.Since(s).Truncate(time.Second).Seconds()) // elapsed = A(1s) + C retry(0.5s+0.5s*3) + D(1s) = 3s
	})

	t.Run("retry with weak dependency", func(t *testing.T) {
		t.Parallel()
		ctx, c, s := context.Background(), new(exampleCtx), time.Now()

		alwaysFail := func() error {
			time.Sleep(500 * time.Millisecond)
			return errors.New("always fails")
		}

		err := NewGroup().
			AddRunner(alwaysFail).Key("fail").WithRetry(2).
			AddRunner(c.X).Key("x").WeakDep("fail").
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, 1, c.x)                                                    // X should execute
		assert.Equal(t, float64(2), time.Since(s).Truncate(time.Second).Seconds()) // elapsed = fail retry(0.5s+0.5s*2) + X(1s) = 2s
	})

	t.Run("multiple nodes with retry", func(t *testing.T) {
		t.Parallel()
		ctx, s := context.Background(), time.Now()

		var attemptsB, attemptsC int
		retryB := func() error {
			attemptsB++
			time.Sleep(300 * time.Millisecond)
			if attemptsB < 2 {
				return errors.New("B failed")
			}
			return nil
		}
		retryC := func() error {
			attemptsC++
			time.Sleep(250 * time.Millisecond)
			if attemptsC < 4 {
				return errors.New("C failed")
			}
			return nil
		}

		err := NewGroup().
			AddRunner(retryB).Key("b").WithRetry(1).
			AddRunner(retryC).Key("c").WithRetry(3).
			Go(ctx)

		assert.Nil(t, err)
		assert.Equal(t, 2, attemptsB)
		assert.Equal(t, 4, attemptsC)
		assert.Equal(t, float64(1), time.Since(s).Truncate(time.Second).Seconds()) // max(B retry(0.3s+0.3s), C(0.25s + 0.25s*3)) = 1s
	})
}

func TestGroupGoNodeInterceptor(t *testing.T) {
	t.Parallel()

	t.Run("node pre and after both run", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		var preRan, afterRan bool
		var capturedErr error
		var capturedShared any

		err := NewGroup().
			AddRunner(func() error { return nil }).Key("a").
			WithPreFunc(func(ctx context.Context, shared any) error {
				preRan = true
				capturedShared = shared
				return nil
			}).
			WithAfterFunc(func(ctx context.Context, shared any, err error) error {
				afterRan = true
				capturedErr = err
				return err
			}).
			AddRunner(func() error { return errors.New("task error") }).Key("b").
			Go(ctx)

		assert.NotNil(t, err)
		assert.True(t, preRan)
		assert.True(t, afterRan)
		assert.Nil(t, capturedErr) // node a succeeded
		assert.Nil(t, capturedShared)
	})

	t.Run("node pre fail blocks execution", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		var executed bool

		err := NewGroup().
			AddRunner(func() error { executed = true; return nil }).Key("a").
			WithPreFunc(func(ctx context.Context, shared any) error {
				return errors.New("pre failed")
			}).
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, "pre failed", err.Error())
		assert.False(t, executed)
	})

	t.Run("node after suppresses error", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		err := NewGroup().
			AddRunner(func() error { return errors.New("original") }).Key("a").
			WithAfterFunc(func(ctx context.Context, shared any, err error) error {
				return nil // suppress error
			}).
			Go(ctx)

		assert.Nil(t, err)
	})

	t.Run("node after wraps error", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		err := NewGroup().
			AddRunner(func() error { return errors.New("original") }).Key("a").
			WithAfterFunc(func(ctx context.Context, shared any, err error) error {
				if err != nil {
					return errors.New("wrapped: " + err.Error())
				}
				return nil
			}).
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, "wrapped: original", err.Error())
	})

	t.Run("node interceptor with shared state", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		shared := &sharedUnit{mark: "INIT"}
		var preShared, afterShared any

		err := NewGroup().
			AddSharedTask(func(ctx context.Context, s any) error {
				if unit, ok := s.(*sharedUnit); ok {
					unit.Lock()
					unit.mark = "MODIFIED"
					unit.Unlock()
				}
				return nil
			}).Key("a").
			WithPreFunc(func(ctx context.Context, s any) error {
				preShared = s
				return nil
			}).
			WithAfterFunc(func(ctx context.Context, s any, err error) error {
				afterShared = s
				return err
			}).
			Go(ctx, shared)

		assert.Nil(t, err)
		assert.Equal(t, shared, preShared)
		assert.Equal(t, shared, afterShared)
		assert.Equal(t, "MODIFIED", shared.mark)
	})

	t.Run("node pre with dependency", func(t *testing.T) {
		t.Parallel()
		ctx, c, s := context.Background(), new(exampleCtx), time.Now()
		var preRanA, preRanB bool

		err := NewGroup().
			AddRunner(c.A).Key("a").
			WithPreFunc(func(ctx context.Context, shared any) error {
				preRanA = true
				return nil
			}).
			AddRunner(c.B).Key("b").Dep("a").
			WithPreFunc(func(ctx context.Context, shared any) error {
				preRanB = true
				return nil
			}).
			Go(ctx)

		assert.Nil(t, err)
		assert.True(t, preRanA)
		assert.True(t, preRanB)
		assert.Equal(t, 2, c.b)
		assert.Equal(t, float64(2), time.Since(s).Truncate(time.Second).Seconds())
	})

	t.Run("node after runs even on group timeout", func(t *testing.T) {
		t.Parallel()
		ctx, c := context.Background(), new(exampleCtx)
		var afterRan bool
		var capturedErr error
		var mu sync.Mutex

		err := NewGroup(WithTimeout(1 * time.Second)).
			AddRunner(c.C).Key("c"). // C takes 2s but timeout at 1s
			WithAfterFunc(func(ctx context.Context, shared any, err error) error {
				mu.Lock()
				afterRan = true
				capturedErr = err
				mu.Unlock()
				return err
			}).
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, "group anonymous timeout", err.Error())

		// wait for node execution to finish (group will early return on timeout)
		time.Sleep(2 * time.Second)

		mu.Lock()
		assert.True(t, afterRan)
		assert.Nil(t, capturedErr) // group level timeout error will not be captured in node
		mu.Unlock()
	})

	t.Run("node after runs with node timeout", func(t *testing.T) {
		t.Parallel()
		ctx, c := context.Background(), new(exampleCtx)
		var afterRan bool
		var capturedErr error

		err := NewGroup().
			AddRunner(c.C).Key("c").WithTimeout(1 * time.Second). // C takes 2s but timeout at 1s
			WithAfterFunc(func(ctx context.Context, shared any, err error) error {
				afterRan = true
				capturedErr = err
				return err
			}).
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, "node c timeout", err.Error())
		assert.True(t, afterRan)
		assert.NotNil(t, capturedErr) // will capture timeout error
	})

	t.Run("node interceptor with retry", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		var attempts int
		var preCount, afterCount int

		failTwice := func() error {
			attempts++
			if attempts < 3 {
				return fmt.Errorf("attempt %d failed", attempts)
			}
			return nil
		}

		err := NewGroup().
			AddRunner(failTwice).Key("retry").WithRetry(2).
			WithPreFunc(func(ctx context.Context, shared any) error {
				preCount++
				return nil
			}).
			WithAfterFunc(func(ctx context.Context, shared any, err error) error {
				afterCount++
				return err
			}).
			Go(ctx)

		assert.Nil(t, err)
		assert.Equal(t, 3, attempts)
		assert.Equal(t, 1, preCount)   // pre runs once before all retries
		assert.Equal(t, 1, afterCount) // after runs once after all retries
	})

	t.Run("multiple nodes with different interceptors", func(t *testing.T) {
		t.Parallel()
		ctx, c := context.Background(), new(exampleCtx)
		var preA, afterA, preB, afterB bool

		err := NewGroup().
			AddRunner(c.A).Key("a").
			WithPreFunc(func(ctx context.Context, shared any) error {
				preA = true
				return nil
			}).
			WithAfterFunc(func(ctx context.Context, shared any, err error) error {
				afterA = true
				return err
			}).
			AddRunner(c.B).Key("b").Dep("a").
			WithPreFunc(func(ctx context.Context, shared any) error {
				preB = true
				return nil
			}).
			WithAfterFunc(func(ctx context.Context, shared any, err error) error {
				afterB = true
				return err
			}).
			Go(ctx)

		assert.Nil(t, err)
		assert.True(t, preA)
		assert.True(t, afterA)
		assert.True(t, preB)
		assert.True(t, afterB)
		assert.Equal(t, 2, c.b)
	})

	t.Run("node pre fail blocks downstream", func(t *testing.T) {
		t.Parallel()
		ctx, c := context.Background(), new(exampleCtx)

		err := NewGroup().
			AddRunner(c.A).Key("a").
			WithPreFunc(func(ctx context.Context, shared any) error {
				return errors.New("pre a failed")
			}).
			AddRunner(c.B).Key("b").Dep("a").
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, "pre a failed", err.Error())
		assert.Equal(t, 0, c.b) // b should not execute
	})

	t.Run("node after modifies error for downstream", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		err := NewGroup().
			AddRunner(func() error { return errors.New("original") }).Key("a").
			WithAfterFunc(func(ctx context.Context, shared any, err error) error {
				if err != nil {
					return errors.New("modified: " + err.Error())
				}
				return nil
			}).
			AddRunner(func() error { return nil }).Key("b").WeakDep("a").
			Go(ctx)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "modified: original")
	})
}

func TestGroupGoNodeTimeout(t *testing.T) {
	t.Parallel()

	t.Run("node timeout blocks downstream", func(t *testing.T) {
		t.Parallel()
		ctx, c, s := context.Background(), new(exampleCtx), time.Now()

		err := NewGroup().
			AddRunner(c.A).Key("a").
			AddRunner(c.C).Key("c").Dep("a").WithTimeout(1 * time.Second). // C takes 2s but timeout at 1s
			AddRunner(c.D).Key("d").Dep("c").
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, "node c timeout", err.Error())
		assert.Equal(t, float64(2), time.Since(s).Truncate(time.Second).Seconds()) // elapsed = A(1s) + C timeout(1s) = 2s
		assert.Equal(t, 0, c.d)                                                    // D should not execute
	})

	t.Run("node timeout with weak dependency", func(t *testing.T) {
		t.Parallel()
		ctx, c, s := context.Background(), new(exampleCtx), time.Now()

		err := NewGroup().
			AddRunner(c.A).Key("a").
			AddRunner(c.C).Key("c").Dep("a").WithTimeout(1 * time.Second). // C takes 2s but timeout at 1s
			AddRunner(c.X).Key("x").WeakDep("c").                          // X should execute despite C timeout
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, "node c timeout", err.Error())
		assert.Equal(t, float64(3), time.Since(s).Truncate(time.Second).Seconds()) // elapsed = A(1s) + C timeout(1s) + X(1s) = 3s
		assert.Equal(t, 1, c.x)                                                    // X should execute
	})

	t.Run("multiple node timeouts", func(t *testing.T) {
		t.Parallel()
		ctx, c, s := context.Background(), new(exampleCtx), time.Now()

		err := NewGroup().
			AddRunner(c.B).Key("b").WithTimeout(500 * time.Millisecond). // B takes 1s but timeout at 500ms
			AddRunner(c.C).Key("c").WithTimeout(1 * time.Second).        // C takes 2s but timeout at 1s
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, errors.Join(
			errors.New("node b timeout"), errors.New("node c timeout"),
		).Error(), err.Error())
		assert.Equal(t, float64(1), time.Since(s).Truncate(time.Second).Seconds()) // timeout in parallel hence elapsed = max(1s, 500ms) = 1s
	})
}

func TestGroupGoNodeRollback(t *testing.T) {
	t.Parallel()

	t.Run("rollback runs on node failure", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		var rollbackRan bool
		var capturedErr error

		err := NewGroup().
			AddRunner(func() error { return errors.New("task failed") }).Key("a").
			WithRollback(func(ctx context.Context, shared any, err error) error {
				rollbackRan = true
				capturedErr = err
				return nil
			}).
			Go(ctx)

		assert.NotNil(t, err)
		assert.True(t, rollbackRan)
		assert.Equal(t, "task failed", capturedErr.Error())
	})

	t.Run("rollback does not run on success", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		var rollbackRan bool

		err := NewGroup().
			AddRunner(func() error { return nil }).Key("a").
			WithRollback(func(ctx context.Context, shared any, err error) error {
				rollbackRan = true
				return nil
			}).
			Go(ctx)

		assert.Nil(t, err)
		assert.False(t, rollbackRan)
	})

	t.Run("rollback with shared state", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		shared := &sharedUnit{mark: "INIT"}

		err := NewGroup().
			AddSharedTask(func(ctx context.Context, s any) error {
				if unit, ok := s.(*sharedUnit); ok {
					unit.Lock()
					unit.mark = "MODIFIED"
					unit.Unlock()
				}
				return errors.New("task failed")
			}).Key("a").
			WithRollback(func(ctx context.Context, s any, err error) error {
				if unit, ok := s.(*sharedUnit); ok {
					unit.Lock()
					unit.mark = "ROLLBACK"
					unit.Unlock()
				}
				return nil
			}).
			Go(ctx, shared)

		assert.NotNil(t, err)
		assert.Equal(t, "ROLLBACK", shared.mark)
	})

	t.Run("rollback error is combined with task error", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		err := NewGroup().
			AddRunner(func() error { return errors.New("task failed") }).Key("a").
			WithRollback(func(ctx context.Context, shared any, err error) error {
				return errors.New("rollback failed")
			}).
			Go(ctx)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "task failed")
		assert.Contains(t, err.Error(), "rollback failed")
	})

	t.Run("multiple nodes with rollback", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		var rollbackA, rollbackB, rollbackC bool

		err := NewGroup().
			AddRunner(func() error { return nil }).Key("a").
			WithRollback(func(ctx context.Context, shared any, err error) error {
				rollbackA = true
				return nil
			}).
			AddRunner(func() error { return errors.New("b failed") }).Key("b").
			WithRollback(func(ctx context.Context, shared any, err error) error {
				rollbackB = true
				return nil
			}).
			AddRunner(func() error { return errors.New("c failed") }).Key("c").
			WithRollback(func(ctx context.Context, shared any, err error) error {
				rollbackC = true
				return nil
			}).
			Go(ctx)

		assert.NotNil(t, err)
		assert.True(t, rollbackA)
		assert.True(t, rollbackB)
		assert.True(t, rollbackC)
	})

	t.Run("rollback with retry", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		var attempts int
		var rollbackCount int

		alwaysFail := func() error {
			attempts++
			return fmt.Errorf("attempt %d failed", attempts)
		}

		err := NewGroup().
			AddRunner(alwaysFail).Key("a").WithRetry(2).
			WithRollback(func(ctx context.Context, shared any, err error) error {
				rollbackCount++
				return nil
			}).
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, 3, attempts)      // initial + 2 retries
		assert.Equal(t, 1, rollbackCount) // rollback runs once after all retries exhausted
	})

	t.Run("not rollbacking when group timeout", func(t *testing.T) {
		t.Parallel()
		ctx, c := context.Background(), new(exampleCtx)
		var rollbackRan bool
		var capturedErr error

		err := NewGroup(WithTimeout(1 * time.Second)).
			AddRunner(c.C).Key("c"). // C takes 2s but timeout at 1s
			WithRollback(func(ctx context.Context, shared any, err error) error {
				rollbackRan = true
				capturedErr = err
				return nil
			}).
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, "group anonymous timeout", err.Error())
		assert.False(t, rollbackRan)
		assert.Nil(t, capturedErr) // timeout error happens at group level
	})

	t.Run("not rollbacking when group timeout", func(t *testing.T) {
		t.Parallel()
		ctx, c := context.Background(), new(exampleCtx)
		var rollbackRan bool
		var capturedErr error

		err := NewGroup(WithTimeout(1 * time.Second)).
			AddRunner(c.C).Key("c"). // C takes 2s but timeout at 1s
			WithRollback(func(ctx context.Context, shared any, err error) error {
				rollbackRan = true
				capturedErr = err
				return nil
			}).
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, "group anonymous timeout", err.Error())
		assert.False(t, rollbackRan) // rollback is tracked when node is done however group timeout occurs first
		assert.Nil(t, capturedErr)   // timeout error happens at group level
	})

	t.Run("rollback with node timeout", func(t *testing.T) {
		t.Parallel()
		ctx, c := context.Background(), new(exampleCtx)
		var rollbackRan bool
		var capturedErr error

		err := NewGroup().
			AddRunner(c.C).Key("c").WithTimeout(1 * time.Second). // C takes 2s but timeout at 1s
			WithRollback(func(ctx context.Context, shared any, err error) error {
				rollbackRan = true
				capturedErr = err
				return nil
			}).
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, "node c timeout", err.Error())

		// wait for rollback to execute
		time.Sleep(1 * time.Second)

		assert.True(t, rollbackRan)
		assert.NotNil(t, capturedErr)
	})

	t.Run("rollback with pre and after functions", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		var preRan, afterRan, rollbackRan bool

		err := NewGroup().
			AddRunner(func() error { return errors.New("task failed") }).Key("a").
			WithPreFunc(func(ctx context.Context, shared any) error {
				preRan = true
				return nil
			}).
			WithAfterFunc(func(ctx context.Context, shared any, err error) error {
				afterRan = true
				return err
			}).
			WithRollback(func(ctx context.Context, shared any, err error) error {
				rollbackRan = true
				return nil
			}).
			Go(ctx)

		assert.NotNil(t, err)
		assert.True(t, preRan)
		assert.True(t, afterRan)
		assert.True(t, rollbackRan)
	})

	t.Run("rollback after pre function failure", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		var rollbackRan bool
		var executed bool

		err := NewGroup().
			AddRunner(func() error { executed = true; return nil }).Key("a").
			WithPreFunc(func(ctx context.Context, shared any) error {
				return errors.New("pre failed")
			}).
			WithRollback(func(ctx context.Context, shared any, err error) error {
				rollbackRan = true
				return nil
			}).
			Go(ctx)

		assert.NotNil(t, err)
		assert.False(t, executed)
		assert.True(t, rollbackRan) // rollback runs even if pre fails
	})

	t.Run("rollback with weak dependency", func(t *testing.T) {
		t.Parallel()
		ctx, c := context.Background(), new(exampleCtx)
		var rollbackFail, rollbackX bool

		alwaysFail := func() error {
			time.Sleep(500 * time.Millisecond)
			return errors.New("always fails")
		}

		err := NewGroup().
			AddRunner(alwaysFail).Key("fail").
			WithRollback(func(ctx context.Context, shared any, err error) error {
				rollbackFail = true
				return nil
			}).
			AddRunner(c.X).Key("x").WeakDep("fail").
			WithRollback(func(ctx context.Context, shared any, err error) error {
				if err != nil {
					rollbackX = true // rollback only when x fails
				}
				return nil
			}).
			Go(ctx)

		assert.NotNil(t, err)
		assert.True(t, rollbackFail) // fail node should rollback
		assert.False(t, rollbackX)   // x succeeded, no rollback
	})
}
