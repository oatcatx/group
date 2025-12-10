package group

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
)

var ErrPanic = errors.New("panic recovered")

const bufSize int = 64 << 10

func RecoverContext(ctx context.Context, err *error) {
	if r := recover(); r != nil {
		panicAttrs := []slog.Attr{
			slog.String("type", fmt.Sprintf("%T", r)),
			slog.Any("value", r),
		}
		var loc string
		if pc, file, line, ok := runtime.Caller(2); ok {
			loc = fmt.Sprintf("%s:%d", file, line)
			locAttrs := []slog.Attr{
				slog.String("file", file),
				slog.Int("line", line),
			}
			if fn := runtime.FuncForPC(pc); fn != nil {
				loc += " " + fn.Name()
				locAttrs = append(locAttrs, slog.String("func", fn.Name()))
			}
			panicAttrs = append(panicAttrs, slog.GroupAttrs("location", locAttrs...))
		}
		buf := make([]byte, bufSize)
		buf = buf[:runtime.Stack(buf, false)]
		slog.LogAttrs(ctx, slog.LevelError, ErrPanic.Error(), slog.GroupAttrs("panic", panicAttrs...), slog.String("stack", string(buf)))
		if _, ok := r.(error); !ok {
			r = fmt.Errorf("%v", r)
		}
		*err = fmt.Errorf("%w at %s: %w", ErrPanic, loc, r.(error))
	}
}

func SafeRun(ctx context.Context, f func() error) (err error) {
	defer RecoverContext(ctx, &err)
	return f()
}

func SafeRunNode(ctx context.Context, f func(context.Context, any) error, shared any) (err error) {
	defer RecoverContext(ctx, &err)
	return f(ctx, shared)
}
