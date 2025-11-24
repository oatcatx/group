package group

import (
	"context"
	"fmt"
)

type node struct {
	f   func() error
	fc  func(ctx context.Context) error
	fca func(ctx context.Context, arg any) error

	idx    int
	key    any
	deps   []int // dependencies
	to     []int // to nodes
	weakTo []int // to weak nodes
	ff     bool  // fast-fail flag
	*Group
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

func (n *node) FF() *node {
	n.ff = true
	return n
}

func (n *node) Verify(panicking bool) *node {
	n.Group.Verify(panicking)
	return n
}
