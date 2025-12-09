package group

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
)

const bufSize int = 64 << 10

func RecoverContext(ctx context.Context, err *error) {
	if r := recover(); r != nil {
		buf := make([]byte, bufSize)
		buf = buf[:runtime.Stack(buf, false)]
		slog.ErrorContext(ctx, "panic recovered", slog.Any("panic", r), slog.String("stack", string(buf)))
		if e, ok := r.(error); ok {
			*err = fmt.Errorf("panic: %w", e)
		} else {
			*err = fmt.Errorf("panic: %v", r)
		}
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
