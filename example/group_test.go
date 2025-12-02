package group

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	. "github.com/oatcatx/group"
)

func TestGroupGo(t *testing.T) {
	t.Parallel()
	ctx, c, s := context.Background(), new(exampleCtx), time.Now()

	err := NewGroup().
		AddRunner(c.A).Key("a").
		AddRunner(c.B).Key("b").Dep("a").
		AddRunner(c.C).Key("c").Dep("a").
		AddRunner(c.D).Key("d").Dep("b", "c").
		AddRunner(c.X).Dep("a", "b", "c").
		Go(ctx)

	assert.Nil(t, err)
	assert.Equal(t, 4, c.Res())
	assert.Equal(t, float64(4), time.Since(s).Truncate(time.Second).Seconds())
}

func TestConcurrentGroupGo(t *testing.T) {
	t.Parallel()

	var a, b, c, d = func() error { return nil }, func() error { return nil }, func() error { return nil }, func() error { return nil }
	g := NewGroup().
		AddRunner(a).Key("a").
		AddRunner(b).Key("b").Dep("a").
		AddRunner(c).Key("c").Dep("a").
		AddRunner(d).Key("d").Dep("b", "c").Group

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		assert.Nil(t, g.Go(context.Background()))
	}()
	go func() {
		defer wg.Done()
		assert.Nil(t, g.Go(context.Background()))
	}()
	wg.Wait() // should not trigger data race unless a node modifies the global data
}

func TestGroupGoLimit(t *testing.T) {
	t.Parallel()
	ctx, c, s := context.Background(), new(exampleCtx), time.Now()

	err := NewGroup(WithLimit(1)).
		AddRunner(c.A).
		AddRunner(c.X).
		Go(ctx)

	assert.Nil(t, err)
	assert.Equal(t, float64(2), time.Since(s).Truncate(time.Second).Seconds())
}

func TestGroupGoWithLog(t *testing.T) {
	t.Parallel()
	ctx, c, s := context.Background(), new(exampleCtx), time.Now()

	err := NewGroup(WithLog).
		AddRunner(c.A).Key("a").
		AddRunner(c.B).Key("b").Dep("a").
		AddRunner(c.C).Key("c").Dep("a").
		AddRunner(c.D).Key("d").Dep("b", "c").
		Go(ctx)

	assert.Nil(t, err)
	assert.Equal(t, 4, c.Res())
	assert.Equal(t, float64(4), time.Since(s).Truncate(time.Second).Seconds())
}

// region ERR CASE

func TestGroupGoErr(t *testing.T) {
	t.Parallel()

	t.Run("upstream error", func(t *testing.T) {
		t.Parallel()
		ctx, c, s := context.Background(), new(exampleCtx), time.Now()

		err := NewGroup().
			AddRunner(c.F).Key("f").
			AddRunner(c.X).Dep("f"). // X will be blocked due to upstream F errors
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, float64(1), time.Since(s).Truncate(time.Second).Seconds())
		assert.Equal(t, 0, c.x)
	})

	t.Run("weak dependency", func(t *testing.T) {
		t.Parallel()
		ctx, c, s := context.Background(), new(exampleCtx), time.Now()

		err := NewGroup().
			AddRunner(c.F).Key("f").
			AddRunner(c.X).WeakDep("f"). // X will execute even if upstream F errors
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, float64(2), time.Since(s).Truncate(time.Second).Seconds())
		assert.Equal(t, 1, c.x)
	})
}

func TestGroupGoErrWrap(t *testing.T) {
	t.Parallel()

	t.Run("non-ff wrapped error", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		var ferr, xerr = errors.New("F_ERR"), errors.New("X_ERR")
		var f = func() error { return ferr }
		var x = func() error { return xerr }

		err := NewGroup().
			AddRunner(f).Key("f").
			AddRunner(x).WeakDep("f"). // x will be executed, error will be wrapped
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, "X_ERR <- F_ERR", err.Error())
		assert.True(t, errors.Is(err, ferr))
		assert.True(t, errors.Is(err, xerr))
	})

	t.Run("diamond-wrapped error", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		var ferr, xerr, yerr = errors.New("F_ERR"), errors.New("X_ERR"), errors.New("Y_ERR")
		var f, x, y, z = func() error { return ferr }, func() error { return xerr }, func() error { return yerr }, func() error { return nil }

		err := NewGroup().
			AddRunner(f).Key("f").
			AddRunner(x).Key("x").WeakDep("f"). // x will be executed, error will be wrapped
			AddRunner(y).Key("y").WeakDep("f"). // y will be executed, error will be wrapped
			AddRunner(z).Dep("x", "y").
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, errors.Join(
			errors.New("X_ERR <- F_ERR"), errors.New("Y_ERR <- F_ERR"),
		).Error(), err.Error())
		assert.True(t, errors.Is(err, ferr))
		assert.True(t, errors.Is(err, xerr))
	})
}

// region CANCEL CASE

func TestGoTimeout(t *testing.T) {
	t.Parallel()
	ctx, c, s := context.Background(), new(exampleCtx), time.Now()

	err := Go(ctx, Opts(WithTimeout(1*time.Second)), c.X, c.C)

	assert.NotNil(t, err)
	assert.Equal(t, "group anonymous timeout", err.Error())
	assert.Equal(t, float64(1), time.Since(s).Truncate(time.Second).Seconds())
	assert.Equal(t, 0, c.c)
}

