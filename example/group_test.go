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

// A -> B -> D
// \        /
// -\> C -/ -> E
// F -> X (F will fail)
// time spent (without E) = A + max(B, C) + D = 4
// time spent (with E) = A + max(B, C) + D + E = 5
// res(d) = (1 + 1) + (1 + 1) = 4
type exampleCtx struct {
	a, b, c, d, e, x int
}

func (c *exampleCtx) A() error {
	time.Sleep(1 * time.Second)
	fmt.Println("A")
	c.a = 1
	return nil
}

func (c *exampleCtx) B() error {
	time.Sleep(1 * time.Second)
	fmt.Println("B")
	c.b = c.a + 1
	return nil
}

func (c *exampleCtx) C() error {
	time.Sleep(2 * time.Second)
	fmt.Println("C")
	c.c = c.a + 1
	return nil
}

func (c *exampleCtx) D() error {
	time.Sleep(1 * time.Second)
	fmt.Println("D")
	c.d = c.b + c.c
	return nil
}

func (c *exampleCtx) E() error {
	time.Sleep(1 * time.Second)
	fmt.Println("E")
	c.e = c.c + 1
	return nil
}

func (c *exampleCtx) F() error {
	time.Sleep(1 * time.Second)
	fmt.Println("F")
	return errors.New("F_ERR")
}

func (c *exampleCtx) X() error {
	time.Sleep(1 * time.Second)
	c.x = 1
	return nil
}

func (c *exampleCtx) Res() int {
	return c.d
}

func TestGo(t *testing.T) {
	t.Parallel()
	ctx, c, s := context.Background(), new(exampleCtx), time.Now()

	//= GroupGo serial dependency control
	err := Go(ctx, nil, c.A)
	assert.Nil(t, err)
	err = Go(ctx, nil, c.B, c.C)
	assert.Nil(t, err)
	err = Go(ctx, nil, c.D)
	assert.Nil(t, err)

	assert.Equal(t, 4, c.Res())
	assert.Equal(t, float64(4), time.Since(s).Truncate(time.Second).Seconds())

	//= GroupGo manual signal dependency control
	c, s = new(exampleCtx), time.Now()

	type signal = chan struct{}
	var aS, bS, cS, done = make(signal), make(signal), make(signal), make(signal)
	err = Go(ctx, nil,
		func() error {
			defer close(aS)
			return c.A()
		},
		func() error {
			<-aS
			defer close(bS)
			return c.B()
		},
		func() error {
			<-aS
			defer close(cS)
			return c.C()
		},
		func() error {
			_, _ = <-bS, <-cS
			defer close(done)
			return c.D()
		})
	assert.Nil(t, err)

	assert.Equal(t, 4, c.Res())
	assert.Equal(t, float64(4), time.Since(s).Truncate(time.Second).Seconds())
}

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

func TestGroupGoFFCtrl(t *testing.T) {
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
			AddRunner(c.F).Key("f").FF(). // fast-fail
			AddRunner(c.C).Key("c").      // C will execute for 2 seconds
			AddRunner(c.X).Dep("c").      // X will be blocked since fast-fail error occurs
			Go(ctx)

		assert.NotNil(t, err)
		assert.Equal(t, float64(2), time.Since(s).Truncate(time.Second).Seconds())
		assert.Equal(t, 0, c.x)
	})
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

	assert.Equal(t, "group timeout", err.Error())
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

	assert.Equal(t, "group timeout", err.Error())
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
			// ctx timeout while b & c are running
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
