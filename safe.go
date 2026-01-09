package group

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
)

var ErrPanic = errors.New("panic recovered")

const bufSize int = 64 << 10

func RecoverCtx(ctx context.Context) {
	if r := recover(); r != nil {
		panicAttrs, locAttrs := make([]slog.Attr, 0, 3), make([]slog.Attr, 0, 3)
		panicAttrs = append(panicAttrs, slog.String("type", fmt.Sprintf("%T", r)), slog.Any("value", r))
		var loc string
		if pc, file, line, ok := runtime.Caller(2); ok {
			loc = file + ":" + strconv.Itoa(line)
			locAttrs = append(locAttrs, slog.String("file", file), slog.Int("line", line))
			if fn := runtime.FuncForPC(pc); fn != nil {
				fnName := fn.Name()
				loc += " (" + fnName + ")"
				locAttrs = append(locAttrs, slog.String("func", fnName))
			}
			panicAttrs = append(panicAttrs, slog.GroupAttrs("location", locAttrs...))
		}
		buf := make([]byte, bufSize)
		slog.LogAttrs(ctx, slog.LevelError, ErrPanic.Error(), slog.GroupAttrs("panic", panicAttrs...), slog.String("stack", string(buf[:runtime.Stack(buf, false)])))
	}
}

func RecoverCtxErr(ctx context.Context, err *error) {
	if r := recover(); r != nil {
		panicAttrs, locAttrs := make([]slog.Attr, 0, 3), make([]slog.Attr, 0, 3)
		panicAttrs = append(panicAttrs, slog.String("type", fmt.Sprintf("%T", r)), slog.Any("value", r))
		var loc string
		if pc, file, line, ok := runtime.Caller(2); ok {
			loc = file + ":" + strconv.Itoa(line)
			locAttrs = append(locAttrs, slog.String("file", file), slog.Int("line", line))
			if fn := runtime.FuncForPC(pc); fn != nil {
				fnName := fn.Name()
				loc += " (" + fnName + ")"
				locAttrs = append(locAttrs, slog.String("func", fnName))
			}
			panicAttrs = append(panicAttrs, slog.GroupAttrs("location", locAttrs...))
		}
		buf := make([]byte, bufSize)
		slog.LogAttrs(ctx, slog.LevelError, ErrPanic.Error(), slog.GroupAttrs("panic", panicAttrs...), slog.String("stack", string(buf[:runtime.Stack(buf, false)])))

		var panicErr error
		if e, ok := r.(error); ok {
			panicErr = e
		} else {
			panicErr = fmt.Errorf("%v", r)
		}
		if loc != "" {
			*err = fmt.Errorf("%w at %s: %w", ErrPanic, loc, panicErr)
		} else {
			*err = fmt.Errorf("%w: %w", ErrPanic, panicErr)
		}
	}
}

func SafeRun(ctx context.Context, f func() error) (err error) {
	defer RecoverCtxErr(ctx, &err)
	return f()
}

func SafeRunNode(ctx context.Context, f func(context.Context, any) error, shared any) (err error) {
	defer RecoverCtxErr(ctx, &err)
	return f(ctx, shared)
}
