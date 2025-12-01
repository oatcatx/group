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
	Prefix  string        // group name, used for log
	Limit   int           // concurrency limit
	Pre     PreFunc       // group pre-execution interceptor
	After   AfterFunc     // group post-execution interceptor
	Timeout time.Duration // group timeout
	ErrC    chan error    // error collector
	WithLog bool
}

type option func(*Options)

func Opts(opts ...option) *Options {
	opt := &Options{}
	for _, o := range opts {
		o(opt)
	}
	return opt
}

func WithPrefix(s string) option { return func(o *Options) { o.Prefix = s } }
func WithLimit(x int) option {
	if x <= 0 {
		panic("limit must be positive")
	}
	return func(o *Options) { o.Limit = x }
}
func WithPreFunc(f PreFunc) option     { return func(o *Options) { o.Pre = f } }
func WithAfterFunc(f AfterFunc) option { return func(o *Options) { o.After = f } }
func WithTimeout(t time.Duration) option {
	if t <= 0 {
		panic("timeout must be positive")
	}
	return func(o *Options) { o.Timeout = t }
}
func WithErrorCollector(errC chan error) option { return func(o *Options) { o.ErrC = errC } }
func WithLogger(logger *slog.Logger) option {
	return func(o *Options) { o.WithLog = true; slog.SetDefault(logger) }
}

var WithLog option = func(o *Options) { o.WithLog = true }
