package group

import (
	"context"
	"log/slog"
	"time"
)

// group level interceptor
type PreFunc func(context.Context) error
type AfterFunc func(context.Context, error) error
type Options struct {
	prefix  string        // group name, used for log
	limit   int           // concurrency limit
	pre     PreFunc       // group pre-execution interceptor
	after   AfterFunc     // group post-execution interceptor
	timeout time.Duration // group timeout
	log     bool          // enable logging with default or custom logger

	ErrC chan error // error collector
}

type option func(*Options)

func Opts(opts ...option) *Options {
	opt := &Options{}
	for _, o := range opts {
		o(opt)
	}
	return opt
}

func WithPrefix(s string) option { return func(o *Options) { o.prefix = s } }
func WithLimit(x int) option {
	if x <= 0 {
		panic("limit must be positive")
	}
	return func(o *Options) { o.limit = x }
}
func WithPreFunc(f PreFunc) option     { return func(o *Options) { o.pre = f } }
func WithAfterFunc(f AfterFunc) option { return func(o *Options) { o.after = f } }
func WithTimeout(t time.Duration) option {
	if t <= 0 {
		panic("timeout must be positive")
	}
	return func(o *Options) { o.timeout = t }
}

var WithLog option = func(o *Options) { o.log = true }

func WithLogger(logger *slog.Logger) option {
	return func(o *Options) { o.log = true; slog.SetDefault(logger) }
}

func WithErrorCollector(errC chan error) option { return func(o *Options) { o.ErrC = errC } }
