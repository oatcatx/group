package group

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"
)

func Go(ctx context.Context, opts *Options, fs ...func() error) (err error) {
	if len(fs) == 0 {
		return nil
	}

	// no opts short circuit
	if opts == nil {
		g, gtx := errgroup.WithContext(ctx)
		g.SetLimit(len(fs)) // limit defaults to number of funcs
		exec(gtx, g, nil, fs...)
		return g.Wait()
	}

	if opts.Prefix == "" {
		opts.Prefix = "anonymous" // default prefix
	}

	if opts.WithLog {
		defer func(start time.Time) {
			groupMonitor(ctx, "Go", opts.Prefix, start, opts.WithLog, err)
		}(time.Now())
	}

	limit := len(fs) // limit defaults to number of funcs
	if opts.Limit > 0 {
		limit = opts.Limit
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)

	// group timeout
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// group pre-execution interceptor
	if opts.Pre != nil {
		if err := opts.Pre(ctx); err != nil {
			return err
		}
	}
	exec(ctx, g, opts, fs...)
	// group post-execution interceptor
	if opts.After != nil {
		defer func() {
			err = opts.After(ctx, err)
		}()
	}

	// outer timeout control
	if opts.Timeout > 0 {
		done := make(chan error, 1)
		go func() {
			done <- g.Wait()
		}()
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) { // actual timeout
				if opts.WithLog {
					slog.InfoContext(ctx, fmt.Sprintf("[Group::Go] group %s timeout", opts.Prefix), slog.Duration("after", opts.Timeout))
				}
				return fmt.Errorf("group %s timeout", opts.Prefix)
			}
			return <-done
		case err = <-done:
			return
		}
	}
	return g.Wait()
}

func TryGo(ctx context.Context, opts *Options, fs ...func() error) (ok bool, err error) {
	if len(fs) == 0 {
		return true, nil
	}

	// no opts short circuit
	if opts == nil {
		g, ctx := errgroup.WithContext(ctx)
		// limit defaults to number of funcs
		g.SetLimit(len(fs))
		return tryExec(ctx, g, nil, fs...), g.Wait()
	}

	if opts.Limit < len(fs) {
		return false, errors.New("limit cannot be less than the number of funcs")
	}
	if opts.Prefix == "" {
		opts.Prefix = "anonymous"
	}
	if opts.WithLog {
		defer func(start time.Time) {
			groupMonitor(ctx, "TryGo", opts.Prefix, start, opts.WithLog, err)
		}(time.Now())
	}

	g, gtx := errgroup.WithContext(ctx)
	limit := len(fs) // limit defaults to number of funcs
	if opts.Limit > 0 {
		limit = opts.Limit
	}
	g.SetLimit(limit)
	// set timeout for group and fs
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		gtx, cancel = context.WithTimeout(gtx, opts.Timeout)
		defer cancel()
	}

	// group pre-execution interceptor
	if opts.Pre != nil {
		if err = opts.Pre(ctx); err != nil {
			return
		}
	}
	ok = tryExec(gtx, g, opts, fs...)
	// group post-execution interceptor
	if opts.After != nil {
		defer func() {
			err = opts.After(ctx, err)
		}()
	}

	// outer timeout control
	if opts.Timeout > 0 {
		done := make(chan error, 1)
		go func() {
			done <- g.Wait()
		}()
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) { // actual timeout
				if opts.WithLog {
					slog.InfoContext(ctx, fmt.Sprintf("[Group::TryGo] group %s timeout", opts.Prefix), slog.Duration("after", opts.Timeout))
				}
				return ok, fmt.Errorf("group %s timeout", opts.Prefix)
			}
			return ok, <-done
		case err = <-done:
			return
		}
	}
	return ok, g.Wait()
}

func exec(ctx context.Context, g *errgroup.Group, opts *Options, fs ...func() error) {
	for _, f := range fs {
		g.Go(func() (err error) {
			// ctx check before exec
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// no opts short circuit
			if opts == nil || !opts.WithLog && opts.ErrC == nil {
				return SafeRun(ctx, f)
			}

			if opts.WithLog || opts.ErrC != nil {
				defer func(start time.Time) {
					funcMonitor(ctx, "[Go -> exec]", opts.Prefix, funcName(f), start, opts.WithLog, opts.ErrC, err)
				}(time.Now())
			}
			return SafeRun(ctx, f)
		})
	}
}

func tryExec(ctx context.Context, g *errgroup.Group, opts *Options, fs ...func() error) bool {
	ok := true
	for _, f := range fs {
		ok = ok && g.TryGo(func() (err error) {
			// ctx check before exec
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// no opts short circuit
			if opts == nil || !opts.WithLog && opts.ErrC == nil {
				return SafeRun(ctx, f)
			}

			if opts.WithLog || opts.ErrC != nil {
				defer func(start time.Time) {
					funcMonitor(ctx, "[TryGo -> exec]", opts.Prefix, funcName(f), start, opts.WithLog, opts.ErrC, err)
				}(time.Now())
			}
			return SafeRun(ctx, f)
		})
	}
	return ok
}
