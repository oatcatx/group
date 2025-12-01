package group

import (
	"context"
	"fmt"
	"time"
)

type Node interface {
	Key() any
	Dep() []any
	WeakDep() []any
	Exec(ctx context.Context, shared any) error
}

type node struct {
	idx              int
	key              any
	deps, to, weakTo []int // dependencies | to nodes | weak to nodes
	f                func(ctx context.Context, shared any) error
	nodeSpec
	*Group
}

// node level interceptor
type NodePreFunc func(ctx context.Context, shared any) error
type NodeAfterFunc func(ctx context.Context, shared any, err error) error

type nodeSpec struct {
	ff      bool // fast-fail flag
	retry   int
	pre     func(ctx context.Context, shared any) error
	after   func(ctx context.Context, shared any, err error) error
	timeout time.Duration
}

func (n *node) Key(key any) *node {
	if _, ok := n.idxMap[key]; ok {
		panic(fmt.Sprintf("duplicate node key %q", key))
	}
	n.key, n.idxMap[key] = key, n.idx
	return n
}

func (n *node) Dep(keys ...any) *node {
	for _, key := range keys {
		idx, ok := n.idxMap[key]
		if !ok {
			panic(fmt.Sprintf("missing dependency %q -> %q", n.key, key))
		}
		for _, depIdx := range n.deps {
			if depIdx == idx {
				panic(fmt.Sprintf("duplicate dependency %q -> %q", n.key, key))
			}
		}
		n.deps, n.nodes[idx].to = append(n.deps, idx), append(n.nodes[idx].to, n.idx)
	}
	return n
}

func (n *node) WeakDep(keys ...any) *node {
	for _, key := range keys {
		idx, ok := n.idxMap[key]
		if !ok {
			panic(fmt.Sprintf("missing dependency %q -> %q", n.key, key))
		}
		for _, depIdx := range n.deps {
			if depIdx == idx {
				panic(fmt.Sprintf("duplicate dependency %q -> %q", n.key, key))
			}
		}
		n.deps, n.nodes[idx].to, n.nodes[idx].weakTo = append(n.deps, idx), append(n.nodes[idx].to, n.idx), append(n.nodes[idx].weakTo, n.idx)
	}
	return n
}

func (n *node) FastFail() *node {
	n.ff = true
	return n
}

func (n *node) WithRetry(times int) *node {
	if times < 0 {
		panic("retry times must be non-negative")
	}
	n.retry = times
	return n
}

func (n *node) WithPreFunc(f NodePreFunc) *node {
	n.pre = f
	return n
}

func (n *node) WithAfterFunc(f NodeAfterFunc) *node {
	n.after = f
	return n
}

func (n *node) WithTimeout(t time.Duration) *node {
	if t <= 0 {
		panic("timeout must be positive")
	}
	n.timeout = t
	return n
}

func (n *node) Verify(panicking bool) *node {
	n.Group.Verify(panicking)
	return n
}
