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

	if opts.prefix == "" {
		opts.prefix = "anonymous" // default prefix
	}

	if opts.log {
		defer func(start time.Time) {
			groupMonitor(ctx, "Go", opts.prefix, start, opts.log, err)
		}(time.Now())
	}

	limit := len(fs) // limit defaults to number of funcs
	if opts.limit > 0 {
		limit = opts.limit
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)

	// group timeout
	if opts.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.timeout)
		defer cancel()
	}

	// group pre-execution interceptor
	if opts.pre != nil {
		if err := opts.pre(ctx); err != nil {
			return err
		}
	}
	exec(ctx, g, opts, fs...)
	// group post-execution interceptor
	if opts.after != nil {
		defer func() {
			err = opts.after(ctx, err)
		}()
	}

	// outer timeout control
	if opts.timeout > 0 {
		done := make(chan error, 1)
		go func() {
			done <- g.Wait()
		}()
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) { // actual timeout
				if opts.log {
					slog.InfoContext(ctx, fmt.Sprintf("[Group::Go] group %s timeout", opts.prefix), slog.Duration("after", opts.timeout))
				}
				return fmt.Errorf("group %s timeout", opts.prefix)
			}
			return <-done
		case err = <-done:
			return
		}
	}
	return g.Wait()
}

func GoCtx(ctx context.Context, opts *Options, fs ...func(context.Context) error) error {
	fcs := make([]func() error, 0, len(fs))
	for _, f := range fs {
		fcs = append(fcs, func() error { return f(ctx) })
	}
	return Go(ctx, opts, fcs...)
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

	if opts.limit < len(fs) {
		return false, errors.New("limit cannot be less than the number of funcs")
	}
	if opts.prefix == "" {
		opts.prefix = "anonymous"
	}
	if opts.log {
		defer func(start time.Time) {
			groupMonitor(ctx, "TryGo", opts.prefix, start, opts.log, err)
		}(time.Now())
	}

	g, gtx := errgroup.WithContext(ctx)
	limit := len(fs) // limit defaults to number of funcs
	if opts.limit > 0 {
		limit = opts.limit
	}
	g.SetLimit(limit)
	// set timeout for group and fs
	if opts.timeout > 0 {
		var cancel context.CancelFunc
		gtx, cancel = context.WithTimeout(gtx, opts.timeout)
		defer cancel()
	}

	// group pre-execution interceptor
	if opts.pre != nil {
		if err = opts.pre(ctx); err != nil {
			return
		}
	}
	ok = tryExec(gtx, g, opts, fs...)
	// group post-execution interceptor
	if opts.after != nil {
		defer func() {
			err = opts.after(ctx, err)
		}()
	}

	// outer timeout control
	if opts.timeout > 0 {
		done := make(chan error, 1)
		go func() {
			done <- g.Wait()
		}()
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) { // actual timeout
				if opts.log {
					slog.InfoContext(ctx, fmt.Sprintf("[Group::TryGo] group %s timeout", opts.prefix), slog.Duration("after", opts.timeout))
				}
				return ok, fmt.Errorf("group %s timeout", opts.prefix)
			}
			return ok, <-done
		case err = <-done:
			return
		}
	}
	return ok, g.Wait()
}

func TryGoCtx(ctx context.Context, opts *Options, fs ...func(context.Context) error) (bool, error) {
	fcs := make([]func() error, 0, len(fs))
	for _, f := range fs {
		fcs = append(fcs, func() error { return f(ctx) })
	}
	return TryGo(ctx, opts, fcs...)
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
			if opts == nil || !opts.log && opts.ErrC == nil {
				return SafeRun(ctx, f)
			}

			if opts.log || opts.ErrC != nil {
				defer func(start time.Time) {
					funcMonitor(ctx, "[Go -> exec]", opts.prefix, funcName(f), start, opts.log, opts.ErrC, err)
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
			if opts == nil || !opts.log && opts.ErrC == nil {
				return SafeRun(ctx, f)
			}

			if opts.log || opts.ErrC != nil {
				defer func(start time.Time) {
					funcMonitor(ctx, "[TryGo -> exec]", opts.prefix, funcName(f), start, opts.log, opts.ErrC, err)
				}(time.Now())
			}
			return SafeRun(ctx, f)
		})
	}
	return ok
}
