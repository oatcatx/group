package group

import (
	"context"
	"log/slog"
	"runtime"
)

const bufSize int = 64 << 10

func RecoverContext(ctx context.Context) {
	if x := recover(); x != nil {
		buf := make([]byte, bufSize)
		buf = buf[:runtime.Stack(buf, false)]
		slog.ErrorContext(ctx, "runtime panic: %v\n%s", x, string(buf))
	}
}

func SafeRun(ctx context.Context, f func() error) error {
	defer RecoverContext(ctx)
	return f()
}

func SafeRunCtx(ctx context.Context, f func(context.Context) error) error {
	defer RecoverContext(ctx)
	return f(ctx)
}

func SafeRunCtxShared(ctx context.Context, f func(context.Context, any) error, shared any) error {
	defer RecoverContext(ctx)
	return f(ctx, shared)
}
