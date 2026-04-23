package group

import (
	"time"
)

// nodes represents a batch of nodes that share configurations
type nodes struct {
	indices []int
	*Group
}

func (ns *nodes) Keys(keys ...any) *nodes {
	for idx, key := range keys {
		ns.nodes[ns.indices[idx]].Key(key)
	}
	return ns
}

func (ns *nodes) Dep(keys ...any) *nodes {
	for _, idx := range ns.indices {
		ns.nodes[idx].Dep(keys...)
	}
	return ns
}

func (ns *nodes) WeakDep(keys ...any) *nodes {
	for _, idx := range ns.indices {
		ns.nodes[idx].WeakDep(keys...)
	}
	return ns
}

func (ns *nodes) FastFail() *nodes {
	for _, idx := range ns.indices {
		ns.nodes[idx].FastFail()
	}
	return ns
}

func (ns *nodes) SilentFail() *nodes {
	for _, idx := range ns.indices {
		ns.nodes[idx].SilentFail()
	}
	return ns
}

func (ns *nodes) WithRetry(times int) *nodes {
	for _, idx := range ns.indices {
		ns.nodes[idx].WithRetry(times)
	}
	return ns
}

func (ns *nodes) WithPreFunc(f NodePreFunc) *nodes {
	for _, idx := range ns.indices {
		ns.nodes[idx].WithPreFunc(f)
	}
	return ns
}

func (ns *nodes) WithAfterFunc(f NodeAfterFunc) *nodes {
	for _, idx := range ns.indices {
		ns.nodes[idx].WithAfterFunc(f)
	}
	return ns
}

func (ns *nodes) WithRollback(f NodeRollbackFunc) *nodes {
	for _, idx := range ns.indices {
		ns.nodes[idx].WithRollback(f)
	}
	return ns
}

func (ns *nodes) WithCondition(f NodeConditionFunc) *nodes {
	for _, idx := range ns.indices {
		ns.nodes[idx].WithCondition(f)
	}
	return ns
}

func (ns *nodes) SkipIf(skip bool) *nodes {
	for _, idx := range ns.indices {
		ns.nodes[idx].SkipIf(skip)
	}
	return ns
}

func (ns *nodes) WithTimeout(t time.Duration) *nodes {
	for _, idx := range ns.indices {
		ns.nodes[idx].WithTimeout(t)
	}
	return ns
}
