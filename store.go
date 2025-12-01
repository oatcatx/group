package group

import (
	"context"
)

// context with store
func WithStore(ctx context.Context, store Storer) context.Context {
	return context.WithValue(ctx, fetchKey{}, store)
}

type Storer interface {
	Store(key any, value any)
	Load(key any) (value any, ok bool)
}

type storeKey struct{}
type storeFunc func(any)

func Store[T any](ctx context.Context, v T) {
	if f, _ := ctx.Value(storeKey{}).(storeFunc); f != nil {
		f(v)
	} else {
		panic("missing store in context")
	}
}

type fetchKey struct{}

func Fetch[T any](ctx context.Context, key any) (T, bool) {
	if store, _ := ctx.Value(fetchKey{}).(Storer); store != nil {
		if val, ok := store.Load(key); ok {
			if v, ok := val.(T); ok {
				return v, true
			}
		}
	}
	v, ok := ctx.Value(key).(T)
	return v, ok
}
