package group

import (
	"context"
	"sync/atomic"
)

// context with store
func WithStore(ctx context.Context, store Storer) context.Context {
	return context.WithValue(ctx, fetchKey{}, store)
}

type Storer interface {
	Store(key, value any)
	Load(key any) (value any, ok bool)
}

type storeKey struct{}
type storeFunc func(any)

func Store[V any](ctx context.Context, value V) {
	if f, _ := ctx.Value(storeKey{}).(storeFunc); f != nil {
		f(value)
	} else {
		panic("missing store func in context")
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

func Put[K, V any](ctx context.Context, key K, value V) {
	if store, _ := ctx.Value(fetchKey{}).(Storer); store != nil {
		store.Store(key, value)
	} else {
		panic("missing store in context")
	}
}

type mapStore struct {
	ptr atomic.Pointer[map[any]any]
}

// copy-on-write map store
// [ideal for read-heavy scenarios]
func NewMapStore() *mapStore {
	s := &mapStore{}
	s.ptr.Store(&map[any]any{})
	return s
}

func (s *mapStore) Load(key any) (any, bool) {
	v, ok := (*s.ptr.Load())[key]
	return v, ok
}

func (s *mapStore) Store(key, value any) {
	for {
		oldMapPtr := s.ptr.Load()
		newMap := make(map[any]any, len(*oldMapPtr)+1)
		for k, v := range *oldMapPtr {
			newMap[k] = v
		}
		newMap[key] = value
		if s.ptr.CompareAndSwap(oldMapPtr, &newMap) {
			return
		}
	}
}
