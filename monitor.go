package group

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
	"time"
)

func groupMonitor(ctx context.Context, method, prefix string, start time.Time, log bool, err error) {
	if log {
		slog.LogAttrs(ctx, slog.LevelInfo, fmt.Sprintf("[Group::%s] group %s done", method, prefix), slog.Duration("time_to_go", time.Since(start)))
		if err != nil {
			slog.LogAttrs(ctx, slog.LevelError, fmt.Sprintf("Group::[%s] group %s failed", method, prefix), slog.String("err", err.Error()))
		}
	}
}

func funcMonitor(ctx context.Context, method, prefix, name string, start time.Time, log bool, errC chan error, err error) {
	if log {
		slog.LogAttrs(ctx, slog.LevelInfo, fmt.Sprintf("[Group::%s] group %s: %s done", method, prefix, name), slog.Duration("time_to_go", time.Since(start)))
		if err != nil {
			slog.LogAttrs(ctx, slog.LevelError, fmt.Sprintf("[Group::%s] group %s: %s failed", method, prefix, name), slog.String("err", err.Error()))
		}
	}
	if errC != nil && err != nil {
		select { // avoid blocking
		case errC <- fmt.Errorf("func %s failed: %w", name, err):
		default:
		}
	}
}

func nodeMonitor(ctx context.Context, prefix string, key any, start time.Time, log bool, errC chan error, err error) {
	if log {
		slog.LogAttrs(ctx, slog.LevelInfo, fmt.Sprintf("[Group::node -> exec] group %s: node %s done", prefix, key), slog.Duration("time_to_go", time.Since(start)))
		if err != nil {
			slog.LogAttrs(ctx, slog.LevelError, fmt.Sprintf("[Group::node -> exec] group %s: node %s failed", prefix, key), slog.String("err", err.Error()))
		}
	}
	if errC != nil && err != nil {
		select { // avoid blocking
		case errC <- fmt.Errorf("node %s failed: %w", key, err):
		default:
		}
	}
}

func funcName(f any) string {
	v := reflect.ValueOf(f)
	if v.Kind() != reflect.Func {
		return "<not a func>"
	}
	if fn := runtime.FuncForPC(v.Pointer()); fn != nil {
		return fn.Name()
	}
	return "<unknown>"
}
