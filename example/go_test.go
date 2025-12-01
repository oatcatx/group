package group

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	. "github.com/oatcatx/group"
)

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

func TestGoInterceptor(t *testing.T) {
	t.Parallel()

	t.Run("pre and after both run", func(t *testing.T) {
		t.Parallel()
		var preRan, afterRan bool
		var capturedErr error

		err := Go(context.Background(), Opts(
			WithPreFunc(func(ctx context.Context) error { preRan = true; return nil }),
			WithAfterFunc(func(ctx context.Context, err error) error { afterRan = true; capturedErr = err; return err }),
		),
			func() error { return nil },
			func() error { return errors.New("task error") },
		)

		assert.NotNil(t, err)
		assert.True(t, preRan)
		assert.True(t, afterRan)
		assert.NotNil(t, capturedErr)
	})

	t.Run("pre fail skips execution", func(t *testing.T) {
		t.Parallel()
		var executed bool

		err := Go(context.Background(), Opts(WithPreFunc(func(ctx context.Context) error { return errors.New("pre failed") })),
			func() error { executed = true; return nil },
		)

		assert.NotNil(t, err)
		assert.Equal(t, "pre failed", err.Error())
		assert.False(t, executed)
	})

	t.Run("after suppresses error", func(t *testing.T) {
		t.Parallel()

		err := Go(context.Background(), Opts(WithAfterFunc(func(ctx context.Context, err error) error { return nil })),
			func() error { return errors.New("original") },
		)

		assert.Nil(t, err)
	})

	t.Run("after wraps error", func(t *testing.T) {
		t.Parallel()

		err := Go(context.Background(),
			Opts(WithAfterFunc(func(ctx context.Context, err error) error {
				if err != nil {
					return errors.New("wrapped: " + err.Error())
				}
				return nil
			})),
			func() error { return errors.New("original") },
		)

		assert.NotNil(t, err)
		assert.Equal(t, "wrapped: original", err.Error())
	})
}
