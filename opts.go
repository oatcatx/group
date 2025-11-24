package group

import (
	"context"
	"log/slog"
	"time"
)

type Options struct {
	Prefix  string        // group name, used for log
	Limit   int           // concurrency limit
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

func WithPrefix(s string) option                { return func(o *Options) { o.Prefix = s } }
func WithLimit(x int) option                    { return func(o *Options) { o.Limit = x } }
func WithTimeout(t time.Duration) option        { return func(o *Options) { o.Timeout = t } }
func WithErrorCollector(errC chan error) option { return func(o *Options) { o.ErrC = errC } }
func WithLogger(logger *slog.Logger) option {
	return func(o *Options) { o.WithLog = true; slog.SetDefault(logger) }
}

var WithLog option = func(o *Options) { o.WithLog = true }

// group specific
func WithStore(ctx context.Context, store Storer) context.Context {
	return context.WithValue(ctx, fetchKey{}, store)
}