func TestGroupGoTimeout(t *testing.T) {
	t.Parallel()
	ctx, c, s := context.Background(), new(exampleCtx), time.Now()

	err := NewGroup(WithTimeout(1*time.Second)).
		AddRunner(c.A).Key("a").
		AddRunner(c.B).Key("b").Dep("a").
		AddRunner(c.C).Key("c").Dep("a").
		AddRunner(c.D).Key("d").Dep("b", "c").
		Go(ctx)

	assert.Equal(t, "group anonymous timeout", err.Error())
	assert.Equal(t, float64(1), time.Since(s).Truncate(time.Second).Seconds())
	// will prevent execution
	assert.Equal(t, 0, c.b)
	assert.Equal(t, 0, c.c)
	assert.Equal(t, 0, c.d)
}

func TestGroupGoCtxCancel(t *testing.T) {
	t.Parallel()

	t.Run("cancel during execution", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			// ctx timeout while C is running
			time.Sleep(3 * time.Second)
			cancel()
		}()
		c, s := new(exampleCtx), time.Now()

		err := NewGroup().
			AddRunner(c.A).Key("a").
			AddRunner(c.B).Key("b").Dep("a").
			AddRunner(c.C).Key("c").Dep("a").
			AddRunner(c.D).Key("d").Dep("b", "c").
			Go(ctx)

		assert.Equal(t, context.Canceled, err)
		assert.Equal(t, c.d, 0)
		assert.Equal(t, float64(3), time.Since(s).Truncate(time.Second).Seconds())
	})

	t.Run("cancel before execution", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // already cancelled
		c, s := new(exampleCtx), time.Now()

		err := NewGroup().
			AddRunner(c.A).Key("a").
			AddRunner(c.B).Key("b").Dep("a").
			AddRunner(c.C).Key("c").Dep("a").
			AddRunner(c.D).Key("d").Dep("b", "c").
			Go(ctx)

		assert.Equal(t, context.Canceled, err)
		assert.Equal(t, c.d, 0)
		assert.Equal(t, float64(0), time.Since(s).Truncate(time.Second).Seconds())
	})
}

// region INTERCEPTOR CASE
func TestGroupGoInterceptor(t *testing.T) {
	t.Parallel()

	t.Run("pre and after both run", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		var preRan, afterRan bool
		var capturedErr error

		err := NewGroup(
			WithPreFunc(func(ctx context.Context) error { preRan = true; return nil }),
			WithAfterFunc(func(ctx context.Context, err error) error { afterRan = true; capturedErr = err; return err }),
		).
			AddRunner(func() error { return nil }).Key("a").
			AddRunner(func() error { return errors.New("task error") }).Key("b").
			Go(ctx)

		assert.NotNil(t, err)
		assert.True(t, preRan)
		assert.True(t, afterRan)
		assert.NotNil(t, capturedErr)
	})

	t.Run("pre fail skips execution", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		var executed bool

		err := NewGroup(
			WithPreFunc(func(ctx context.Context) error { return errors.New("pre failed") }),
		).
			AddRunner(func() error { executed = true; return nil }).
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, "pre failed", err.Error())
		assert.False(t, executed)
	})

	t.Run("after suppresses error", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		err := NewGroup(
			WithAfterFunc(func(ctx context.Context, err error) error { return nil }),
		).
			AddRunner(func() error { return errors.New("original") }).
			Go(ctx)

		assert.Nil(t, err)
	})

	t.Run("after wraps error", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		err := NewGroup(
			WithAfterFunc(func(ctx context.Context, err error) error {
				if err != nil {
					return errors.New("wrapped: " + err.Error())
				}
				return nil
			}),
		).
			AddRunner(func() error { return errors.New("original") }).
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, "wrapped: original", err.Error())
	})

	t.Run("pre and after with successful execution", func(t *testing.T) {
		t.Parallel()
		ctx, c, s := context.Background(), new(exampleCtx), time.Now()
		var preRan, afterRan bool

		err := NewGroup(
			WithPreFunc(func(ctx context.Context) error { preRan = true; return nil }),
			WithAfterFunc(func(ctx context.Context, err error) error { afterRan = true; return err }),
		).
			AddRunner(c.A).Key("a").
			AddRunner(c.B).Key("b").Dep("a").
			Go(ctx)

		assert.Nil(t, err)
		assert.True(t, preRan)
		assert.True(t, afterRan)
		assert.Equal(t, 2, c.b)
		assert.Equal(t, float64(2), time.Since(s).Truncate(time.Second).Seconds())
	})

	t.Run("after runs even on timeout", func(t *testing.T) {
		t.Parallel()
		ctx, c := context.Background(), new(exampleCtx)
		var afterRan bool
		var capturedErr error

		err := NewGroup(
			WithTimeout(1*time.Second),
			WithAfterFunc(func(ctx context.Context, err error) error {
				afterRan = true
				capturedErr = err
				return err
			}),
		).
			AddRunner(c.C). // takes 2 seconds
			Go(ctx)

		assert.NotNil(t, err)
		assert.True(t, afterRan)
		assert.NotNil(t, capturedErr)
		assert.Contains(t, err.Error(), "timeout")
	})
}
